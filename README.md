# critic

A Claude Code plugin for multi-reviewer fiction critique. It runs parallel reviews from Claude (as a subagent inside cowork), Codex (via the OpenAI Codex CLI), and Pi (via the [Pi harness](https://pi.dev)), has them rebut each other, and synthesises the result into ranked issues with stable IDs. Reviews save to your vault so later reviews know what came before.

The vault is a plain folder. Chapters live in `Story/`, one markdown file each, with scenes as `## ` headings. Characters and locations live in `Background/Characters/` and `Background/Locations/`; worldbuilding docs sit alongside them in `Background/`. See [docs/vault.md](docs/vault.md).

## Why it exists

Ask a model for feedback on a chapter and you get a compliment sandwich. The critic gets around that by having reviewers report to someone other than you, running a rejection pass and a frank reader against the constructive reviews, and forcing the reviewers to argue with each other before anything is synthesised. The result is feedback you can act on.

Two frames decide what the reviewers are measuring against.

The publication frame is the original. Reviewers advise the literary agent who represents you, the bar is a finished book an acquiring editor would buy, and prior issues are tracked across reviews. Use it when a publisher is in the picture.

The craft frame is for everything else. Reviewers advise your developmental editor. The bar is still professional craft, but the measure is what you say the book is trying to be, not a market. Nobody comments on your pace, nobody counts how many reviews an issue has survived, and an issue you haven't fixed is treated as your sequencing decision rather than an oversight. Use it when you're writing the book you want to write.

Both frames keep the arm's length. The difference is what the reviewers want for the book.

## Install

Build the server and load the plugin:

```bash
cd critic/server && go build -o ../bin/critic .
```

```bash
claude --plugin-dir /path/to/critic
```

Point it at your book folder (the one containing `Story/`) and pick a frame:

```
/critic:settings vault_path /path/to/vault/MyNovel
/critic:settings frame craft
```

Configure Pi if you use it. The frank reader gets its own model so the panel has two aesthetics:

```
/critic:settings pi_provider google
/critic:settings pi_model gemini-2.5-pro
/critic:settings adversary_provider openai
/critic:settings adversary_model gpt-5
```

Codex needs `codex login`. Pi needs its own auth. Either can be turned off (`codex_enabled false`, `pi_enabled false`); Claude always runs.

## Tell the reviewers what the book is

Write `stage.md` at the vault root. Without it the server derives a stage block from chapter counts, which is honest but says nothing about intent. Two paragraphs are enough, and the goals section is what the craft frame measures against:

```markdown
# Current Stage

Roughly three quarters of act 1 of a planned three-act novel.
Target ~120,000 words. Current ~26,000.

## What this project is

A book I write for my own satisfaction, when I feel like it. No deadline,
no submission target. Judge the pages, not the pace.

## What I want this book to be

- A slow-burn mystery where the strangeness is felt before it's explained
- A relationship under power asymmetry that neither party fully sees
- Prose that stays close and physical; no narrator editorialising

## What this draft is currently trying to do

- Establish the Gateway anomaly
- Set up the financial pressure that drives act 2

## Not yet attempted, by design

- Resolution of any major thread
- Character arcs completing
```

The stage block goes first in every reviewer's prompt. Reviewers are told it's authoritative: they don't flag what you haven't attempted, and in the craft frame they don't flag what you've said is out of scope.

A style guide at `style.md` (or `Background/style.md`) is passed to every reviewer too. If yours invites market or acquisition framing, the craft frame tells reviewers to disregard that invitation.

## The loop

Most of the time you want the small review, not the big one.

After a burst of drafting or revision, run the delta review with a note saying what you were trying to do:

```
/critic:delta gave Luma a decision in the Geneva scene instead of another summons
```

The reviewers get the whole manuscript for context and the scenes you added or changed as the target. The report answers four things: what you set out to do, whether it landed, whether the new material is as good as the best of what's already there, and what it cost elsewhere. Then issues with IDs, strengths to protect, and a verdict on the changes. Non-interactive; it runs with the configured reviewers.

Once an act, or when you want a whole-book read, run the manuscript review:

```
/critic:manuscript-craft
/critic:manuscript-craft new chapters 13 and 14; the act turn is on the page now
```

Interactive setup (which reviewers, which Pi models, which steps), then it runs straight through. If there are changes since the last review it opens with the same four questions the delta asks, then reads the foundations, pacing, character work, threads, tone, and prose as a whole.

If a publisher enters the picture, the same run under the other frame:

```
/critic:manuscript-publication
```

Each frame keeps its own snapshot lineage. A craft review diffs against the last craft review, a publication review against the last publication review, and neither ever sees the other's baseline. Every snapshot records which review took it; `list-snapshots` shows the history. Switching frames starts fresh: the first review in a lineage has nothing to diff against and says so.

## Reading a review

Each review is one file under `Review/`, numbered globally: `009-delta-craft-...`, `010-manuscript-craft-...`. The synthesis is at the top. The raw reviewer outputs and rebuttals sit below a sentinel line, kept for the record and never fed back into later reviews.

Every issue has an ID like `ISSUE-009-03`. You have three moves on an issue you don't just fix:

- `/critic:rebuttal ISSUE-009-03 Intentional; the ambiguity resolves in chapter 7.` Adds a callout inline. Reviewers treat it as a decision.
- `/critic:rebuttal ISSUE-009-03` then choose defer. The issue goes to `issues.md`; reviewers see it and don't re-raise it.
- `/critic:assess ISSUE-009-03 Is Henry actually passive, or only in this scene?` A conversational dig into one issue, pulling only the scenes it needs.

In the craft frame you can also do nothing. Silence is deferral. Rebuttals still help because they tell reviewers why.

Don't take the synthesis as gospel. It's an aggregation. When a note surprises you, read what the individual reviewers said below the sentinel.

## Narrower tools

`/critic:review chapter 5` runs a four-role review (analytical, immersive, structural, adversarial) on one chapter or one scene, with cross-review and synthesis. It honours the `frame` setting.

`/critic:close-read chapter 4` is the copy-editor. One subagent per scene, quote-and-fix output for typos, prose slips, and style-guide breaches. The other reviewers are told to leave those alone; this is where they get caught. `all` runs it over the whole manuscript.

`/critic:downstream chapter 3` reads a chapter you restructured plus everything after it, and reports what broke: invalidated setups, dialogue that references cut content, timeline slips.

`/critic:extract chapter 4` reconciles the prose against the Codex (character and location files) and proposes updates. Every change is approved by hand.

`/critic:consult Is the trust fund a real threat or just lampshading?` gets a short second opinion from Codex and Pi.

## Commands

| Skill | What it does |
|-------|--------------|
| `/critic:delta [note]` | Review the changes since the last review against your note. The frequent one. |
| `/critic:manuscript-craft [note]` | Whole-book review, craft frame. Interactive setup. |
| `/critic:manuscript-publication [note]` | Whole-book review, publication frame. Interactive setup. |
| `/critic:review chapter <N>` / `scene <id>` | Four-role review of one slice. |
| `/critic:close-read scene <id>` / `chapter <N>` / `all` | Copy-edit pass, one subagent per scene. |
| `/critic:downstream chapter <N>` / `scene <id>` | What breaks after an edit. |
| `/critic:extract chapter <N>` / `scene <id>` / `entity <name>` | Reconcile prose against the Codex. |
| `/critic:summarize` | Per-chapter summaries to `summary/`. |
| `/critic:consult <question>` | Second opinions from Codex and Pi. |
| `/critic:rebuttal <issue-id>` | Rebut, defer, or accept an issue. |
| `/critic:assess <issue-id> [question]` | Dig into one issue. |
| `/critic:settings [key] [value]` | View and update settings. |
| `/critic:help` | Skill reference. |

Full reference in [docs/skills.md](docs/skills.md).

## Settings

| Key | What it does |
|-----|--------------|
| `vault_path` | The book folder. Required. |
| `frame` | `craft` or `publication` (default). Applies to `review`, `delta`, `downstream`, `consult`. The manuscript skills are named by frame. |
| `codex_enabled`, `codex_model` | Codex on/off, model override. |
| `pi_enabled`, `pi_provider`, `pi_model` | Pi on/off, default provider and model. |
| `adversary_provider`, `adversary_model` | Frank reader's Pi provider and model for `delta`. Defaults to the Pi settings. |
| `claude_enabled`, `claude_model` | The `invoke-claude` tool, for non-Claude leaders only. |

## Tuning the reviewers

Every prompt is a markdown file. Drop a copy at `<vault>/prompts/<name>.md` and the server uses it instead of the embedded default, no rebuild. The two files that set the tone are `craft-framing.md` and `agent-framing.md`; the catalog is in [docs/prompts.md](docs/prompts.md).

## When things go wrong

A full-manuscript review outruns a single tool call. The `invoke-*` tools return a job handle and the skill polls it. If a run dies partway, finished reviews are on disk under the path each job reported, and `invoke-status` with no arguments lists what the server still holds.

If a reviewer errors, the skill stops and asks: retry, continue without it, or abort. It never synthesises around a gap.

The first `/critic:delta` in a lineage has nothing to diff against. It writes the baseline and says so; the next one reviews changes from there. A run that dies after snapshotting doesn't lose you anything: the retry skips the orphaned snapshot and diffs from the last review that landed.

If reviews keep telling you things you already know, the fix is usually `stage.md`, not the prompts.

## Documentation

- [Goals](docs/goals.md). What the critic is for and what it deliberately isn't.
- [Architecture](docs/architecture.md). How the pieces fit together.
- [Vault layout](docs/vault.md). What files live where and what critic reads or writes.
- [Skills](docs/skills.md). Every `/critic:*` command in detail.
- [Server](docs/server.md). The Go MCP server: tools, vault layer, agent wrappers.
- [Prompts](docs/prompts.md). Every prompt template, both frames, override rules.
- [Workflows](docs/workflows.md). Example sessions from a fresh project through a revision cycle.

## Prerequisites

- [Claude Code](https://claude.com/claude-code) 1.0.33+
- Go 1.21+ to build the server
- [Codex CLI](https://developers.openai.com/codex/) authenticated (`codex login`). Optional.
- [Pi CLI](https://pi.dev) installed and authenticated. Optional.

Claude itself runs as a Task subagent inside cowork. No external process, nothing to configure.
