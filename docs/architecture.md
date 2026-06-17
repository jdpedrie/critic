# Architecture

How the pieces fit together.

## Two layers

```
       cowork session (Claude)
              │
   ┌──────────┼──────────┐
   │          │          │
   ▼          ▼          ▼
 Task     invoke-codex  invoke-pi      ← MCP tools (Go server)
  │          │          │
  ▼          ▼          ▼
Claude     Codex      Pi
subagent   CLI        CLI
```

The plugin is a Go MCP server and a set of skills (markdown files Claude follows). The Go server is thin. It does file I/O on the vault, manages review files, loads prompt templates, and shells out to the Codex and Pi CLIs. The skills do everything that needs reasoning: composing prompts, dispatching subagents, presenting results, asking the author questions.

The split matters. Anything that requires judgment ("which reviewer ran best on chapter 3", "rephrase this rebuttal so it reads as authoritative") happens in the cowork session where Claude can think. Anything that's pure plumbing ("write this snapshot to disk", "load this prompt template, substitute these vars") happens in Go where it's deterministic and fast.

## The three reviewer surfaces

Three reviewers, each reached differently.

Claude runs as a `Task` subagent inside the cowork session. No external process. The orchestrating Claude (the one running the skill) spawns subagents via the `Task` tool. The subagent gets its own context window, returns its review, and is gone.

Codex is the OpenAI Codex CLI authenticated via your ChatGPT subscription. The Go server wraps it through the `codex-go-sdk` Go binding, which talks to the CLI over its own protocol. The server keeps the Codex `thread_id` so the orchestrator can resume a session for cross-review.

Pi is the [Pi harness](https://pi.dev), a unified CLI that fronts multiple model providers (Anthropic, OpenAI, Google, others). The server shells out to `pi -p` (one-shot mode), maintaining session continuity in-memory by replaying message history per call. Provider and model are configurable per session at creation time.

The asymmetry is intentional. Claude lives inside the cowork session because spawning a Task subagent there is cheap and gives true context continuity for the review-plus-rejection pattern. Codex and Pi need wrapping because the cowork session can't run external CLIs directly.

## Session continuity for cross-review

The cross-review matrix has each reviewer rebut the others. That's only useful if each reviewer remembers what they said.

Codex has real sessions. The server holds `thread_id`s in a per-process map and resumes the thread for the rebuttal call.

Pi has emulated sessions. `pi -p` is stateless, so the server replays the full message history every call, holding turns in an in-memory map keyed by a synthetic `pi-N` session id.

Claude subagents are stateless too. Each Task subagent is a fresh context. The orchestrator gets continuity by inlining the subagent's prior review in the new subagent's prompt. The review-and-rejection-pass pattern goes further: it folds both steps into one subagent so the rejection has true context, not emulated.

## Skills as orchestrators

A skill is a markdown file in `skills/<name>/SKILL.md` that Claude reads when the user types `/critic:<name>`. The frontmatter tells Claude when to use it; the body tells Claude what to do.

Skills compose prompts (via `get-prompt` MCP tool), inline data (via the various `read-*` and `assemble-*` tools), and dispatch work to subagents (via the `Task` tool) or external reviewers (via `invoke-codex` / `invoke-pi`).

The split between "skill" and "server tool" follows a rule. Anything that needs to read context and decide what to do next is a skill. Anything that's a pure operation on the filesystem or a wrapped external process is a server tool.

The full skill list lives in [skills.md](skills.md).

## Vault layer

The Go server's `vault` package (`server/vault/`) wraps the storyline project on disk. Two files.

`vault.go` does project discovery (find the `<Title>.md` file with `type: storyline` frontmatter), scene loading (walk `Scenes/`, parse frontmatter, sort by act/chapter/sequence), manuscript assembly (emit the same Markdown format storyline's "Export project" produces), Codex and Research access, and the auto-derived stage block.

`review.go` handles review files, issues, snapshots, summaries, and the reviewer memory hook. Snapshots are assembled by calling `ReadManuscript()` so they're always in the storyline export format. Diffs are unified diffs against the prior snapshot.

The vault layer is the only place that knows about the storyline file layout. Every skill, every MCP tool, every prompt template treats the vault as an opaque thing addressed by act/chapter/sequence or by entity name.

Details in [vault.md](vault.md).

## Prompt resolution

Prompt templates live in `server/prompts/*.md` and are embedded into the Go binary via `//go:embed`. The `get-prompt` tool resolves a prompt name in this order:

1. `<vault>/prompts/<name>.md` (per-project author override)
2. `$CLAUDE_PLUGIN_ROOT/prompts/<name>.md` (plugin-level override)
3. The embedded default

This means an author can edit the manuscript-reviewer's framing for their specific project without rebuilding the plugin. Templated prompts use Go `text/template` syntax (`{{.Role}}`, `{{.MaxIssues}}`, `{{.ReviewNum}}`).

Catalog in [prompts.md](prompts.md).

## Review file format

```
review/NNN-prefix-YYYY-MM-DD-HHMMSS.md

  [synthesis with ISSUE-NNN-NN ids: the human-readable report]

  <!-- RAW AGENT OUTPUTS BELOW: NOT INCLUDED IN FUTURE REVIEW CONTEXT -->

  # Claude Review
  # Claude Rejection Pass
  # Codex Review
  # Pi Review
  # Pi Adversarial Pass
  # Claude Cross-Review Rebuttal
  ...
```

The numeric prefix is a global counter across every review type. Issue IDs follow `ISSUE-NNN-NN` where `NNN` matches the review number and `NN` is a sequential per-document counter. The sentinel splits the synthesis (which future reviewers see) from the raw agent outputs (which are retained for traceability but not fed back into future review prompts).

Author rebuttals are inserted inline after the relevant issue as Obsidian callouts:

```markdown
### ISSUE-003-04: Henry's motivation muddied in chapter 5

> [!quote] Author Rebuttal (ISSUE-003-04)
> Intentional. The ambiguity resolves in chapter 7 when he chooses.
```

Future reviewers see the rebuttal and are told to respect it unless the issue has materially escalated.

## Why this shape

A few choices worth calling out explicitly because they constrain everything downstream.

Skills are markdown. They're versioned alongside the code, the author can read them, and they get the benefit of Claude's reasoning at runtime. The alternative (encoding the workflow in Go) would be deterministic but inflexible. Reviews need judgment.

The server is thin. Anything that requires reasoning happens in the cowork session. The server doesn't know the difference between a manuscript review and a chapter review; it just exposes tools (`assemble-manuscript`, `assemble-chapter`, `read-scene`, etc.) that skills compose into workflows.

Storyline is the layout. Reusing the obsidian-storyline plugin's data model means we get rich frontmatter (act, chapter, sequence, characters, location, etc.) for free, and the manuscript snapshot format matches the plugin's own export byte-for-byte. The cost is hard-coding to that one plugin. The benefit is leverage on an active ecosystem of fiction-writing tools.

Reviewers run blind. The three reviewers running in parallel during a manuscript review have no knowledge of each other. Cross-review forces engagement. This is more expensive than a single reviewer but produces real disagreements you can act on.

Issue IDs are stable. A review writes `ISSUE-005-07` to disk, and every subsequent review references the same ID if the underlying issue persists. Author rebuttals and deferrals chain off the ID. This is the durable spine the system rests on.
