package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jdp/critic/server/agent"
	"github.com/jdp/critic/server/vault"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

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
			v := vaultFromReq(req)
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
			v := vaultFromReq(req)
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

	// list-chapters
	s.AddTool(
		mcp.NewTool("list-chapters",
			mcp.WithDescription("List all chapter names in the vault's story/ directory."),
			vaultParam,
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			v := vaultFromReq(req)
			names, err := v.ListChapterNames()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("list chapters: %v", err)), nil
			}
			return mcp.NewToolResultText(strings.Join(names, "\n")), nil
		},
	)

	// summarize-chapter
	s.AddTool(
		mcp.NewTool("summarize-chapter",
			mcp.WithDescription("Read a single chapter and generate a summary. Returns the chapter text for the agent to summarize."),
			vaultParam,
			mcp.WithString("chapter", mcp.Required(), mcp.Description("Chapter filename (e.g. chapter-01)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			v := vaultFromReq(req)
			chapter, _ := req.RequireString("chapter")
			text, err := v.ReadChapter(chapter)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("read chapter: %v", err)), nil
			}
			pages := vault.PageCount(text)
			return mcp.NewToolResultText(fmt.Sprintf("Chapter: %s (~%d pages)\n\n%s", chapter, pages, text)), nil
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
			v := vaultFromReq(req)
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
			v := vaultFromReq(req)
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
			v := vaultFromReq(req)
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
			v := vaultFromReq(req)
			num := v.NextReviewNumber()
			return mcp.NewToolResultText(fmt.Sprintf("%d", num)), nil
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

func vaultFromReq(req mcp.CallToolRequest) *vault.Vault {
	path, _ := req.RequireString("vault")
	return vault.New(path)
}


func makeReadIssueHandler() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		v := vaultFromReq(req)
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
		v := vaultFromReq(req)
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
		v := vaultFromReq(req)
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
		v := vaultFromReq(req)
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
