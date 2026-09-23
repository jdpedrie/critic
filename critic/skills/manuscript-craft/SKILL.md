---
name: manuscript-craft
description: Whole-manuscript review in the craft frame. Reviewers act as consulting readers for the author's developmental editor and judge the pages against the author's stated intent, not a market. No ledger of prior issues, no commentary on pace or process. Interactive setup, then non-interactive execution. Use for a project with no publisher in the picture, or whenever the question is "is this good and is it becoming the book I described" rather than "will this sell". For changes-only review between full passes, use /critic:delta.
---

# Manuscript Review (craft frame)

The vault path is the user's configured vault. Call `read-settings` first to get `vault_path`. If it isn't set, ask the user for it before doing anything else.

This skill orchestrates the workflow. All Claude work is done by `Task` subagents (you spawn them). External models (Codex, Pi) are reached via `invoke-codex` / `invoke-pi`, and their results are collected with `invoke-status` (see *Collecting long invocations* below). Prompts are loaded via `get-prompt`.

## What the craft frame changes

Same pipeline as `/critic:manuscript-publication`, different prompts and a different contract with the reviewers:

- The principal is the author's developmental editor, not a literary agent. The bar is professional craft; the measure is the author's stated intent (from `stage.md` and the author's note). Market, comparable titles, and acquisition are out of scope.
- The prior review is context, not a checklist. Reviewers are told that an unaddressed prior issue is a sequencing decision by the author, and that they must not count cycles, cite prior issue IDs, or report what was or wasn't addressed.
- The author's pace and process are out of scope.
- When there are changes since the last review, the review opens with a section on the changes: what the author set out to do, whether it landed, whether the new material is good on its own terms, and what it cost.

Prompt files: `craft-framing.md`, `manuscript-craft.md`, `verdict-craft.md`, `rejection-pass-craft.md`, `adversarial-craft.md`, `synthesis-craft.md`. `cross-review.md` is shared with the publication frame.

## $ARGUMENTS: Author's note

If $ARGUMENTS is non-empty, treat it as an **author's note** for this review: what the author was trying to accomplish with the changes since the last review. Hold it as `author_note`; it goes into the user prompt in B5 under `=== AUTHOR'S NOTE FOR THIS REVIEW ===` and is staged into the saved review (B9).

If $ARGUMENTS is empty, no note is included. Proceed normally.

Examples:
- `/critic:manuscript-craft tightened the chapter 4 cafe scene and added the chapel scene to give Byrne some interiority`
- `/critic:manuscript-craft new chapter 13; trying to land the reunion without a speech`

The note is distinct from `stage.md`. `stage.md` describes the long-running project (what the book is trying to be, what it's for, what hasn't been attempted). The note describes the intent of this one revision. Reviewers assess whether the changes achieved the stated intent. They are not required to agree with it.

## Phase A: Interactive Setup

You (the parent Claude) act as supervisor. Ask the user four things, in order. Keep it tight. One question at a time.

### A1. Which reviewers?

- **Claude** (subagent. Runs in this session, with rejection pass)
- **Codex** (subscription-authenticated CLI)
- **Pi** (https://pi.dev. Provider/model picker next)

Default: all three. Accept brief replies ("all", "claude and pi", "skip codex").

### A2. Pi (constructive) provider/model: only if Pi was selected

Call `pi-list-models`. Present the result. Ask which provider/model to use for the constructive Pi reviewer. If they say "default" or don't care, omit `provider`/`model` from invoke-pi calls (server uses configured defaults).

### A3. Frank-reader provider/model: only if the frank-reader step is on

The frank reader (the craft frame's adversary) is a separate Pi invocation with a different framing. Ask which provider/model to use. It should usually differ from A2: a different aesthetic distribution gives more useful pushback. Use the `pi-list-models` output from A2. Accept "same as primary".

### A4. Which steps?

- Prior-review load (on). Context only; see B1.
- Rejection pass after Claude's review (on)
- Frank-reader pass (on, only if Pi enabled)
- Cross-review round, including the frank reader as a 4th matrix participant (on)
- Synthesis (on)
- Save to `Review/` (on)

Accept "all", "default", or specific opt-outs.

After A4, restate the chosen configuration in one short paragraph and run.

## Phase B: Execution (non-interactive, except on reviewer failure)

Do not stop for user input from here on, with one exception: **if any enabled
reviewer invocation fails** (Codex or Pi returns an error, a Claude subagent
errors out), STOP immediately. Do not continue with the remaining steps, do
not synthesize around the gap. Present the exact error to the user and ask
how to proceed: retry the failed invocation, continue without that reviewer,
or abort. This applies to the independent reviews (B6) and to cross-review
rebuttals (B7) alike.

Non-reviewer errors (a missing optional file, an empty diff) are not
failures. Handle those as the steps describe.

### Collecting long invocations

`invoke-codex` / `invoke-pi` / `invoke-claude` do not block until the model is
finished. A short call answers inline with `{status:"done", response,
session_id}`. A long one answers with `{status:"running", job_id}` as soon as
its `wait_seconds` budget is up. Every full-manuscript review takes that second
path. It is not an error and not a timeout: the work is still running on the
server.

Collect it by calling `invoke-status(job_id: <id>, wait_seconds: 60)` until
`status` is no longer `running`. A finished job returns the same `{response,
session_id}` an inline call would have.

**Start every reviewer before collecting any of them.** Fire all the invoke-*
calls in one turn, keep the `job_id`s, then poll.

Results are also written to disk at the `output_file` path in each result. If a
run dies part-way, finished reviews survive there, and `invoke-status` with no
`job_id` lists every job the server still holds.

### B1. Load prior review (if step enabled)

Find the most recent craft-lineage review: the highest-numbered file matching `Review/NNN-manuscript-craft-*.md` or `Review/NNN-delta-craft-*.md`. Use the parent's built-in `Glob` and `Read` tools. Cut at the HTML comment containing `RAW AGENT OUTPUTS BELOW`; keep only the synthesis above it. If there is none, there is no prior review: publication-frame reviews (`manuscript-critic`, `delta-publication`) are a different lineage and are never loaded here.

Also read `issues.md` from the vault root (if present) for deferred issues.

### B2. Determine review number

Call `next-review-number`.

### B3. Snapshot the manuscript and diff against the prior snapshot

Call `snapshot-and-diff(vault: <vault>, lineage: "craft", kind: "manuscript-craft", review: <N from B2>)`. It assembles the manuscript from `Story/`, writes it to `Review/.snapshots/craft-<timestamp>.md` with a `.json` sidecar recording this review, diffs against the prior snapshot in the craft lineage (written by `/critic:manuscript-craft` or `/critic:delta` under the craft frame; publication snapshots are never used), and saves the diff as a paired `.diff`. Returns JSON `{snapshot_path, prior_path, diff_path, diff_text, changed_scenes, changed_text}`.

If `diff_text` is empty (first run in the craft lineage, or no substantive changes): hold `changes_block` as `""` and `diff_full_path` as `""`. Skip to B4. A first craft run after publication-frame reviews is expected to land here: it establishes the craft baseline.

Otherwise build `changes_block`:

1. One line per entry in `changed_scenes`, in order: `- Chapter 4: Arcadia / Departure for Arcadia: modified, 812 to 1,020 words`. Use `added`, `modified`, `removed` as the tool reports them. An entry with an empty scene title is the chapter's preamble or its single untitled scene; label it `(chapter text)`.
2. Below the list, a short summary you write from `diff_text`: a sentence or two per chapter that changed, plus one line on structural shifts (new chapters, reorders, large rewrites). Describe what changed. Do not evaluate it; that's the reviewers' job.

Hold `diff_path` as `diff_full_path`. Ignore `changed_text` here (it's for `/critic:delta`).

If the diff is whitespace-only or trivially small, `changes_block` can be the single line "No substantive changes since the last review."

### B4. Compose the manuscript-review system prompt

Call `get-prompt` three times and concatenate:
- `get-prompt(name: "craft-framing.md", vault: <vault>)`
- `get-prompt(name: "manuscript-craft.md", vault: <vault>)`
- `get-prompt(name: "verdict-craft.md", vault: <vault>)`

Hold as `manuscript_system_prompt`.

### B5. Build user prompt prefix

Construct in this order, only the sections that apply. The stage block goes
first because it calibrates everything else. The changes block and the
author's note sit together because they're paired.

Deliberately excluded: Research worldbuilding and Codex entries. Manuscript
reviewers judge what's on the page as a reader would. Canon work belongs to
`/critic:extract` and `/critic:close-read`.

- **Current draft stage**: `read-stage(vault: <vault>)`, prefixed `=== CURRENT DRAFT STAGE ===`. If the author has a `stage.md` it's used verbatim; otherwise the server derives one. Include this first.
- **Style guide**: `read-style-guide(vault: <vault>)`, prefixed `=== STYLE GUIDE ===`. Skip if empty.
- **Known issues**: `read-issues(vault: <vault>)`, prefixed `=== KNOWN ISSUES ===`. Skip if empty. Follow it with: "These are issues the author has already acknowledged and deferred. Context only. Do not re-raise them."
- **Prior review** (if loaded in B1): prefixed `=== PRIOR REVIEW (CONTEXT ONLY) ===`. Follow it with: "The author has read this. It is here so you don't spend words rediscovering it. It is not a checklist. Do not report which of these issues remain open, do not count cycles, do not cite these IDs. Anything here that still matters, state fresh and briefly."
- **Changes since last review** (if `changes_block` is non-empty): prefixed `=== CHANGES SINCE LAST REVIEW ===`. Follow it with: "This lists what the author changed since the previous review. Use it to know what's new, and to write your `## The changes` section: what the author set out to do, whether it landed, whether the new material is good on its own terms, and what it cost. It is not a list of prior issues to audit."
- **Author's note** (if $ARGUMENTS was non-empty): prefixed `=== AUTHOR'S NOTE FOR THIS REVIEW ===`, the note verbatim, then: "This is what the author says they were trying to accomplish with this revision. Assess whether the changes achieved it. You are not required to agree. If the intent was achieved but introduced new problems, say so."

Do NOT include the manuscript in this string. The `invoke-*` tools take `include_manuscript_from: <vault>` to append it server-side. For Claude subagents, call `assemble-manuscript(vault: <vault>)` and inline it.

### B6. Independent reviews (parallel)

Spawn all enabled reviewers in a single turn. External ones return job handles; collect them only after all have been started.

**Claude (subagent. Review + rejection pass in one shot, if rejection is enabled):**

`Task` with `subagent_type: "general-purpose"`. One subagent does both so the rejection pass has true context continuity. Prompt structure:
1. `manuscript_system_prompt`
2. "Do the manuscript review per the system prompt. Output it in full. Then, on a new line, output exactly `# REJECTION PASS` as a separator. Then do a rejection pass on your own review using the instructions below. Be blunt. You have the reasoning behind your review fresh in mind, use it."
3. `get-prompt("rejection-pass-craft.md", vault)`
4. The user-prompt prefix (B5)
5. The manuscript text from `assemble-manuscript`

If the rejection pass is disabled, omit 2 and 3. Split the result on `# REJECTION PASS` into `claude_review` and `claude_rejection`.

**Codex:**

```
invoke-codex(system_prompt: manuscript_system_prompt, user_prompt: <B5 prefix>, include_manuscript_from: <vault>)
```

Collect; store `codex_review`, `codex_session_id`.

**Pi (constructive):**

```
invoke-pi(system_prompt: manuscript_system_prompt, user_prompt: <B5 prefix>, include_manuscript_from: <vault>, provider: <A2>, model: <A2>)
```

Collect; store `pi_review`, `pi_session_id`.

**Pi (frank reader). Only if the step is on:**

```
invoke-pi(system_prompt: get-prompt("adversarial-craft.md"), user_prompt: <B5 prefix>, include_manuscript_from: <vault>, provider: <A3>, model: <A3>)
```

Fresh session, own session_id. Collect; store `adv_review`, `adv_session_id`.

A `{status:"running"}` result is not an error. Keep polling. If any reviewer errors, STOP per the Phase B exception.

### B7. Cross-review (if step enabled)

Full matrix. Each participating reviewer rebuts the others. Start all, then collect (see *Collecting long invocations*).

For each reviewer, the user prompt is the concatenation of all OTHER reviewers' reviews, labeled by source (`## Claude's Review`, `## Codex's Review`, `## Pi's Review`, `## Pi (Frank Reader)'s Review`), separated by `---`. The system prompt for every rebuttal is `get-prompt("cross-review.md", vault, vars: {MaxNewIssues: 3})`.

- Claude rebuts: `Task` subagent with Claude's own review verbatim, the other reviews, and the cross-review prompt. Capture `claude_rebuttal`.
- Codex rebuts: `invoke-codex(system_prompt: <cross-review>, user_prompt: <others>, session_id: <codex_session_id>)`.
- Pi rebuts: `invoke-pi(..., session_id: <pi_session_id>)`.
- Frank reader rebuts (if it ran): `invoke-pi(..., session_id: <adv_session_id>)`. It keeps its stance through session continuity.

Skip rebuttals for reviewers that were skipped in B6.

### B8. Synthesis (if step enabled, subagent)

Spawn a `Task` subagent with:
- All primary reviews, Claude's rejection pass (if any), the frank reader's review (if any), all rebuttals (if any)
- The system prompt: `get-prompt("craft-framing.md")` + `get-prompt("synthesis-craft.md", vars: {ReviewNum: "<padded>"})` + `get-prompt("verdict-craft.md")`

Pad the review number from B2 to 3 digits (3 becomes "003"). Capture the markdown as `synthesis`.

### B9. Stage parts (as you go)

Stage via `stage-review-part(vault, name, content)` as each step completes:

- `author-note` → `# Author's Note\n\n<author_note>` (if any)
- `changes` → `# Changes Since Last Review\n\n<changes_block>\n\nFull diff: \`<diff_full_path>\`` (if a diff was produced)
- `claude-review` → `# Claude Review\n\n<claude_review>`
- `claude-rejection` → `# Claude Rejection Pass\n\n<claude_rejection>` (if any)
- `codex-review` → `# Codex Review\n\n<codex_review>` (if any)
- `pi-review` → `# Pi (Constructive) Review\n\n<pi_review>` (if any)
- `adversary-review` → `# Pi (Frank Reader) Review\n\n<adv_review>` (if any)
- `claude-rebuttal`, `codex-rebuttal`, `pi-rebuttal`, `adversary-rebuttal` → `# <Source> Cross-Review Rebuttal\n\n<text>` (if any)
- `synthesis` → `<synthesis>` (raw, no heading)

### B10. Assemble (if save enabled)

```
assemble-review(
  vault: <vault>,
  prefix: "manuscript-craft",
  synthesis_part: "synthesis",
  raw_parts: "author-note,changes,claude-review,claude-rejection,codex-review,pi-review,adversary-review,claude-rebuttal,codex-rebuttal,pi-rebuttal,adversary-rebuttal",
)
```

Missing staged parts are skipped automatically.

### B11. Present

Tell the user the saved file path and review number. Present the synthesis. Then briefly call out the rejection-pass findings (the most important corrective), the frank reader's findings, and any contested points the user should weigh in on.

## Notes

- Run Phase B straight through. Do not stop between steps, except on reviewer failure.
- The source of truth is the chapter files in `<vault>/Story/`, via `assemble-manuscript`. Do not read chapter files individually or fall back to `summary/` or `Review/`.
- All reviewers see the same user-prompt prefix. The manuscript is appended server-side for invoke-* calls and inlined for Claude subagents.
- Issue IDs use the review number from B2 padded to 3 digits. `/critic:rebuttal` and `/critic:assess` work on them as usual.
- Saved as `Review/NNN-manuscript-craft-<timestamp>.md`. The publication frame saves as `manuscript-critic`. The two lineages never share snapshots or prior-review context: craft reviews diff against craft snapshots and load craft reviews, and nothing else.
