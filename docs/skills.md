# Skills

Every `/critic:*` command in detail. Skills live in `critic/skills/<name>/SKILL.md`. Claude reads them when you invoke the command.

## Frames

Two sets of prompts decide what reviewers measure against. The publication frame advises a literary agent and judges the foundations against a saleable finished book; prior issues are tracked across reviews. The craft frame advises the author's developmental editor and judges the pages against the author's stated intent; market, pace, and the ledger of prior issues are out of scope. Both keep the reviewer at arm's length from the author.

The two manuscript skills are named by frame. `/critic:delta`, `/critic:review`, `/critic:downstream`, and `/critic:consult` take the frame from the `frame` setting (default `publication`).

---

## /critic:delta

Iterative review of the changes since the last manuscript-level review. The full manuscript is context; the scenes the author added or modified are the target. Non-interactive.

### Invocation

```
/critic:delta
/critic:delta gave Luma a decision in the Geneva scene instead of another summons
/critic:delta trying to answer ISSUE-008-03; the railgun report is now on the table between them
```

$ARGUMENTS is the author's note: what this round of changes was trying to do. Reviewers assess the changes against it. If the note cites issue IDs, those issue blocks (and only those) are pulled from the prior reviews and shown to reviewers. Without a note, reviewers judge the changes on their merits.

### Workflow

1. `next-review-number`.
2. `snapshot-and-diff(lineage: <frame>, kind: "delta-<frame>", review: N)`. The lineage is the frame (`craft` by default), so a craft delta diffs only against craft snapshots. If the lineage has no prior snapshot, this run is its baseline: say so and stop. If nothing changed, say so and stop. Otherwise hold `changed_scenes` (one line per added, modified, or removed scene with word counts) and `changed_text` (the full prose of the added and modified scenes).
3. Context: `read-stage`, `read-style-guide`, `read-issues`, plus `read-issue` for any IDs the note cites.
4. Prompts by frame. Craft: `craft-framing.md` + `delta.md`; rejection `rejection-pass-craft.md`; frank reader `adversarial-craft.md` with a bridging paragraph scoping it to the changes; synthesis `craft-framing.md` + `synthesis-delta.md`. Publication swaps in `agent-framing.md`, `rejection-pass.md`, `adversarial.md`.
5. User prompt: stage, style, known issues, referenced prior issues, changed scenes, author's note, changed material. The manuscript is appended server-side (`include_manuscript_from`) or inlined for the Claude subagent.
6. Reviews in parallel: Claude subagent (review plus rejection pass), Codex if enabled, Pi if enabled, frank reader if Pi is enabled (using `adversary_provider` / `adversary_model` when set).
7. Cross-review, full matrix.
8. Synthesis with `synthesis-delta.md`: the intent, did it land, on its own merits, what it cost, issues with IDs, strengths, open questions, and a verdict on the changes (landed / partly / didn't; new material above / at / below the manuscript's best; fix first; protect).
9. Stage and assemble with prefix `delta-<frame>`.

### Output

```
Review/NNN-delta-craft-YYYY-MM-DD-HHMMSS.md
Review/NNN-delta-publication-YYYY-MM-DD-HHMMSS.md
```

### When to use

After a burst of drafting or revision. Between full manuscript passes. Whenever the question is "did this round do what I meant, and is it good".

---

## /critic:manuscript-craft

Whole-manuscript review in the craft frame. Interactive setup, then straight through.

### Invocation

```
/critic:manuscript-craft
/critic:manuscript-craft new chapters 13 and 14; the act turn is on the page now
```

$ARGUMENTS is the author's note for this review, as with `/critic:delta`.

### What differs from the publication frame

Prompts: `craft-framing.md`, `manuscript-craft.md`, `verdict-craft.md`, `rejection-pass-craft.md`, `adversarial-craft.md`, `synthesis-craft.md`. `cross-review.md` is shared.

The prior review is loaded as context, not as a checklist: reviewers are told an unaddressed issue is the author's sequencing decision, and that they must not count cycles, cite prior IDs, or report what was or wasn't addressed. If an old problem still matters, they state it fresh.

When there are changes since the last review, the review and the synthesis open with a section on them (intent, landed, merits, cost) before the whole-book sections. The verdict adds two lines for the changes.

The prior review it loads is the latest `manuscript-craft` or `delta-craft` review. It snapshots into the `craft` lineage and diffs only against craft snapshots. Publication-frame reviews and snapshots are never used, so the first craft review after a run of publication reviews has no prior and establishes the craft baseline.

### Phases

Phase A asks four questions: which reviewers, the Pi model, the frank reader's model, which steps. Phase B is the same pipeline as the publication skill: prior review, review number, snapshot and diff, system prompt, user prompt, parallel reviews with a rejection pass, cross-review matrix, synthesis, stage, assemble, present. Reviewer failure stops the run.

### Output

```
Review/NNN-manuscript-craft-YYYY-MM-DD-HHMMSS.md
```

### When to use

Once an act, or whenever a whole-book read is wanted after several deltas.

---

## /critic:manuscript-publication

Whole-manuscript review in the publication frame. The original manuscript skill, renamed. Three reviewers in parallel, optional adversary, cross-review matrix, synthesis, save.

### Invocation

```
/critic:manuscript-publication
/critic:manuscript-publication tightened the chapter 4 scene; added the chapel scene to give Byrne some interiority
```

If $ARGUMENTS is non-empty, it's treated as an author's note for this review: what the author was trying to do with the changes since the last review. The note is injected near the diff summary and reviewers are told to assess whether the intent was achieved.

### Phase A: interactive setup

The skill asks four questions in order.

First, which reviewers? Claude (subagent), Codex, Pi. Default: all three.

Second, Pi (constructive) provider/model, only if Pi is on. The skill runs `pi-list-models` and shows the list.

Third, adversary provider/model, only if the adversarial step is on. The adversary is a separate Pi invocation with a different system prompt. It should usually differ from the primary Pi to get a different aesthetic distribution.

Fourth, which steps? Prior-review summary load, rejection pass after Claude, adversarial pass, cross-review, synthesis, save. Default: all on.

### Phase B: execution (non-interactive, except on reviewer failure)

The skill runs straight through, with one exception: if any enabled reviewer invocation fails (Codex or Pi errors, a Claude subagent errors out), the run stops immediately and asks the author how to proceed (retry, continue without that reviewer, or abort). No synthesizing around a gap.

The steps:

B1. Load the prior review's synthesis (above the sentinel), if step enabled. Publication lineage only: the latest `manuscript-critic` or `delta-publication` review.

B2. Get the next review number via `next-review-number`.

B3. `snapshot-and-diff(lineage: "publication", kind: "manuscript-critic", review: N)` writes a fresh manuscript snapshot (assembled in plugin export format) and computes a diff against the prior publication snapshot; `manuscript-*` snapshots from before lineages existed count as publication. If a diff exists, the orchestrator summarises it into a few sentences per affected chapter, using the tool's `changed_scenes` list to keep the summary accurate.

B4. Compose the manuscript-review system prompt: `agent-framing.md` + `manuscript.md` + `verdict.md`.

B5. Build the user prompt prefix: stage, style, known-issues, prior-review-summary, diff-summary, author-note. Each section is gated on existence. Research and Codex are deliberately excluded: manuscript reviewers read as readers, and inlining the worldbuilding bible invites them to fill gaps with insider knowledge instead of flagging them. Canon work belongs to `/critic:extract` and `/critic:close-read`.

B6. Independent reviews in parallel. Claude subagent does review plus rejection pass in one shot (true context continuity). Codex and Pi are invoked with `include_manuscript_from` so the server appends the manuscript text. Pi adversary runs in the same parallel turn with the adversarial system prompt.

B7. Cross-review matrix. Each reviewer sees the others' reviews and produces a rebuttal. Codex and Pi sessions resume; the Claude subagent gets its own prior review inlined for continuity.

B8. Synthesis subagent produces the ranked-issue report with `ISSUE-NNN-NN` IDs.

B9. Stage every artifact (`stage-review-part`).

B10. Assemble the final review file (`assemble-review`).

B11. Present the synthesis, the saved file path, and call out the rejection pass and adversary findings.

### Output

```
Review/NNN-manuscript-critic-YYYY-MM-DD-HHMMSS.md
```

The `manuscript-critic` prefix is kept so older publication-frame reviews sort with new ones.

### When to use

When a publisher or agent is in the picture and the question is whether the foundations will carry a saleable book.

---

## /critic:review

Four-role review (analytical, immersive, structural, adversarial) scoped to one chapter or one scene. Same shape as the manuscript reviews, narrower scope, no interactive setup. The framing, synthesis, and verdict prompts come from the `frame` setting.

### Invocation

```
/critic:review chapter 3
/critic:review scene 01-01 Customs at Fontenoy
/critic:review 5                      # bare integer → chapter 5
/critic:review scene 02-03 Hiring Sam Perry.md
```

### Workflow

1. Fetch the slice. `assemble-chapter` for chapter mode, `read-scene` for scene mode. Both return text plus the entity union from frontmatter.
2. Gather context in parallel: `read-stage`, `read-style-guide`, `read-research`, `read-codex(names: <slice entities>)`, `read-issues`.
3. Compose four system prompts, one per role. Each is the frame's framing prompt + `review-base` (templated with `Role` and `MaxIssues`) + `review-<role>` + the frame's verdict prompt.
4. Build user prompt with stage, style, research, codex, known issues, target slice.
5. Run four reviewers in parallel. Analytical to Claude subagent. Immersive to Codex. Structural to Claude subagent. Adversarial to Pi.
6. Pairwise cross-review. Analytical against immersive (text-only pair). Structural against adversarial (full-context pair).
7. Synthesis subagent produces the ranked-issue report.
8. Stage and save with prefix `chapter-<N>-review` or `scene-<slug>-review`.
9. Present the synthesis and the saved path.

### Output

```
Review/NNN-chapter-<N>-review-YYYY-MM-DD-HHMMSS.md
Review/NNN-scene-<CC-SS>-<slug>-review-YYYY-MM-DD-HHMMSS.md
```

### When to use

After drafting a chapter or scene, before moving on. When the manuscript review flagged an issue scoped to one chapter and you want a closer look.

---

## /critic:close-read

Line-editor and copy-editor pass. Subagent per scene. Quote-and-fix output covering typos, prose-level issues, micro-structure, canon adherence, style. This is the opposite role from the manuscript reviews, `/critic:delta`, and `/critic:review`. The manuscript reviewers are told not to flag typos. Close-read is the typo and prose-level pass.

### Invocation

```
/critic:close-read scene 01-01 Customs at Fontenoy
/critic:close-read chapter 3
/critic:close-read all
/critic:close-read 5                  # bare integer → chapter 5
```

### Workflow

1. Enumerate scenes based on mode.
2. Set up the run. Generate a timestamp run ID, create `<vault>/Review/close-read/<run-id>/`, load `style.md` and the close-read system prompt once.
3. Spawn subagents. `scene` mode spawns one. `chapter` mode spawns all chapter scenes in parallel. `all` mode runs waves of 8 in parallel, writing each wave's files before starting the next. Each subagent gets: scene text, style guide, Codex entries for that scene's frontmatter entities, and the `read-codex-entry` tool for ad-hoc lookups.
4. Write per-scene files to `<vault>/Review/close-read/<run-id>/<CC-SS>-<scene-slug>.md`.
5. Write `index.md` linking each report with a one-line summary.
6. Present the aggregate counts and any canon contradictions. Those need editor attention, not just author preference.

### Output

```
Review/close-read/<run-id>/
  index.md
  01-01-01-Epigraph.md
  01-01-02-Customs-at-Fontenoy.md
  ...
```

### Voice constraints

The prompt explicitly forbids the subagent from smoothing idiosyncratic choices into conventional ones, adding hedging the author didn't use, stripping contractions, replacing specific nouns with generic ones, and breaking punchy short sentences into balanced ones.

When in doubt about a prose fix, the subagent is told to describe the issue and stop rather than draft. Better to leave the rewrite to the author than to flatten voice.

### When to use

Before declaring a chapter done. As a polish pass after the structural critique has been addressed. Run `all` on the manuscript before submitting for external review.

---

## /critic:downstream

After editing a chapter or scene, assess what breaks in everything that comes after.

### Invocation

```
/critic:downstream chapter 3
/critic:downstream scene 02-02 Bringing Sam Aboard
```

### Workflow

1. Determine the slice and downstream. `list-scenes` enumerates. The slice is either chapter N or one scene (`CC-SS`). Downstream is the rest of the scene's chapter plus every later chapter.
2. Pull grounding context: style, research, codex.
3. Fetch the edit-point text and the downstream text: `read-scene` for later scenes in the same chapter, `assemble-chapter` for every later chapter, in batches of 8, concatenated in manuscript order.
4. Compose prompts: the frame's framing prompt + `downstream.md`. User prompt has style, research, codex, edited slice, downstream scenes.
5. Run a Task subagent.
6. Present issues grouped by affected scene, in manuscript order, critical first.

### Output

No saved file. The assessment is presented in conversation. The author follows up by spawning `/critic:review` on affected scenes if needed.

### When to use

After any significant edit that might invalidate downstream content. Especially after restructuring or removing setups.

---

## /critic:extract

Reconcile prose against the Codex (Characters plus Locations) and Research bibles. Maintains `.claude/codex-inventory.md`. Three modes. Every change requires approval.

### Invocation

```
/critic:extract chapter 3
/critic:extract scene 01-01 Customs at Fontenoy
/critic:extract entity Henry Nelson
/critic:extract entity Mark Andersen
```

### Common preamble (all modes)

The skill always reads `.claude/codex-inventory.md` (treating as empty if absent), `list-codex-entries` for the roster, `read-research` for the worldbuilding bibles, and `read-codex` (no filter) for all current Codex entries.

### Slice mode (chapter or scene)

1. Fetch the slice via `assemble-chapter` or `read-scene`.
2. Spawn a Task subagent with `extract-slice.md` as the system prompt and the codex, research, roster, inventory, slice as user prompt.
3. The subagent returns a structured report: entities in the slice (new facts, confirmed, contradictions), implicit-entity candidates (proper-noun phrases not in the roster), inventory updates, recommended Codex edits.
4. Walk through each proposal interactively. Accept, edit, or skip. Apply approved changes via `Edit` or `Write`.

### Entity mode

1. Resolve the entity name (case-insensitive substring match against the roster).
2. `find-entity-mentions(name)` returns every scene that mentions the entity, with bodies, in manuscript order.
3. `read-codex-entry(name)` for the current entry, if any.
4. Spawn a Task subagent with `extract-entity.md` as the system prompt.
5. The subagent walks every mention in manuscript order, identifies new, confirmed, and contradicted facts, drafts a consolidated Codex entry, and proposes an inventory update.
6. Walk through interactively. Contradictions must be resolved before any Codex write proceeds.

### Output

Either `<vault>/Background/Characters/<Name>.md` or `<vault>/Background/Locations/<Name>.md`, created or modified. Plus `<vault>/.claude/codex-inventory.md`, updated.

### When to use

When prose introduces new entities (characters, locations, concepts) that should join the Codex. When you want to check a chapter for canon drift. When you want a consolidated Codex entry for a character whose details are scattered across many scenes.

---

## /critic:summarize

Per-chapter reference summaries to `summary/`.

### Invocation

```
/critic:summarize
```

No arguments. Always processes every chapter.

### Workflow

1. `list-chapters` to enumerate chapters.
2. For each chapter (sequentially), `assemble-chapter` and compose a 200 to 400 word summary covering setting, characters, events, state changes, threads, tone, pacing.
3. Write to `<vault>/summary/chapter-<NN>.md`. Always overwrite.

### Output

```
summary/chapter-01.md
summary/chapter-02.md
...
```

### When to use

Author reference only. Reviews don't consume summaries. Run after significant scene additions or restructuring if you want the summary set to stay current.

---

## /critic:consult

Short focused second opinion from Codex and Pi on a narrow question. For situations where you want an outside view but don't need a full review.

### Invocation

```
/critic:consult Is the trust fund a real threat or just lampshading?
/critic:consult Compare the chapel scene to the Customs scene. Which is doing more work?
```

### Workflow

1. Compose a short system prompt: publishing consultant (publication frame) or consulting reader for the author's editor (craft frame); answer directly, ground in context, quote when relevant, no hedging.
2. The user prompt has context (the question, plus a passage if needed) inline. If the question needs the full manuscript, the skill uses `include_manuscript_from` instead of inlining.
3. `invoke-codex` and `invoke-pi` in parallel.
4. Present both responses labeled by source. Add your own take if useful.

### Output

Conversational. No saved file.

### When to use

When the orchestrating Claude wants a second opinion. When the author wants a focused outside read. When you don't want to spend the tokens on a full review.

---

## /critic:rebuttal

Author response to a flagged issue. Rebut, defer, or accept.

### Invocation

```
/critic:rebuttal ISSUE-003-01 This is intentional. The ambiguity resolves in chapter 7.
/critic:rebuttal 003-02
```

### Workflow

1. `read-issue` to pull the issue text.
2. Conversation. Author articulates their position. Skill helps refine.
3. Pick one of three outcomes. Rebut: `add-rebuttal` inserts an Obsidian callout inline after the issue in the review file. Defer: `append-issue` adds the issue to `issues.md` under a heading, plus a short rebuttal noting the deferral is added to the review file. Accept: no tool calls. Author will fix it.

### Output

The review file is modified inline (rebuttals as callouts). `issues.md` updated for deferrals.

### When to use

Whenever a review flags an issue you don't immediately fix. Either you disagree (rebut), or you agree but it's not the right time (defer), or you agree and you'll fix it (accept, no rebuttal needed).

---

## /critic:assess

Conversational deep-dive on one issue. You (the orchestrator) do the analysis directly, no subagent.

### Invocation

```
/critic:assess ISSUE-003-03
/critic:assess 003-01 Is the trust fund actually load-bearing?
```

### Workflow

1. `read-issue` to pull the issue.
2. Pull only what the issue needs. `list-scenes` to enumerate. `read-scene`, `assemble-chapter`, or `assemble-manuscript` for slices. `read-codex`, `read-codex-entry`, or `read-research` for canon. `find-entity-mentions` for character-spanning issues.
3. Investigate based on issue type. Style or pattern issues need every occurrence found. Structural issues need the pattern traced across slices. Single-scene issues need focused detail. Canon issues need both sides plus resolution paths.
4. Present findings as prose, quoting and citing scene locations.
5. Discuss with the author.
6. Offer to save to `<vault>/notes/<issue-id>.md`.

### Output

Conversational, plus an optional saved note file.

### When to use

When a review flags an issue you want to investigate in depth before deciding how to handle. When you want to know how pervasive a flagged pattern actually is.

---

## /critic:settings

View and update plugin settings.

### Invocation

```
/critic:settings                                  # view all
/critic:settings vault_path /path/to/project      # set
/critic:settings pi_provider                      # view one
```

### Settings

| Key | Description |
|-----|-------------|
| `vault_path` | Absolute path to the book folder (the one containing `Story/`). |
| `frame` | `craft` or `publication` (default). Frame for `/critic:review`, `/critic:delta`, `/critic:downstream`, `/critic:consult`. |
| `codex_enabled` | Enable the Codex reviewer (true/false). |
| `codex_model` | Codex model name (omit to let the CLI pick). |
| `openai_api_key` | OpenAI API key (omit to use Codex CLI login). |
| `pi_enabled` | Enable the Pi reviewer (true/false). |
| `pi_provider` | Default Pi provider: `anthropic`, `openai`, `google`. |
| `pi_model` | Default Pi model. Skills can override per-call. |
| `adversary_provider` | Frank reader's Pi provider for `/critic:delta`. Empty = `pi_provider`. |
| `adversary_model` | Frank reader's Pi model for `/critic:delta`. Empty = `pi_model`. |
| `claude_enabled` | Enable the `invoke-claude` tool (true/false). |
| `claude_model` | Model for `invoke-claude`. Empty = CLI default. |

Inside cowork, Claude is always available as a Task subagent regardless of these settings. The `claude_*` settings affect only the `invoke-claude` MCP tool used by non-Claude leaders.

### Output

Settings file at `${CLAUDE_PLUGIN_DATA}/settings.json`. Defaults in `critic/server/config.yaml`.

---

## /critic:help

Print the skill reference, settings, prompt-override resolution, and vault structure.
