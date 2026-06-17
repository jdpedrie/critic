package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jdp/critic/server/agent"
	"github.com/jdp/critic/server/vault"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// filepathBase is a tiny alias to keep call sites tidy. We import path/filepath
// for a single helper; this lets readers see intent at the callsite.
func filepathBase(p string) string { return filepath.Base(p) }

// concatNamedFiles joins a map of file paths → content into a single markdown
// block, with `### <path>` headers between files. Keys are sorted so output is
// stable for diffing and caching.
func concatNamedFiles(files map[string]string) string {
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "### %s\n\n%s\n\n", k, strings.TrimSpace(files[k]))
	}
	return strings.TrimRight(b.String(), "\n")
}

func normalizeIssueID(id string) string {
	id = strings.TrimSpace(id)
	if !strings.HasPrefix(strings.ToUpper(id), "ISSUE-") {
		id = "ISSUE-" + id
	}
	return strings.ToUpper(id)
}

func main() {
	if len(os.Args) < 2 || os.Args[1] != "serve" {
		fmt.Fprintf(os.Stderr, "usage: critic serve\n")
		os.Exit(1)
	}

	cfgPath := os.Getenv("CRITIC_CONFIG")
	if cfgPath == "" {
		cfgPath = "config.yaml"
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Load persistent settings for API keys.
	ps, _ := readSettings()

	// Initialize external-harness agents (Claude is handled by cowork subagents,
	// not from the MCP server).
	var codexAgent *agent.Codex
	if cfg.Codex.Enabled {
		codexAgent = agent.NewCodex(cfg.Codex.Model, settingOrEnv(ps, "openai_api_key"))
	}

	var piAgent *agent.Pi
	if cfg.Pi.Enabled {
		piAgent = agent.NewPi(cfg.Pi.Provider, cfg.Pi.Model)
	}

	s := server.NewMCPServer(
		"critic",
		"0.1.0",
		server.WithToolCapabilities(true),
	)

	// Common vault param for all tools that touch the filesystem.
	vaultParam := mcp.WithString("vault", mcp.Required(),
		mcp.Description("Absolute path to the Obsidian vault root"))

	// invoke-codex
	if codexAgent != nil {
		s.AddTool(
			mcp.NewTool("invoke-codex",
				mcp.WithDescription("Invoke Codex with a system + user prompt. Returns {response, session_id}. Pass session_id to resume a prior conversation. Set include_manuscript_from to a vault path to have the server append the manuscript text to the user prompt (avoids passing it through the tool call)."),
				mcp.WithString("system_prompt", mcp.Required(), mcp.Description("System prompt (role, framing, instructions)")),
				mcp.WithString("user_prompt", mcp.Required(), mcp.Description("User prompt (the actual question or task)")),
				mcp.WithString("session_id", mcp.Description("Session ID from a previous invoke-codex call to resume")),
				mcp.WithString("include_manuscript_from", mcp.Description("Vault path. If set, the server appends '=== MANUSCRIPT ===' followed by all chapters in order to the user prompt.")),
			),
			makeInvokeCodexHandler(codexAgent),
		)
	}

	// invoke-pi
	if piAgent != nil {
		s.AddTool(
			mcp.NewTool("invoke-pi",
				mcp.WithDescription("Invoke the pi harness (https://pi.dev) with a system + user prompt. Returns {response, session_id}. Pass session_id to resume. Provider/model can be overridden per call (only honored on new sessions; resumed sessions use the original provider/model). Set include_manuscript_from to append manuscript text server-side."),
				mcp.WithString("system_prompt", mcp.Required(), mcp.Description("System prompt")),
				mcp.WithString("user_prompt", mcp.Required(), mcp.Description("User prompt")),
				mcp.WithString("session_id", mcp.Description("Session ID from a previous invoke-pi call to resume")),
				mcp.WithString("provider", mcp.Description("Pi provider (e.g. anthropic, openai, google). Defaults to config.")),
				mcp.WithString("model", mcp.Description("Pi model. Defaults to config.")),
				mcp.WithString("include_manuscript_from", mcp.Description("Vault path. If set, server appends manuscript text to user_prompt.")),
			),
			makeInvokePiHandler(piAgent),
		)
	}

	// pi-list-models
	if piAgent != nil {
		s.AddTool(
			mcp.NewTool("pi-list-models",
				mcp.WithDescription("List models available to the pi harness via `pi --list-models`. Use to populate model pickers in interactive skills."),
			),
			makePiListModelsHandler(piAgent),
		)
	}

	// get-prompt
	s.AddTool(
		mcp.NewTool("get-prompt",
			mcp.WithDescription("Load a prompt template. Resolution: <vault>/prompts/<name> > $CLAUDE_PLUGIN_ROOT/prompts/<name> > embedded default. Optional vars are substituted using Go text/template (e.g. {{.Role}}, {{.MaxIssues}}, {{.ReviewNum}})."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Prompt filename (e.g. agent-framing.md, manuscript.md, verdict.md, rejection-pass.md)")),
			mcp.WithString("vault", mcp.Description("Vault path. If set, vault/prompts/<name> is checked first.")),
			mcp.WithString("vars", mcp.Description("JSON object of template variables (e.g. {\"Role\":\"analytical\",\"MaxIssues\":7}).")),
		),
		makeGetPromptHandler(),
	)

	// stage-review-part
	s.AddTool(
		mcp.NewTool("stage-review-part",
			mcp.WithDescription("Stage a named part for later assembly into a review file. Use this to avoid passing large content through save-review. Call once per section (e.g., 'synthesis', 'claude-review', 'codex-review', 'cross-review')."),
			vaultParam,
			mcp.WithString("name", mcp.Required(), mcp.Description("Part name (e.g. synthesis, claude-review, claude-rejection, codex-review, grok-review, adversarial, cross-review)")),
			mcp.WithString("content", mcp.Required(), mcp.Description("Content for this part")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			v, vErr := vaultFromReq(req)
			if vErr != nil {
				return vaultErr(vErr), nil
			}
			name, _ := req.RequireString("name")
			content, _ := req.RequireString("content")
			if err := v.WriteStagedPart(name, content); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("stage part: %v", err)), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Staged part: %s", name)), nil
		},
	)

	// assemble-review
	s.AddTool(
		mcp.NewTool("assemble-review",
			mcp.WithDescription("Assemble a review from previously staged parts. The synthesis part goes above the sentinel; all other parts go below. Much faster than save-review for large documents."),
			vaultParam,
			mcp.WithString("prefix", mcp.Required(), mcp.Description("Filename prefix (e.g. manuscript-critic)")),
			mcp.WithString("synthesis_part", mcp.Required(), mcp.Description("Name of the staged part to use as synthesis (above the sentinel)")),
			mcp.WithString("raw_parts", mcp.Required(), mcp.Description("Comma-separated names of staged parts for the raw outputs section (below the sentinel), in order")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			v, vErr := vaultFromReq(req)
			if vErr != nil {
				return vaultErr(vErr), nil
			}
			prefix, _ := req.RequireString("prefix")
			synthesisPart, _ := req.RequireString("synthesis_part")
			rawPartsStr, _ := req.RequireString("raw_parts")

			var rawParts []string
			for _, p := range strings.Split(rawPartsStr, ",") {
				p = strings.TrimSpace(p)
				if p != "" {
					rawParts = append(rawParts, p)
				}
			}

			relPath, reviewNum, err := v.AssembleReview(prefix, synthesisPart, rawParts)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("assemble review: %v", err)), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Saved to %s (review #%03d)", relPath, reviewNum)), nil
		},
	)

	// save-review (kept for backward compatibility and small reviews)
	s.AddTool(
		mcp.NewTool("save-review",
			mcp.WithDescription("Save a review to the vault's review/ directory as a timestamped markdown file. For large reviews, prefer stage-review-part + assemble-review."),
			vaultParam,
			mcp.WithString("prefix", mcp.Required(), mcp.Description("Filename prefix (e.g. manuscript-critic, chapter-05-review)")),
			mcp.WithString("content", mcp.Required(), mcp.Description("Full markdown content to write")),
		),
		makeSaveReviewHandler(),
	)

	// read-stage
	s.AddTool(
		mcp.NewTool("read-stage",
			mcp.WithDescription("Return the draft-stage block to inject as `=== CURRENT DRAFT STAGE ===` for reviewers. If <vault>/stage.md exists, returns its contents verbatim (author override). Otherwise synthesizes a stage description from the storyline project frontmatter (acts, chapters, labels) and scene metadata."),
			vaultParam,
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			v, vErr := vaultFromReq(req)
			if vErr != nil {
				return vaultErr(vErr), nil
			}
			if override := v.ReadStage(); strings.TrimSpace(override) != "" {
				return mcp.NewToolResultText(override), nil
			}
			p, err := v.ReadProject()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("read project: %v", err)), nil
			}
			scenes, err := v.ReadScenes()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("read scenes: %v", err)), nil
			}
			return mcp.NewToolResultText(v.DerivedStage(p, scenes)), nil
		},
	)

	// read-style-guide
	s.AddTool(
		mcp.NewTool("read-style-guide",
			mcp.WithDescription("Return the project's style guide. Looks for <vault>/style.md first; falls back to <vault>/Research/style.md if the root file is absent. Returns empty string if neither exists."),
			vaultParam,
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			v, vErr := vaultFromReq(req)
			if vErr != nil {
				return vaultErr(vErr), nil
			}
			if content := v.ReadStyleGuide(); strings.TrimSpace(content) != "" {
				return mcp.NewToolResultText(content), nil
			}
			research, _ := v.ReadResearchFiles()
			for path, content := range research {
				if strings.EqualFold(filepathBase(path), "style.md") {
					return mcp.NewToolResultText(content), nil
				}
			}
			return mcp.NewToolResultText(""), nil
		},
	)

	// read-research
	s.AddTool(
		mcp.NewTool("read-research",
			mcp.WithDescription("Concatenate every markdown file under <vault>/Research/ into one block, with `### <relative-path>` headers between files. Use to inline worldbuilding context for reviewers."),
			vaultParam,
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			v, vErr := vaultFromReq(req)
			if vErr != nil {
				return vaultErr(vErr), nil
			}
			files, err := v.ReadResearchFiles()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("read research: %v", err)), nil
			}
			return mcp.NewToolResultText(concatNamedFiles(files)), nil
		},
	)

	// read-codex
	s.AddTool(
		mcp.NewTool("read-codex",
			mcp.WithDescription("Concatenate Codex entries (Characters + Locations) into one block, with `### <relative-path>` headers. Pass `names` (comma-separated entity names matching filenames without .md) to filter; omit for all entries. Use to inline character/location reference data for reviewers."),
			vaultParam,
			mcp.WithString("names", mcp.Description("Comma-separated list of entity names (filenames without .md). Omit or pass empty for all entries.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			v, vErr := vaultFromReq(req)
			if vErr != nil {
				return vaultErr(vErr), nil
			}
			var names []string
			if raw := optString(req, "names"); raw != "" {
				for _, n := range strings.Split(raw, ",") {
					if n = strings.TrimSpace(n); n != "" {
						names = append(names, n)
					}
				}
			}
			files, err := v.ReadCodexEntries(names)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("read codex: %v", err)), nil
			}
			return mcp.NewToolResultText(concatNamedFiles(files)), nil
		},
	)

	// read-codex-entry
	s.AddTool(
		mcp.NewTool("read-codex-entry",
			mcp.WithDescription("Read a single Codex entry by name (filename without .md). Searches Characters/ then Locations/. Use for on-demand lookups by Claude when reviewing prose."),
			vaultParam,
			mcp.WithString("name", mcp.Required(), mcp.Description("Entity name, matching the filename without .md (e.g. \"Henry Nelson\", \"Fontenoy Harbor\").")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			v, vErr := vaultFromReq(req)
			if vErr != nil {
				return vaultErr(vErr), nil
			}
			name, _ := req.RequireString("name")
			content, err := v.ReadCodexEntry(name)
			if err != nil {
				if os.IsNotExist(err) {
					return mcp.NewToolResultError(fmt.Sprintf("no Codex entry named %q", name)), nil
				}
				return mcp.NewToolResultError(fmt.Sprintf("read codex entry: %v", err)), nil
			}
			return mcp.NewToolResultText(content), nil
		},
	)

	// assemble-manuscript
	s.AddTool(
		mcp.NewTool("assemble-manuscript",
			mcp.WithDescription("Return the full manuscript assembled from the storyline project's Scenes/ folder, in the same Markdown format the storyline plugin's `Export project` command produces (# Title, ## Act N: <label>, ### Chapter N: <label>, #### <scene title>, body). Use to inline the manuscript for Claude subagents."),
			vaultParam,
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			v, vErr := vaultFromReq(req)
			if vErr != nil {
				return vaultErr(vErr), nil
			}
			manuscript, err := v.ReadManuscript()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("assemble manuscript: %v", err)), nil
			}
			return mcp.NewToolResultText(manuscript), nil
		},
	)

	// list-codex-entries
	s.AddTool(
		mcp.NewTool("list-codex-entries",
			mcp.WithDescription("List every Codex entry name (filename without .md), from Codex/Characters/ and Codex/Locations/. Returns one name per line, sorted. Use to give a subagent the canonical entity roster so it can flag prose mentions of unknown entities."),
			vaultParam,
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			v, vErr := vaultFromReq(req)
			if vErr != nil {
				return vaultErr(vErr), nil
			}
			names, err := v.ListCodexEntries()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("list codex entries: %v", err)), nil
			}
			return mcp.NewToolResultText(strings.Join(names, "\n")), nil
		},
	)

	// find-entity-mentions
	s.AddTool(
		mcp.NewTool("find-entity-mentions",
			mcp.WithDescription("Scan every scene in the storyline project for case-insensitive substring matches of an entity name. Returns JSON array `[{filename, act, chapter, sequence, title, body}]` of scenes that mention the entity, in manuscript order. Use for entity-mode canon extraction (gather all mentions of one entity across the book in a single call)."),
			vaultParam,
			mcp.WithString("name", mcp.Required(), mcp.Description("Entity name to search for (case-insensitive substring match against scene body text).")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			v, vErr := vaultFromReq(req)
			if vErr != nil {
				return vaultErr(vErr), nil
			}
			name, _ := req.RequireString("name")
			needle := strings.ToLower(strings.TrimSpace(name))
			if needle == "" {
				return mcp.NewToolResultError("name is empty"), nil
			}
			scenes, err := v.ReadScenes()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("read scenes: %v", err)), nil
			}
			type match struct {
				Filename string `json:"filename"`
				Act      int    `json:"act"`
				Chapter  int    `json:"chapter"`
				Sequence int    `json:"sequence"`
				Title    string `json:"title"`
				Body     string `json:"body"`
			}
			var hits []match
			for _, s := range scenes {
				if !strings.Contains(strings.ToLower(s.Body), needle) {
					continue
				}
				hits = append(hits, match{
					Filename: s.Filename,
					Act:      s.Act,
					Chapter:  s.Chapter,
					Sequence: s.Sequence,
					Title:    s.Title,
					Body:     s.Body,
				})
			}
			data, _ := json.Marshal(hits)
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	// assemble-chapter
	s.AddTool(
		mcp.NewTool("assemble-chapter",
			mcp.WithDescription("Return the assembled text of one chapter (all scenes for the given chapter number, sorted by sequence, formatted as `### Chapter N: <label>\\n\\n#### <scene title>\\n\\n<body>`) plus the union of entity names referenced in those scenes' frontmatter (POV, characters, location). Response is JSON: {text, entities, scene_count}. Use to scope a chapter review and prefilter Codex."),
			vaultParam,
			mcp.WithString("chapter", mcp.Required(), mcp.Description("Chapter number (integer, matches the `chapter:` frontmatter field on scenes).")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			v, vErr := vaultFromReq(req)
			if vErr != nil {
				return vaultErr(vErr), nil
			}
			chapterStr, _ := req.RequireString("chapter")
			var chapter int
			if _, err := fmt.Sscanf(strings.TrimSpace(chapterStr), "%d", &chapter); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("chapter must be an integer: %q", chapterStr)), nil
			}
			p, err := v.ReadProject()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("read project: %v", err)), nil
			}
			scenes, err := v.ReadScenes()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("read scenes: %v", err)), nil
			}
			var chScenes []vault.Scene
			for _, s := range scenes {
				if s.Chapter == chapter {
					chScenes = append(chScenes, s)
				}
			}
			if len(chScenes) == 0 {
				return mcp.NewToolResultError(fmt.Sprintf("no scenes found for chapter %d", chapter)), nil
			}
			text := vault.RenderChapter(p, chapter, chScenes)
			entities := vault.SceneEntityNames(chScenes)
			data, _ := json.Marshal(map[string]any{
				"text":        text,
				"entities":    entities,
				"scene_count": len(chScenes),
			})
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	// read-scene
	s.AddTool(
		mcp.NewTool("read-scene",
			mcp.WithDescription("Return one scene's assembled text (`#### <title>\\n\\n<body>`, wikilinks stripped) plus the entity names from its frontmatter. Response is JSON: {text, entities, act, chapter, sequence, title}. Use to scope a single-scene review and prefilter Codex."),
			vaultParam,
			mcp.WithString("scene", mcp.Required(), mcp.Description("Scene filename, with or without .md (e.g. \"01-01 Customs at Fontenoy\" or \"01-01 Customs at Fontenoy.md\").")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			v, vErr := vaultFromReq(req)
			if vErr != nil {
				return vaultErr(vErr), nil
			}
			sceneArg, _ := req.RequireString("scene")
			sceneArg = strings.TrimSuffix(strings.TrimSpace(sceneArg), ".md")
			scenes, err := v.ReadScenes()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("read scenes: %v", err)), nil
			}
			var found *vault.Scene
			for i := range scenes {
				if scenes[i].Filename == sceneArg {
					found = &scenes[i]
					break
				}
			}
			if found == nil {
				return mcp.NewToolResultError(fmt.Sprintf("scene %q not found in Scenes/", sceneArg)), nil
			}
			text := vault.RenderScene(*found)
			entities := vault.SceneEntityNames([]vault.Scene{*found})
			data, _ := json.Marshal(map[string]any{
				"text":     text,
				"entities": entities,
				"act":      found.Act,
				"chapter":  found.Chapter,
				"sequence": found.Sequence,
				"title":    found.Title,
			})
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	// list-scenes
	s.AddTool(
		mcp.NewTool("list-scenes",
			mcp.WithDescription("List every scene in the storyline project, one per line, formatted as `act/chapter/sequence | filename | title`. Sorted in manuscript order (act → chapter → sequence)."),
			vaultParam,
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			v, vErr := vaultFromReq(req)
			if vErr != nil {
				return vaultErr(vErr), nil
			}
			scenes, err := v.ReadScenes()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("read scenes: %v", err)), nil
			}
			var b strings.Builder
			for _, s := range scenes {
				fmt.Fprintf(&b, "%d/%d/%d | %s | %s\n", s.Act, s.Chapter, s.Sequence, s.Filename, s.Title)
			}
			return mcp.NewToolResultText(strings.TrimRight(b.String(), "\n")), nil
		},
	)

	// write-summary
	s.AddTool(
		mcp.NewTool("write-summary",
			mcp.WithDescription("Write a chapter summary to summary/<chapter-name>.md in the vault."),
			vaultParam,
			mcp.WithString("chapter", mcp.Required(), mcp.Description("Chapter name (e.g. chapter-01)")),
			mcp.WithString("content", mcp.Required(), mcp.Description("The summary content to write")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			v, vErr := vaultFromReq(req)
			if vErr != nil {
				return vaultErr(vErr), nil
			}
			chapter, _ := req.RequireString("chapter")
			content, _ := req.RequireString("content")
			if err := v.WriteSummary(chapter, content); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("write summary: %v", err)), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Saved summary/%s.md", chapter)), nil
		},
	)

	// read-issues
	s.AddTool(
		mcp.NewTool("read-issues",
			mcp.WithDescription("Read the issues.md file from the vault root. Contains known issues deferred for later resolution."),
			vaultParam,
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			v, vErr := vaultFromReq(req)
			if vErr != nil {
				return vaultErr(vErr), nil
			}
			content := v.ReadIssues()
			if content == "" {
				return mcp.NewToolResultText("No issues.md file found."), nil
			}
			return mcp.NewToolResultText(content), nil
		},
	)

	// append-issue
	s.AddTool(
		mcp.NewTool("append-issue",
			mcp.WithDescription("Add a deferred issue to issues.md under a heading (e.g. 'General', 'Chapter 3'). Creates the file and heading if needed."),
			vaultParam,
			mcp.WithString("heading", mcp.Required(), mcp.Description("Section heading (e.g. General, Chapter 3)")),
			mcp.WithString("entry", mcp.Required(), mcp.Description("The issue entry text (should include the issue ID if from a review)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			v, vErr := vaultFromReq(req)
			if vErr != nil {
				return vaultErr(vErr), nil
			}
			heading, _ := req.RequireString("heading")
			entry, _ := req.RequireString("entry")
			if err := v.AppendIssue(heading, entry); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("append issue: %v", err)), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Added to issues.md under \"%s\"", heading)), nil
		},
	)

	// next-review-number
	s.AddTool(
		mcp.NewTool("next-review-number",
			mcp.WithDescription("Get the next global review number. Use before synthesis to get the correct issue ID prefix."),
			vaultParam,
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			v, vErr := vaultFromReq(req)
			if vErr != nil {
				return vaultErr(vErr), nil
			}
			num := v.NextReviewNumber()
			return mcp.NewToolResultText(fmt.Sprintf("%d", num)), nil
		},
	)

	// snapshot-and-diff
	s.AddTool(
		mcp.NewTool("snapshot-and-diff",
			mcp.WithDescription("Atomic: writes a new snapshot of story/ to review/.snapshots/<prefix>-<timestamp>.md, locates the prior snapshot for the same prefix, computes a unified diff against it, and saves the diff as a paired <prefix>-<timestamp>.diff file alongside the snapshot. Returns JSON {snapshot_path, prior_path, diff_path, diff_text}. If there's no prior snapshot (first run) or the manuscript is unchanged, diff_path and diff_text are empty."),
			vaultParam,
			mcp.WithString("prefix", mcp.Required(), mcp.Description("Snapshot filename prefix (e.g. \"manuscript\").")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			v, vErr := vaultFromReq(req)
			if vErr != nil {
				return vaultErr(vErr), nil
			}
			prefix, _ := req.RequireString("prefix")
			snapshotPath, priorPath, diffPath, diffText, err := v.SnapshotAndDiff(prefix)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("snapshot-and-diff: %v", err)), nil
			}
			data, _ := json.Marshal(map[string]string{
				"snapshot_path": snapshotPath,
				"prior_path":    priorPath,
				"diff_path":     diffPath,
				"diff_text":     diffText,
			})
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	// write-snapshot
	s.AddTool(
		mcp.NewTool("write-snapshot",
			mcp.WithDescription("Concatenate every chapter file in story/ into a single timestamped snapshot under review/.snapshots/<prefix>-<timestamp>.md. Returns JSON {path, prior_path}. prior_path is the vault-relative path of the most recent prior snapshot with the same prefix (empty if none)."),
			vaultParam,
			mcp.WithString("prefix", mcp.Required(), mcp.Description("Snapshot filename prefix (e.g. \"manuscript\").")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			v, vErr := vaultFromReq(req)
			if vErr != nil {
				return vaultErr(vErr), nil
			}
			prefix, _ := req.RequireString("prefix")
			path, priorPath, err := v.WriteSnapshot(prefix)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("write snapshot: %v", err)), nil
			}
			data, _ := json.Marshal(map[string]string{
				"path":       path,
				"prior_path": priorPath,
			})
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	// diff-snapshots
	s.AddTool(
		mcp.NewTool("diff-snapshots",
			mcp.WithDescription("Run `diff -u prior current` and return the unified diff text. Paths can be vault-relative or absolute. Returns the diff body verbatim (use it to feed a Claude subagent that writes a human-readable summary)."),
			vaultParam,
			mcp.WithString("prior", mcp.Required(), mcp.Description("Path to the older snapshot (vault-relative or absolute).")),
			mcp.WithString("current", mcp.Required(), mcp.Description("Path to the newer snapshot.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			v, vErr := vaultFromReq(req)
			if vErr != nil {
				return vaultErr(vErr), nil
			}
			prior, _ := req.RequireString("prior")
			current, _ := req.RequireString("current")
			out, err := v.DiffSnapshots(prior, current)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("diff: %v", err)), nil
			}
			return mcp.NewToolResultText(out), nil
		},
	)

	// read-issue
	s.AddTool(
		mcp.NewTool("read-issue",
			mcp.WithDescription("Read a specific issue from a review file by its ISSUE-NNN-NN ID. The review number is extracted from the ID."),
			vaultParam,
			mcp.WithString("issue_id", mcp.Required(), mcp.Description("Issue ID (e.g. ISSUE-003-01 or 003-01)")),
		),
		makeReadIssueHandler(),
	)

	// add-rebuttal
	s.AddTool(
		mcp.NewTool("add-rebuttal",
			mcp.WithDescription("Add an author rebuttal to a specific issue in a review file. The rebuttal is inserted inline after the issue."),
			vaultParam,
			mcp.WithString("issue_id", mcp.Required(), mcp.Description("Issue ID (e.g. ISSUE-003-01 or 003-01)")),
			mcp.WithString("rebuttal", mcp.Required(), mcp.Description("The author's rebuttal text")),
		),
		makeAddRebuttalHandler(),
	)

	// read-settings
	s.AddTool(
		mcp.NewTool("read-settings",
			mcp.WithDescription("Read current critic plugin settings."),
		),
		makeReadSettingsHandler(),
	)

	// write-setting
	s.AddTool(
		mcp.NewTool("write-setting",
			mcp.WithDescription("Write a critic plugin setting."),
			mcp.WithString("key", mcp.Required(), mcp.Description("Setting key (e.g. vault_path, gemini_api_key, claude_enabled)")),
			mcp.WithString("value", mcp.Required(), mcp.Description("Setting value")),
		),
		makeWriteSettingHandler(),
	)

	// update-memory
	s.AddTool(
		mcp.NewTool("update-memory",
			mcp.WithDescription("Update a reviewer's memory file with new information."),
			vaultParam,
			mcp.WithString("role", mcp.Required(), mcp.Description("Reviewer role: analytical, immersive, structural, adversarial")),
			mcp.WithString("content", mcp.Required(), mcp.Description("New memory content to write")),
		),
		makeUpdateMemoryHandler(),
	)

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func vaultFromReq(req mcp.CallToolRequest) (*vault.Vault, error) {
	path, _ := req.RequireString("vault")
	return vault.New(path)
}

// vaultErr renders a vault-open failure as a tool error result.
func vaultErr(err error) *mcp.CallToolResult {
	return mcp.NewToolResultError(fmt.Sprintf("open vault: %v", err))
}


func makeReadIssueHandler() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		v, vErr := vaultFromReq(req)
			if vErr != nil {
				return vaultErr(vErr), nil
			}
		issueID := normalizeIssueID(req.GetArguments()["issue_id"].(string))

		// Extract review number from issue ID (ISSUE-003-01 → 3)
		var reviewNum int
		fmt.Sscanf(issueID, "ISSUE-%d-", &reviewNum)
		if reviewNum == 0 {
			return mcp.NewToolResultError("could not parse review number from issue ID"), nil
		}

		content, _, err := v.ReadReviewByNumber(reviewNum)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("read review: %v", err)), nil
		}

		// Find the issue section — look for the heading containing the issue ID
		lines := strings.Split(content, "\n")
		var issueLines []string
		capturing := false
		for _, line := range lines {
			if strings.Contains(strings.ToUpper(line), issueID) && strings.HasPrefix(strings.TrimSpace(line), "#") {
				capturing = true
				issueLines = append(issueLines, line)
				continue
			}
			if capturing {
				// Stop at the next heading of same or higher level, or sentinel
				trimmed := strings.TrimSpace(line)
				if (strings.HasPrefix(trimmed, "### ISSUE-") || strings.HasPrefix(trimmed, "## ")) && len(issueLines) > 0 {
					break
				}
				if strings.Contains(line, "RAW AGENT OUTPUTS BELOW") {
					break
				}
				issueLines = append(issueLines, line)
			}
		}

		if len(issueLines) == 0 {
			return mcp.NewToolResultError(fmt.Sprintf("issue %s not found in review #%03d", issueID, reviewNum)), nil
		}

		return mcp.NewToolResultText(strings.Join(issueLines, "\n")), nil
	}
}

func makeAddRebuttalHandler() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		v, vErr := vaultFromReq(req)
			if vErr != nil {
				return vaultErr(vErr), nil
			}
		issueID := normalizeIssueID(req.GetArguments()["issue_id"].(string))
		rebuttal, _ := req.RequireString("rebuttal")

		var reviewNum int
		fmt.Sscanf(issueID, "ISSUE-%d-", &reviewNum)
		if reviewNum == 0 {
			return mcp.NewToolResultError("could not parse review number from issue ID"), nil
		}

		content, filename, err := v.ReadReviewByNumber(reviewNum)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("read review: %v", err)), nil
		}

		// Find the issue heading and insert the rebuttal after the issue block
		lines := strings.Split(content, "\n")
		var result []string
		issueFound := false
		capturing := false
		inserted := false

		rebuttalBlock := fmt.Sprintf("\n> [!quote] Author Rebuttal (%s)\n> %s\n",
			issueID,
			strings.ReplaceAll(strings.TrimSpace(rebuttal), "\n", "\n> "))

		for i, line := range lines {
			if !issueFound && strings.Contains(strings.ToUpper(line), issueID) && strings.HasPrefix(strings.TrimSpace(line), "#") {
				issueFound = true
				capturing = true
				result = append(result, line)
				continue
			}
			if capturing && !inserted {
				trimmed := strings.TrimSpace(line)
				isNextSection := (strings.HasPrefix(trimmed, "### ISSUE-") || strings.HasPrefix(trimmed, "## ")) ||
					strings.Contains(line, "RAW AGENT OUTPUTS BELOW")
				isSeparator := trimmed == "---"
				if isNextSection || isSeparator || i == len(lines)-1 {
					result = append(result, rebuttalBlock)
					inserted = true
					capturing = false
				}
			}
			result = append(result, line)
		}

		if !issueFound {
			return mcp.NewToolResultError(fmt.Sprintf("issue %s not found in review file", issueID)), nil
		}
		if !inserted {
			// Issue was the last thing in the file
			result = append(result, rebuttalBlock)
		}

		if err := v.WriteReviewFile(filename, strings.Join(result, "\n")); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("write review: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Added rebuttal to %s in %s", issueID, filename)), nil
	}
}

func makeSaveReviewHandler() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		v, vErr := vaultFromReq(req)
			if vErr != nil {
				return vaultErr(vErr), nil
			}
		prefix, _ := req.RequireString("prefix")
		content, _ := req.RequireString("content")

		relPath, reviewNum, err := v.WriteReview(prefix, content)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("save review: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Saved to %s (review #%03d)", relPath, reviewNum)), nil
	}
}

func makeUpdateMemoryHandler() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		v, vErr := vaultFromReq(req)
			if vErr != nil {
				return vaultErr(vErr), nil
			}
		role, _ := req.RequireString("role")
		content, _ := req.RequireString("content")

		validRoles := map[string]bool{
			"analytical": true, "immersive": true,
			"structural": true, "adversarial": true,
		}
		if !validRoles[strings.ToLower(role)] {
			return mcp.NewToolResultError(fmt.Sprintf("invalid role: %s", role)), nil
		}

		if err := v.WriteReviewerMemory(role, content); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("write memory: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Updated memory for %s reviewer.", role)), nil
	}
}
