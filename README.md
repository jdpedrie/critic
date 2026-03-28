# critic

A Claude Code plugin for multi-agent fiction critique. Runs structured reviews using independent AI reviewers (Claude, Codex, and Gemini), cross-model disagreement, and synthesis into human-readable feedback.

## How it works

The plugin provides a Go MCP server that exposes review tools, and Claude Code skills that orchestrate them. You interact through cowork — the skills dispatch tools in parallel, run cross-review with session continuity, synthesize results, and save the output to your vault.

### Review pipeline (chapter)

1. **Four independent reviewers** run in parallel, each with a constrained role and limited context
2. **Cross-review** — reviewers challenge each other's claims in pairwise rebuttals
3. **Synthesis** — all outputs are combined into a readable critique

Two reviewers see only the chapter text (reader perspective). Two see the full world state and plot (author perspective). Two run on Claude, two on Codex. Diversity is enforced at every level.

### Manuscript pipeline

1. **Three independent reviewers** (Claude, Codex, Gemini) read the full manuscript — text only, no world context
2. **Cross-review** — each model resumes its session and rebuts the others with full context from its own original analysis
3. **Synthesis** — combined into a readable report
4. **Saved** to `review/manuscript-critic-<timestamp>.md`

Prior review summaries are automatically loaded so reviewers can assess whether previous issues were addressed.

### Model assignment (chapter review)

| Role | Perspective | Model |
|------|-------------|-------|
| Analytical reader | Text-only | Claude |
| Immersive reader | Text-only | Codex |
| Structural analyst | Full context | Claude |
| Adversarial critic | Full context | Codex |

## Vault structure

The plugin expects an Obsidian vault with this layout:

```
vault/
  story/
    chapter-01.md
    chapter-02.md
    ...
  world/           (optional)
    *.md            (any nesting — characters/, locations/, etc.)
  plot/            (optional)
    outline.md
    arcs/
    timeline.md
  review/          (created automatically)
  system/          (created automatically)
    reviewer-memory/
```

### Requirements

- **`story/`** — One markdown file per chapter. Files are sorted alphabetically to determine chapter order. Name them with zero-padded numbers (`chapter-01.md`, `chapter-02.md`) or any scheme that sorts correctly.
- **`world/`** — World-building notes. Any directory structure, any markdown format. No frontmatter required. These are read as plain text and passed to full-context reviewers and the canon extraction tool.
- **`plot/`** — Plot outlines, arc descriptions, timelines. Same rules as `world/`. Optional — if missing, full-context reviewers work without it.
- **`review/`** — Created by the plugin. Reviews are saved as timestamped markdown files.
- **`system/`** — Created by the plugin for reviewer memory files.

No frontmatter is required on any file. The plugin reads all markdown files as plain text.

## Installation

### Prerequisites

- Claude Code 1.0.33+
- Go 1.21+ (for building the server)
- `codex` CLI installed and logged in (for Codex reviewer — uses subscription auth)

### Setup

```bash
# Build the server
cd critic/server
go build -o ../bin/critic .

# Load the plugin (development)
claude --plugin-dir /path/to/critic
```

### Configuration

Run `/critic:settings` to configure the plugin:

```
/critic:settings vault_path /path/to/vault
/critic:settings gemini_api_key AIza...
/critic:settings claude_model claude-sonnet-4-6
/critic:settings codex_model gpt-5.4-codex
/critic:settings gemini_model gemini-2.5-flash
```

To disable a provider:
```
/critic:settings codex_enabled false
```

To view current settings:
```
/critic:settings
```

Settings are stored in the plugin data directory and persist across sessions. API keys can also be provided via environment variables (`ANTHROPIC_API_KEY`, `GEMINI_API_KEY`) or the plugin's `userConfig` if running from the CLI.

Default model and review settings are in `config.yaml` — the settings skill overrides these.

## Skills

### `/critic:review <chapter>`

Run a multi-agent review on a single chapter.

Dispatches all four reviewers in parallel, runs cross-review, and synthesizes into a readable critique. Runs non-interactively through all steps.

### `/critic:extract <chapter>`

Extract ground truth from a chapter and diff against the world state.

Pulls every factual assertion (character states, relationships, locations, timeline events, rules) and classifies each as confirmed, new, or contradictory relative to existing canon.

### `/critic:downstream <chapter>`

Assess downstream effects after editing a chapter.

Reads from the edited chapter through the end of the manuscript. Flags continuity breaks, invalidated setups, character state errors, dialogue references to removed content, and timeline issues in all subsequent chapters.

### `/critic:manuscript`

Review the full manuscript at the book level.

Three independent book-level reviews (Claude, Codex, Gemini), cross-review with session resumption, then synthesis. Text-only — no world-building notes or plot outlines. Assesses arc completion, pacing across chapters, character consistency, dangling threads, and tonal drift.

Automatically loads the prior review summary so reviewers can check whether previous issues were addressed. Saves the full review (synthesis + raw outputs + rebuttals) to `review/manuscript-critic-<timestamp>.md`.

### `/critic:settings`

View and update plugin settings — vault path, API keys, model selection, enable/disable providers.

## MCP Tools

The Go server exposes these tools (called by the skills, not directly by users):

| Tool | Description |
|------|-------------|
| `review-analytical` | Text-only review: clarity, coherence |
| `review-immersive` | Text-only review: engagement, pacing, emotion |
| `review-structural` | Full-context review: continuity, causality, plan alignment |
| `review-adversarial` | Full-context review: contradictions, weak motivations |
| `cross-review` | Pairwise rebuttal between two reviews |
| `cross-review-manuscript` | Session-resuming cross-review for manuscript reviews (2 or 3 models) |
| `synthesize` | Combine reviews + rebuttals into readable report |
| `extract-canon` | Fact extraction + canon diff |
| `assess-downstream` | Downstream effect assessment |
| `review-manuscript-claude` | Full manuscript review (Claude) |
| `review-manuscript-codex` | Full manuscript review (Codex) |
| `review-manuscript-gemini` | Full manuscript review (Gemini) |
| `summarize-review` | Summarize the most recent review for a given prefix |
| `save-review` | Save a review to the vault's `review/` directory |
| `read-settings` | Read current plugin settings |
| `write-setting` | Update a plugin setting |
| `update-memory` | Write reviewer memory files |

## Planned enhancements

- **Reviewer memory**: The `update-memory` tool exists but memory is not yet read back into reviewer context automatically. Planned: inject reviewer memory into each review pass, with compression to prevent drift.
- **Canon retrieval refinement**: Currently full-context reviewers receive all canon/plot files. Planned: extract entity names from the chapter first, then pull only matching canon files.
- **Diff-aware review**: Accept a diff or change summary to prioritize review of changed sections rather than re-reviewing the entire chapter.
- **Multi-model canon extraction**: Run extraction with both Claude and Gemini independently, diff their findings for higher precision.
- **Multi-model downstream assessment**: Two models reading the same text catch different continuity breaks.
- **Trend tracking**: Track recurring issues across chapters and surface patterns.
- **Character voice consistency**: Automated extraction of speech patterns per character, checked across chapters.
- **Synthesizer debiasing**: Test and mitigate potential bias toward Claude reviewers in synthesis (since the synthesizer is also Claude).
