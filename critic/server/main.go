package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jdp/critic/server/agent"
	"github.com/jdp/critic/server/reviewer"
	"github.com/jdp/critic/server/vault"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

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

	// Initialize agents based on enabled flags.
	// API key resolution: settings file > CLAUDE_PLUGIN_OPTION_ env > system auth
	var claudeAgent agent.Agent
	if cfg.Claude.Enabled {
		ca := agent.NewClaude(cfg.Claude.Model, settingOrEnv(ps, "anthropic_api_key"))
		ca.VaultPath = settingOrEnv(ps, "vault_path")
		claudeAgent = ca
	}

	// Set vault path for prompt overrides
	reviewer.VaultPath = settingOrEnv(ps, "vault_path")

	var codexAgent agent.Agent
	if cfg.Codex.Enabled {
		codexAgent = agent.NewCodex(cfg.Codex.Model, settingOrEnv(ps, "openai_api_key"))
	}

	var geminiAgent agent.Agent
	if cfg.Gemini.Enabled {
		geminiAgent = agent.NewGemini(cfg.Gemini.Model)
	}

	var adversarialAgent agent.Agent
	if cfg.Adversarial.Enabled && cfg.Adversarial.BaseURL != "" && cfg.Adversarial.Model != "" {
		apiKey := settingOrEnv(ps, "adversarial_api_key")
		if apiKey == "" {
			apiKey = cfg.Adversarial.APIKey
		}
		adversarialAgent = agent.NewOpenAICompat(
			cfg.Adversarial.BaseURL,
			apiKey,
			cfg.Adversarial.Model,
		)
	}

	// agentFor maps config model type names to agent instances.
	// Falls back through available agents if the requested one is disabled.
	agentFor := func(modelType string) agent.Agent {
		switch modelType {
		case "codex":
			if codexAgent != nil {
				return codexAgent
			}
		case "gemini":
			if geminiAgent != nil {
				return geminiAgent
			}
		}
		if claudeAgent != nil {
			return claudeAgent
		}
		if codexAgent != nil {
			return codexAgent
		}
		if geminiAgent != nil {
			return geminiAgent
		}
		log.Fatal("no agents are enabled — enable at least one provider in plugin settings")
		return nil
	}

	s := server.NewMCPServer(
		"critic",
		"0.1.0",
		server.WithToolCapabilities(true),
	)

	// Common vault param for all tools that touch the filesystem.
	vaultParam := mcp.WithString("vault", mcp.Required(),
		mcp.Description("Absolute path to the Obsidian vault root"))

	// review-analytical
	s.AddTool(
		mcp.NewTool("review-analytical",
			mcp.WithDescription("Run analytical reader review on a chapter. Text-only: clarity, coherence, implicit understanding."),
			vaultParam,
			mcp.WithString("chapter", mcp.Required(), mcp.Description("Chapter filename (e.g. chapter-01)")),
			mcp.WithNumber("prior_chapters", mcp.Description("Number of prior chapters to include (default: from config)")),
		),
		makeReviewHandler(cfg, reviewer.RoleAnalytical, agentFor(cfg.Models.TextAnalytical)),
	)

	// review-immersive
	s.AddTool(
		mcp.NewTool("review-immersive",
			mcp.WithDescription("Run immersive reader review on a chapter. Text-only: engagement, pacing, emotional continuity."),
			vaultParam,
			mcp.WithString("chapter", mcp.Required(), mcp.Description("Chapter filename (e.g. chapter-01)")),
			mcp.WithNumber("prior_chapters", mcp.Description("Number of prior chapters to include (default: from config)")),
		),
		makeReviewHandler(cfg, reviewer.RoleImmersive, agentFor(cfg.Models.TextImmersive)),
	)

	// review-structural
	s.AddTool(
		mcp.NewTool("review-structural",
			mcp.WithDescription("Run structural analyst review on a chapter. Full-context: continuity, causality, plan alignment."),
			vaultParam,
			mcp.WithString("chapter", mcp.Required(), mcp.Description("Chapter filename (e.g. chapter-01)")),
		),
		makeFullContextReviewHandler(cfg, reviewer.RoleStructural, agentFor(cfg.Models.FullStructural)),
	)

	// review-adversarial
	s.AddTool(
		mcp.NewTool("review-adversarial",
			mcp.WithDescription("Run adversarial critic review on a chapter. Full-context: contradictions, weak motivations, missed opportunities."),
			vaultParam,
			mcp.WithString("chapter", mcp.Required(), mcp.Description("Chapter filename (e.g. chapter-01)")),
		),
		makeFullContextReviewHandler(cfg, reviewer.RoleAdversarial, agentFor(cfg.Models.FullAdversarial)),
	)

	// cross-review (no vault needed — works on JSON inputs only)
	s.AddTool(
		mcp.NewTool("cross-review",
			mcp.WithDescription("Run cross-review between two reviews. Each reviewer rebuts the other."),
			mcp.WithString("review_a", mcp.Required(), mcp.Description("JSON output from reviewer A")),
			mcp.WithString("review_b", mcp.Required(), mcp.Description("JSON output from reviewer B")),
			mcp.WithString("model_a", mcp.Description("Model type for reviewer A: claude or codex (default: claude)")),
			mcp.WithString("model_b", mcp.Description("Model type for reviewer B: claude or codex (default: codex)")),
		),
		makeCrossReviewHandler(cfg, claudeAgent, codexAgent),
	)

	// synthesize (no vault needed — works on JSON inputs only)
	s.AddTool(
		mcp.NewTool("synthesize",
			mcp.WithDescription("Synthesize all reviews and rebuttals into a readable markdown report with numbered issue IDs."),
			mcp.WithString("reviews", mcp.Required(), mcp.Description("JSON object mapping role names to review JSON strings")),
			mcp.WithString("rebuttals", mcp.Description("JSON object mapping pair names to rebuttal JSON strings")),
			mcp.WithNumber("review_number", mcp.Description("Review sequence number for issue IDs (e.g. 3 → ISSUE-003-01). If omitted, defaults to 0.")),
		),
		makeSynthesizeHandler(cfg, agentFor(cfg.Models.Synthesizer)),
	)

	// extract-canon
	s.AddTool(
		mcp.NewTool("extract-canon",
			mcp.WithDescription("Extract ground truth facts from a chapter and compare against world state."),
			vaultParam,
			mcp.WithString("chapter", mcp.Required(), mcp.Description("Chapter filename (e.g. chapter-01)")),
		),
		makeExtractCanonHandler(cfg, agentFor(cfg.Models.Synthesizer)),
	)

	// assess-downstream
	s.AddTool(
		mcp.NewTool("assess-downstream",
			mcp.WithDescription("Assess downstream effects of editing a chapter. Reads from the edited chapter through the end of the manuscript and flags what breaks."),
			vaultParam,
			mcp.WithString("chapter", mcp.Required(), mcp.Description("The edited chapter filename (e.g. chapter-03). All subsequent chapters will be checked.")),
		),
		makeDownstreamHandler(cfg, agentFor(cfg.Models.Synthesizer)),
	)

	// review-manuscript-claude
	if claudeAgent != nil {
		s.AddTool(
			mcp.NewTool("review-manuscript-claude",
				mcp.WithDescription("Review the full manuscript at the book level using Claude. Text-only: arc completion, pacing, character consistency, dangling threads, tonal drift."),
				vaultParam,
				mcp.WithString("prior_review_summary", mcp.Description("Summary of the previous review. The reviewer will assess whether prior issues were addressed.")),
			),
			makeManuscriptHandler(claudeAgent),
		)
	}

	// review-manuscript-codex
	if codexAgent != nil {
		s.AddTool(
			mcp.NewTool("review-manuscript-codex",
				mcp.WithDescription("Review the full manuscript at the book level using Codex. Text-only: arc completion, pacing, character consistency, dangling threads, tonal drift."),
				vaultParam,
				mcp.WithString("prior_review_summary", mcp.Description("Summary of the previous review. The reviewer will assess whether prior issues were addressed.")),
			),
			makeManuscriptHandler(codexAgent),
		)
	}

	// review-manuscript-gemini
	// review-manuscript-gemini
	if geminiAgent != nil {
		s.AddTool(
			mcp.NewTool("review-manuscript-gemini",
				mcp.WithDescription("Review the full manuscript at the book level using Gemini. Text-only: arc completion, pacing, character consistency, dangling threads, tonal drift."),
				vaultParam,
				mcp.WithString("prior_review_summary", mcp.Description("Summary of the previous review. The reviewer will assess whether prior issues were addressed.")),
			),
			makeManuscriptHandler(geminiAgent),
		)
	}

	// review-manuscript-claude-rejection
	if claudeAgent != nil {
		s.AddTool(
			mcp.NewTool("review-manuscript-claude-rejection",
				mcp.WithDescription("Review the full manuscript using Claude, then automatically run a rejection pass in the same session. Returns both the review and the rejection analysis."),
				vaultParam,
				mcp.WithString("prior_review_summary", mcp.Description("Summary of the previous review.")),
			),
			makeManuscriptRejectionHandler(claudeAgent),
		)
	}

	// review-manuscript-grok (constructive review using the adversarial model)
	if adversarialAgent != nil {
		s.AddTool(
			mcp.NewTool("review-manuscript-grok",
				mcp.WithDescription("Review the full manuscript at the book level using the adversarial model (e.g., Grok) as a constructive reviewer. Same literary agent framing as Claude/Codex but from a less aligned model."),
				vaultParam,
				mcp.WithString("prior_review_summary", mcp.Description("Summary of the previous review.")),
			),
			makeManuscriptHandler(adversarialAgent),
		)
	}

	// review-manuscript-adversarial (rejection framing)
	if adversarialAgent != nil {
		s.AddTool(
			mcp.NewTool("review-manuscript-adversarial",
				mcp.WithDescription("Adversarial manuscript review — assumes the manuscript was rejected and explains why. Uses a separate model (e.g., Grok, DeepSeek) for a less aligned perspective."),
				vaultParam,
			),
			makeAdversarialManuscriptHandler(adversarialAgent),
		)
	}

	// summarize-review
	s.AddTool(
		mcp.NewTool("summarize-review",
			mcp.WithDescription("Read the most recent review for a given prefix and produce a concise summary of its key findings. Use before running a new review so reviewers can assess whether prior issues were addressed."),
			vaultParam,
			mcp.WithString("prefix", mcp.Required(), mcp.Description("Review filename prefix (e.g. manuscript-critic, chapter-05-review)")),
		),
		makeSummarizeReviewHandler(agentFor(cfg.Models.Synthesizer)),
	)

	// cross-review-manuscript
	s.AddTool(
		mcp.NewTool("cross-review-manuscript",
			mcp.WithDescription("Cross-review manuscript reviews by resuming each reviewer's session. Each model rebuts the others' reviews with full context from its own original analysis. Accepts 2 or 3 reviewers."),
			mcp.WithString("claude_review", mcp.Required(), mcp.Description("Claude's manuscript review text")),
			mcp.WithString("codex_review", mcp.Required(), mcp.Description("Codex's manuscript review text")),
			mcp.WithString("gemini_review", mcp.Description("Gemini's manuscript review text (optional — if provided, three-way cross-review)")),
			mcp.WithString("claude_session_id", mcp.Description("Claude session ID for session resumption")),
			mcp.WithString("codex_session_id", mcp.Description("Codex session ID for session resumption")),
			mcp.WithString("gemini_session_id", mcp.Description("Gemini session ID for session resumption")),
		),
		makeCrossReviewManuscriptHandler(cfg, claudeAgent, codexAgent, geminiAgent),
	)

	// save-review
	s.AddTool(
		mcp.NewTool("save-review",
			mcp.WithDescription("Save a review to the vault's review/ directory as a timestamped markdown file."),
			vaultParam,
			mcp.WithString("prefix", mcp.Required(), mcp.Description("Filename prefix (e.g. manuscript-critic, chapter-05-review)")),
			mcp.WithString("content", mcp.Required(), mcp.Description("Full markdown content to write")),
		),
		makeSaveReviewHandler(),
	)

	// consult
	s.AddTool(
		mcp.NewTool("consult",
			mcp.WithDescription("Get a second opinion from other AI models on a narrowly-scoped question about fiction writing. Pass a question and optional context (e.g., a passage, a change, a decision). Returns answers from all enabled non-Claude models."),
			mcp.WithString("question", mcp.Required(), mcp.Description("The question to ask")),
			mcp.WithString("context", mcp.Description("Optional context — a passage, diff, or background information")),
		),
		makeConsultHandler(codexAgent, geminiAgent),
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

func makeReviewHandler(cfg *Config, role reviewer.Role, a agent.Agent) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		v := vaultFromReq(req)
		chapter, _ := req.RequireString("chapter")

		text, err := v.ReadChapter(chapter)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("read chapter: %v", err)), nil
		}

		namedSummaries, _ := v.ReadPriorSummaries(chapter)
		var summaries []struct{ Name, Content string }
		for _, s := range namedSummaries {
			summaries = append(summaries, struct{ Name, Content string }{s.Name, s.Content})
		}

		userPrompt := reviewer.BuildTextOnlyContext(text, summaries)
		userPrompt = reviewer.WithStyleGuide(v.ReadStyleGuide(), userPrompt)
		userPrompt = reviewer.WithPageInfo(vault.PageCount(text), v.TotalPageCount(), userPrompt)

		r := reviewer.New(role, a, cfg.Review.MaxIssues)
		out, err := r.Review(ctx, userPrompt)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("review failed: %v", err)), nil
		}

		return mcp.NewToolResultText(out), nil
	}
}

func makeFullContextReviewHandler(cfg *Config, role reviewer.Role, a agent.Agent) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		v := vaultFromReq(req)
		chapter, _ := req.RequireString("chapter")

		text, err := v.ReadChapter(chapter)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("read chapter: %v", err)), nil
		}

		canon, err := v.ReadCanonFiles()
		if err != nil {
			canon = make(map[string]string)
		}
		plot, err := v.ReadPlotFiles()
		if err != nil {
			plot = make(map[string]string)
		}

		userPrompt := reviewer.BuildFullContext(text, canon, plot)
		userPrompt = reviewer.WithStyleGuide(v.ReadStyleGuide(), userPrompt)
		userPrompt = reviewer.WithPageInfo(vault.PageCount(text), v.TotalPageCount(), userPrompt)

		r := reviewer.New(role, a, cfg.Review.MaxIssues)
		out, err := r.Review(ctx, userPrompt)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("review failed: %v", err)), nil
		}

		return mcp.NewToolResultText(out), nil
	}
}

func makeCrossReviewHandler(cfg *Config, claudeAgent, codexAgent agent.Agent) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		reviewA, _ := req.RequireString("review_a")
		reviewB, _ := req.RequireString("review_b")

		modelA := "claude"
		if m, ok := req.GetArguments()["model_a"].(string); ok && m != "" {
			modelA = m
		}
		modelB := "codex"
		if m, ok := req.GetArguments()["model_b"].(string); ok && m != "" {
			modelB = m
		}

		agentFor := func(model string) agent.Agent {
			if model == "codex" {
				return codexAgent
			}
			return claudeAgent
		}

		rebuttalA, err := reviewer.CrossReview(ctx, agentFor(modelA), reviewA, reviewB, cfg.Review.MaxNewIssuesRound2)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("cross-review A→B failed: %v", err)), nil
		}

		rebuttalB, err := reviewer.CrossReview(ctx, agentFor(modelB), reviewB, reviewA, cfg.Review.MaxNewIssuesRound2)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("cross-review B→A failed: %v", err)), nil
		}

		// Return both rebuttals separated by a clear delimiter.
		result := fmt.Sprintf("## Rebuttal A (reviewer A responds to B)\n\n%s\n\n---\n\n## Rebuttal B (reviewer B responds to A)\n\n%s", rebuttalA, rebuttalB)
		return mcp.NewToolResultText(result), nil
	}
}

func makeSynthesizeHandler(cfg *Config, a agent.Agent) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		reviewsRaw, _ := req.RequireString("reviews")
		rebuttalsRaw := ""
		if r, ok := req.GetArguments()["rebuttals"].(string); ok {
			rebuttalsRaw = r
		}

		var reviews map[string]string
		if err := json.Unmarshal([]byte(reviewsRaw), &reviews); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("parse reviews: %v", err)), nil
		}

		rebuttals := make(map[string]string)
		if rebuttalsRaw != "" {
			if err := json.Unmarshal([]byte(rebuttalsRaw), &rebuttals); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("parse rebuttals: %v", err)), nil
			}
		}

		reviewNum := 0
		if n, ok := req.GetArguments()["review_number"].(float64); ok {
			reviewNum = int(n)
		}

		out, err := reviewer.Synthesize(ctx, a, reviews, rebuttals, reviewNum)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("synthesis failed: %v", err)), nil
		}

		return mcp.NewToolResultText(out), nil
	}
}

func makeExtractCanonHandler(cfg *Config, a agent.Agent) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		v := vaultFromReq(req)
		chapter, _ := req.RequireString("chapter")

		text, err := v.ReadChapter(chapter)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("read chapter: %v", err)), nil
		}

		canon, err := v.ReadCanonFiles()
		if err != nil {
			canon = make(map[string]string)
		}

		out, err := reviewer.ExtractCanon(ctx, a, text, canon, v.ReadStyleGuide(), vault.PageCount(text), v.TotalPageCount())
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("extraction failed: %v", err)), nil
		}

		return mcp.NewToolResultText(out), nil
	}
}

func makeDownstreamHandler(cfg *Config, a agent.Agent) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		v := vaultFromReq(req)
		chapter, _ := req.RequireString("chapter")

		edited, err := v.ReadChapter(chapter)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("read edited chapter: %v", err)), nil
		}

		namedChapters, err := v.ReadChaptersFrom(chapter)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("read downstream chapters: %v", err)), nil
		}

		// Skip the first one — that's the edited chapter itself.
		var downstream []struct{ Name, Content string }
		for _, ch := range namedChapters[1:] {
			downstream = append(downstream, struct{ Name, Content string }{ch.Name, ch.Content})
		}

		if len(downstream) == 0 {
			return mcp.NewToolResultText(`{"downstream_issues":[],"safe_chapters":[],"summary":"No downstream chapters found."}`), nil
		}

		canon, _ := v.ReadCanonFiles()
		plot, _ := v.ReadPlotFiles()

		// Count pages for the edited chapter + all downstream chapters
		downstreamWords := 0
		for _, ch := range downstream {
			downstreamWords += len(strings.Fields(ch.Content))
		}
		inputPages := vault.PageCount(edited) + (downstreamWords / 300)
		out, err := reviewer.AssessDownstream(ctx, a, edited, downstream, canon, plot, v.ReadStyleGuide(), inputPages, v.TotalPageCount())
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("downstream assessment failed: %v", err)), nil
		}

		return mcp.NewToolResultText(out), nil
	}
}

func makeManuscriptHandler(a agent.Agent) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		v := vaultFromReq(req)

		namedChapters, err := v.ReadAllChapters()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("read chapters: %v", err)), nil
		}

		var chapters []struct{ Name, Content string }
		for _, ch := range namedChapters {
			chapters = append(chapters, struct{ Name, Content string }{ch.Name, ch.Content})
		}

		priorSummary := ""
		if s, ok := req.GetArguments()["prior_review_summary"].(string); ok {
			priorSummary = s
		}

		out, sessionID, err := reviewer.ReviewManuscript(ctx, a, chapters, priorSummary, v.ReadStyleGuide(), v.ReadIssues(), v.TotalPageCount())
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("manuscript review failed: %v", err)), nil
		}

		// Return review + session ID so cross-review can resume
		result := map[string]string{
			"review":     out,
			"session_id": sessionID,
		}
		data, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(data)), nil
	}
}

func makeManuscriptRejectionHandler(a agent.Agent) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		v := vaultFromReq(req)

		namedChapters, err := v.ReadAllChapters()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("read chapters: %v", err)), nil
		}

		var chapters []struct{ Name, Content string }
		for _, ch := range namedChapters {
			chapters = append(chapters, struct{ Name, Content string }{ch.Name, ch.Content})
		}

		priorSummary := ""
		if s, ok := req.GetArguments()["prior_review_summary"].(string); ok {
			priorSummary = s
		}

		review, rejection, sessionID, err := reviewer.ReviewManuscriptWithRejection(ctx, a, chapters, priorSummary, v.ReadStyleGuide(), v.ReadIssues(), v.TotalPageCount())
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("manuscript review failed: %v", err)), nil
		}

		result := map[string]string{
			"review":     review,
			"rejection":  rejection,
			"session_id": sessionID,
		}
		data, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(data)), nil
	}
}

func makeAdversarialManuscriptHandler(a agent.Agent) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		v := vaultFromReq(req)

		namedChapters, err := v.ReadAllChapters()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("read chapters: %v", err)), nil
		}

		var chapters []struct{ Name, Content string }
		for _, ch := range namedChapters {
			chapters = append(chapters, struct{ Name, Content string }{ch.Name, ch.Content})
		}

		prompt := reviewer.AdversarialManuscriptPrompt(v.Root)
		userPrompt := reviewer.WithPageInfo(v.TotalPageCount(), v.TotalPageCount(),
			reviewer.WithStyleGuide(v.ReadStyleGuide(), reviewer.BuildManuscriptContext(chapters)))

		out, err := a.Run(ctx, prompt, userPrompt)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("adversarial review failed: %v", err)), nil
		}

		return mcp.NewToolResultText(out), nil
	}
}

func makeSummarizeReviewHandler(a agent.Agent) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		v := vaultFromReq(req)
		prefix, _ := req.RequireString("prefix")

		// Read only the synthesis portion (above the sentinel), which includes
		// issue IDs and any author rebuttals inline.
		prior, err := v.ReadLatestReviewSynthesis(prefix)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("read prior review: %v", err)), nil
		}
		if prior == "" {
			return mcp.NewToolResultText("No prior review found."), nil
		}

		return mcp.NewToolResultText(prior), nil
	}
}

func makeCrossReviewManuscriptHandler(cfg *Config, claudeAgent, codexAgent, geminiAgent agent.Agent) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		claudeReview, _ := req.RequireString("claude_review")
		codexReview, _ := req.RequireString("codex_review")
		geminiReview := ""
		if s, ok := req.GetArguments()["gemini_review"].(string); ok {
			geminiReview = s
		}

		claudeSessionID := ""
		if s, ok := req.GetArguments()["claude_session_id"].(string); ok {
			claudeSessionID = s
		}
		codexSessionID := ""
		if s, ok := req.GetArguments()["codex_session_id"].(string); ok {
			codexSessionID = s
		}
		geminiSessionID := ""
		if s, ok := req.GetArguments()["gemini_session_id"].(string); ok {
			geminiSessionID = s
		}

		// Build counterpart text for each reviewer
		claudeCounterparts := "## Codex's Review\n\n" + codexReview
		codexCounterparts := "## Claude's Review\n\n" + claudeReview
		geminiCounterparts := "## Claude's Review\n\n" + claudeReview + "\n\n---\n\n## Codex's Review\n\n" + codexReview

		if geminiReview != "" {
			claudeCounterparts += "\n\n---\n\n## Gemini's Review\n\n" + geminiReview
			codexCounterparts += "\n\n---\n\n## Gemini's Review\n\n" + geminiReview
		}

		// Claude rebuts others
		claudeRebuttal, err := reviewer.CrossReviewResume(ctx, claudeAgent, claudeSessionID, claudeCounterparts, cfg.Review.MaxNewIssuesRound2)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("claude cross-review failed: %v", err)), nil
		}

		// Codex rebuts others
		codexRebuttal, err := reviewer.CrossReviewResume(ctx, codexAgent, codexSessionID, codexCounterparts, cfg.Review.MaxNewIssuesRound2)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("codex cross-review failed: %v", err)), nil
		}

		result := fmt.Sprintf("## Claude's Rebuttal\n\n%s\n\n---\n\n## Codex's Rebuttal\n\n%s", claudeRebuttal, codexRebuttal)

		// Gemini rebuts others (if present)
		if geminiReview != "" && geminiAgent != nil {
			geminiRebuttal, err := reviewer.CrossReviewResume(ctx, geminiAgent, geminiSessionID, geminiCounterparts, cfg.Review.MaxNewIssuesRound2)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("gemini cross-review failed: %v", err)), nil
			}
			result += fmt.Sprintf("\n\n---\n\n## Gemini's Rebuttal\n\n%s", geminiRebuttal)
		}

		return mcp.NewToolResultText(result), nil
	}
}

func makeConsultHandler(codexAgent, geminiAgent agent.Agent) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		question, _ := req.RequireString("question")
		context_text := ""
		if c, ok := req.GetArguments()["context"].(string); ok {
			context_text = c
		}

		systemPrompt := `You are a fiction writing consultant. Answer the question directly and concisely.
Ground your answer in the provided context if any. Be specific — quote passages when relevant.
Do not hedge or disclaim. Give your honest assessment.`

		userPrompt := question
		if context_text != "" {
			userPrompt = "Context:\n\n" + context_text + "\n\n---\n\nQuestion: " + question
		}

		var results []string

		if codexAgent != nil {
			resp, err := codexAgent.Run(ctx, systemPrompt, userPrompt)
			if err != nil {
				results = append(results, fmt.Sprintf("## Codex\n\n*Error: %v*", err))
			} else {
				results = append(results, fmt.Sprintf("## Codex\n\n%s", resp))
			}
		}

		if geminiAgent != nil {
			resp, err := geminiAgent.Run(ctx, systemPrompt, userPrompt)
			if err != nil {
				results = append(results, fmt.Sprintf("## Gemini\n\n*Error: %v*", err))
			} else {
				results = append(results, fmt.Sprintf("## Gemini\n\n%s", resp))
			}
		}

		if len(results) == 0 {
			return mcp.NewToolResultError("no external models are enabled — enable codex or gemini in /critic:settings"), nil
		}

		return mcp.NewToolResultText(strings.Join(results, "\n\n---\n\n")), nil
	}
}

func normalizeIssueID(id string) string {
	id = strings.TrimSpace(id)
	// Accept "003-01" or "ISSUE-003-01"
	if !strings.HasPrefix(strings.ToUpper(id), "ISSUE-") {
		id = "ISSUE-" + id
	}
	return strings.ToUpper(id)
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
