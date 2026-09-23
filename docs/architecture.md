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

## Harness-agnostic mode

The server also exposes `invoke-claude`, wrapping headless `claude -p` with real session resume. With it, all three reviewers are symmetric MCP tools, and the leader doesn't have to be Claude: register the critic server with any MCP-speaking harness (Codex CLI via `[mcp_servers]` in `~/.codex/config.toml`, for example) and that harness can run the playbook, dispatching Claude, Codex, and Pi uniformly through `invoke-*` calls.

Orchestration stays in markdown regardless of leader. It was moved out of Go deliberately: review workflows need runtime judgment (summarising diffs, deciding what to surface, conversing with the author), and a markdown playbook interpreted by an LLM leader keeps that judgment where it belongs. The server never sequences a review.

Inside cowork, Task subagents remain the default for Claude work; `invoke-claude` is for non-Claude leaders.

## Invocations outlive tool calls

External reviewers are reached through MCP tool calls, and a tool call is a
short-lived thing. A manuscript review is not. Rather than fight that, every
`invoke-*` call starts a background job rooted at the server's context and
returns a handle once its wait budget is up; the leader collects the result
with `invoke-status`. Short calls still answer inline, so a quick consult is
unchanged. See [server.md](server.md) for the mechanics.

The practical consequence for skills: start every reviewer, then collect them.
Polling one to completion before starting the next serialises work that is
meant to overlap.

## Session continuity for cross-review

The cross-review matrix has each reviewer rebut the others. That's only useful if each reviewer remembers what they said.

Codex has real sessions. The server holds `thread_id`s in a per-process map and resumes the thread for the rebuttal call.

Pi has emulated sessions. `pi -p` is stateless, so the server replays the full message history every call, holding turns in an in-memory map keyed by a synthetic `pi-N` session id.

Claude subagents are stateless too. Each Task subagent is a fresh context. The orchestrator gets continuity by inlining the subagent's prior review in the new subagent's prompt. The review-and-rejection-pass pattern goes further: it folds both steps into one subagent so the rejection has true context, not emulated.

## Two frames

The anti-flattery machinery (a third-party principal, the rejection pass, the adversary, cross-review) is the same everywhere. What the reviewers want for the book is set by the frame, which is a choice of prompt files.

The publication frame is the original: the reviewer advises a literary agent, the bar is a saleable finished book, and prior issues are tracked across reviews. The craft frame swaps in six prompts (`craft-framing.md`, `manuscript-craft.md`, `verdict-craft.md`, `rejection-pass-craft.md`, `adversarial-craft.md`, `synthesis-craft.md`): the reviewer advises the author's developmental editor, the bar is the author's stated intent, and market, pace, and the ledger of prior issues are out of scope.

Whole-book reviews are separate skills per frame (`manuscript-craft`, `manuscript-publication`) because the two products differ in shape. Slice reviews, the delta review, downstream, and consult take the frame from the `frame` setting.

Each frame is also a snapshot lineage. A snapshot records which review took it (lineage, kind, number) in a sidecar, and "previous snapshot" means the newest in the same lineage. Craft reviews diff against craft snapshots and load craft reviews as prior context; publication reviews do the same with theirs; the two never cross. That keeps a publication review from measuring against a baseline a craft review set, and the reverse.

The delta review (`/critic:delta`) is the third product: the server compares the new snapshot with the prior one scene by scene, and reviewers get the whole manuscript for context with the added and modified scenes as the target. It answers whether one round of changes did what the author meant, and whether it's good.

## Skills as orchestrators

A skill is a markdown file in `skills/<name>/SKILL.md` that Claude reads when the user types `/critic:<name>`. The frontmatter tells Claude when to use it; the body tells Claude what to do.

Skills compose prompts (via `get-prompt` MCP tool), inline data (via the `read-*` and `assemble-*` tools), and dispatch work to subagents (via the `Task` tool) or external reviewers (via `invoke-claude` / `invoke-codex` / `invoke-pi`).

The split between "skill" and "server tool" follows a rule. Anything that needs to read context and decide what to do next is a skill. Anything that's a pure operation on the filesystem or a wrapped external process is a server tool.

The full skill list lives in [skills.md](skills.md).

## Vault layer

The Go server's `vault` package (`server/vault/`) wraps the book folder on disk. Two files.

`vault.go` reads the book note, parses chapter files in `Story/` (splitting scenes on `## ` headings and pairing them with the `scenes:` metadata), assembles the manuscript, reads the Codex and Research from `Background/`, and derives the stage block.

`review.go` handles review files, issues, snapshots, summaries, and the reviewer memory hook. Snapshots are assembled by calling `ReadManuscript()`, so a snapshot is exactly what reviewers see. Diffs are unified diffs against the prior snapshot in the same lineage, plus a scene-level comparison (added, modified, removed, with the changed prose) that the delta review is built on.

The vault layer is the only place that knows the file layout. Every skill, every MCP tool, every prompt template treats the vault as an opaque thing addressed by chapter number, scene ID (`CC-SS`), or entity name.

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
Review/NNN-prefix-YYYY-MM-DD-HHMMSS.md

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

Plain files are the layout. A chapter is a markdown file; a scene is a `## ` heading in it; scene metadata is a list in the chapter's frontmatter. No Obsidian plugin has to be installed or running, and the author edits prose where they read it. We started on the obsidian-storyline plugin's one-file-per-scene model and left it because it was too unwieldy to write in. Two things survived the move on purpose: scene IDs keep the old `CC-SS` filename prefixes, so every review that cites `04-02` still resolves, and the assembled manuscript keeps the storyline export shape, so snapshots from before and after diff cleanly.

Reviewers run blind. The three reviewers running in parallel during a manuscript review have no knowledge of each other. Cross-review forces engagement. This is more expensive than a single reviewer but produces real disagreements you can act on.

Issue IDs are stable. A review writes `ISSUE-005-07` to disk, and author rebuttals, deferrals, and `/critic:assess` chain off the ID. In the publication frame, later reviews reference the same ID if the issue persists. In the craft frame they don't: the ID still anchors the author's response, but reviewers are told not to keep a ledger, because for a book nobody is selling, "still open after four reviews" is a fact about the author's priorities and not about the pages.
