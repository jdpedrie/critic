# critic

A Claude Code plugin for multi-reviewer fiction critique. The plugin runs parallel reviews from Claude (as a subagent inside cowork), Codex (via the OpenAI Codex CLI), and Pi (via the [Pi harness](https://pi.dev)). It cross-reviews, synthesises into ranked issues with stable IDs, and saves to your vault for future reviews to build on.

The vault has to be an [obsidian-storyline](https://github.com/PixeroJan/obsidian-storyline) project. Scenes live in `Scenes/`, characters and locations in `Codex/`, worldbuilding in `Research/`. The critic reads scenes in `act → chapter → sequence` order, exactly as storyline's own "Export project" command does, and uses the rich frontmatter to scope reviews and pre-filter context.

## Quickstart

```bash
# Build the server
cd critic/server
go build -o ../bin/critic .

# Load the plugin
claude --plugin-dir /path/to/critic

# Point critic at your storyline project (the folder containing Scenes/, Codex/, Research/)
/critic:settings vault_path /path/to/vault/MyNovel

# Configure Pi (optional)
/critic:settings pi_provider google
/critic:settings pi_model gemini-2.5-pro
```

The first review:

```
/critic:manuscript
```

Interactive. Pick which reviewers run, which Pi model, which steps. Then it runs straight through.

## Documentation

- [Goals](docs/goals.md). What the critic is for and what it deliberately isn't.
- [Architecture](docs/architecture.md). How the pieces fit together.
- [Vault layout](docs/vault.md). What files live where and what critic reads or writes.
- [Skills](docs/skills.md). Every `/critic:*` command in detail.
- [Server](docs/server.md). The Go MCP server: tools, vault layer, agent wrappers.
- [Prompts](docs/prompts.md). Every prompt template, who uses it, override rules.
- [Workflows](docs/workflows.md). Example sessions from a fresh project through a revision cycle.

## Skills, briefly

| Skill | What it does |
|-------|--------------|
| `/critic:manuscript [author note]` | Full manuscript review with interactive setup. Three reviewers, optional adversary, cross-review matrix, synthesis. |
| `/critic:review chapter <N>` / `scene <file>` | Four-role review (analytical, immersive, structural, adversarial) scoped to one slice. |
| `/critic:close-read scene <file>` / `chapter <N>` / `all` | Line-editor and copy-editor pass. Subagent per scene. |
| `/critic:downstream chapter <N>` / `scene <file>` | What breaks in later scenes after the edit. |
| `/critic:extract chapter <N>` / `scene <file>` / `entity <name>` | Reconcile prose against the Codex; maintains `.claude/codex-inventory.md`. |
| `/critic:summarize` | Per-chapter summaries to `summary/`. |
| `/critic:consult <question>` | Quick second opinions from Codex and Pi. |
| `/critic:rebuttal <issue-id>` | Rebut, defer, or accept a flagged issue. |
| `/critic:assess <issue-id>` | Deep-dive on a single issue. |
| `/critic:settings` | View and update plugin settings. |
| `/critic:help` | Skill reference. |

See [docs/skills.md](docs/skills.md) for the full reference.

## Prerequisites

- [Claude Code](https://claude.com/claude-code) 1.0.33+
- Go 1.21+ to build the server
- [obsidian-storyline](https://github.com/PixeroJan/obsidian-storyline) plugin set up on an active project
- [Codex CLI](https://developers.openai.com/codex/) authenticated (`codex login`). Optional, disable via `/critic:settings codex_enabled false`.
- [Pi CLI](https://pi.dev) installed and authenticated. Optional, disable via `/critic:settings pi_enabled false`.

Claude itself runs as a Task subagent inside cowork. No external process, nothing to configure.
