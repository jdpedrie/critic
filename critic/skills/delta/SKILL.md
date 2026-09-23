---
name: delta
description: Iterative review of the changes since the last review. Reviewers get the full manuscript for context but review only the scenes the author added or modified, against the author's note if one is given. Answers "did this round do what I meant, and is it good", not "is the whole book on track". Non-interactive; runs with the configured reviewers. Use after a burst of drafting or revision, between full /critic:manuscript-craft passes.
---

# Delta Review

Reviews the changes since the last manuscript-level review. The full manuscript is context; the changed scenes are the target. The output is a short report: the intent, whether it landed, whether the new material is good on its own terms, what it cost, issues with IDs, and a verdict on the changes.

The vault path is the user's configured vault. Call `read-settings` first. If `vault_path` isn't set, ask for it before doing anything else. Hold the settings: `codex_enabled`, `pi_enabled`, `pi_provider`, `pi_model`, `adversary_provider`, `adversary_model`, `frame`.

This skill does not prompt. It runs with the configured reviewers. Claude always runs (as a `Task` subagent). Codex runs unless `codex_enabled` is `false`. Pi runs unless `pi_enabled` is `false`; when Pi runs, so does the frank reader (a second Pi session with a different framing), using `adversary_provider` / `adversary_model` if set, else the Pi defaults.

## $ARGUMENTS: Author's note

If non-empty, it's the author's note: what they were trying to do with this round of changes. Hold as `author_note`. Strongly encouraged; without it the reviewers can only judge the changes on their merits.

Examples:
- `/critic:delta gave Luma a decision in the Geneva scene instead of another summons`
- `/critic:delta cut the jet and the car from chapter 12; trying to get to the house faster`
- `/critic:delta` (no note; merits only)

## Frame

`frame` from settings selects the prompts. Default (unset or `craft`):

| Slot | Prompt |
|------|--------|
| framing | `craft-framing.md` |
| review | `delta.md` (vars `{MaxIssues: 7}`) |
| rejection | `rejection-pass-craft.md` |
| frank reader | `adversarial-craft.md` |
| synthesis | `synthesis-delta.md` (vars `{ReviewNum: "<padded>"}`) |

If `frame` is `publication`, use `agent-framing.md`, `rejection-pass.md`, and `adversarial.md` in place of the craft files. `delta.md` and `synthesis-delta.md` are the same in both frames.

Hold `lineage` as the frame name: `craft` (default) or `publication`. It names the snapshot lineage this review diffs within and the saved-file prefix (`delta-<lineage>`). A craft delta only ever diffs against craft snapshots (from `/critic:manuscript-craft` or earlier craft deltas); a publication delta only against publication ones.

## Execution

Runs straight through, with one exception: **if any enabled reviewer invocation fails** (Codex or Pi returns an error, a Claude subagent errors out), STOP. Show the user the exact error and ask whether to retry, continue without that reviewer, or abort. Do not synthesize around a gap.

### Collecting long invocations

`invoke-codex` / `invoke-pi` return either `{status:"done", response, session_id}` or, when the model outruns the call's `wait_seconds` budget, `{status:"running", job_id}`. Running is not an error. Poll with `invoke-status(job_id: <id>, wait_seconds: 60)` until it's done. **Start every reviewer before collecting any of them.**

### 1. Review number

`next-review-number(vault)`. Pad to 3 digits for issue IDs.

### 2. Snapshot and find the changes

`snapshot-and-diff(vault: <vault>, lineage: <lineage>, kind: "delta-<lineage>", review: <N from step 1>)`. Returns `{snapshot_path, prior_path, diff_path, diff_text, changed_scenes, changed_text}`.

- If `prior_path` is empty, the lineage had no baseline. This run just wrote one. Tell the user: "No prior <lineage> snapshot, so there's nothing to diff. This run established the baseline; the next `/critic:delta` will review changes from here." Stop.
- If `diff_text` is empty, nothing changed. Say so and stop.
- If `changed_scenes` contains only `removed` entries, say what was removed and stop; there's no new material to review. (Removals alongside additions or modifications are reported in the changes list and reviewed as part of what the changes cost.)

Build `changed_scenes_block`: one line per entry, in order, `- Chapter 13: Reunion / Fireside: added, 1,400 words` or `modified, 812 to 1,020 words` or `removed, 300 words`. An entry with an empty scene title is the chapter's preamble or its single untitled scene; label it `(chapter text)`.

Hold `changed_text` verbatim as `changed_material`. Hold `diff_path`.

### 3. Context

In parallel:
- `read-stage(vault)` → `stage_block`
- `read-style-guide(vault)` → `style_block`
- `read-issues(vault)` → `issues_block`

If `author_note` cites issue IDs (`ISSUE-NNN-NN` or bare `NNN-NN`), call `read-issue(vault, issue_id)` for each and concatenate the results as `referenced_issues`. That's the only prior-review material a delta review carries: the note says what the author was responding to, and reviewers see exactly that, not the whole prior report.

### 4. System prompts

- `review_system` = framing + `get-prompt("delta.md", vault, vars: {MaxIssues: 7})`
- `rejection_prompt` = the rejection prompt for the frame
- `frank_system` = the frank-reader prompt for the frame, followed by this paragraph: "This is a review of the changes since the last review, not of the whole manuscript. Apply your structure to the material listed under `=== CHANGED SCENES ===` and reproduced under `=== CHANGED MATERIAL ===`. Read the rest of the manuscript for context only. Issues you raise must be in the changed material or caused by it."
- `synthesis_system` = framing + `get-prompt("synthesis-delta.md", vault, vars: {ReviewNum: "<padded>"})`

### 5. User prompt prefix

In this order, only sections that apply:

- `=== CURRENT DRAFT STAGE ===\n\n<stage_block>`. Always first.
- `=== STYLE GUIDE ===\n\n<style_block>`. Skip if empty.
- `=== KNOWN ISSUES ===\n\n<issues_block>` followed by "Acknowledged and deferred by the author. Context only. Do not re-raise." Skip if empty.
- `=== REFERENCED PRIOR ISSUES ===\n\n<referenced_issues>` followed by "The author's note refers to these. They are here so you can judge whether the changes answer them. Do not audit anything else from prior reviews." Skip if none.
- `=== CHANGED SCENES ===\n\n<changed_scenes_block>`
- `=== AUTHOR'S NOTE FOR THIS REVIEW ===\n\n<author_note>` followed by "This is what the author says they were trying to do. Assess the changes against it. You are not required to agree. If the intent was achieved but introduced new problems, say so." Skip if no note.
- `=== CHANGED MATERIAL ===\n\n<changed_material>`

Do not include the manuscript in this string. Pass `include_manuscript_from: <vault>` on the invoke-* calls; for the Claude subagent, call `assemble-manuscript(vault)` and inline it after the prefix under `=== MANUSCRIPT ===`.

### 6. Reviews (parallel)

Start all enabled reviewers in one turn, then collect.

**Claude (subagent, review + rejection pass):** `Task` with `subagent_type: "general-purpose"`. Prompt: `review_system`; then "Do the review per the system prompt. Output it in full. Then, on a new line, output exactly `# REJECTION PASS` as a separator, then do a rejection pass on your own review using the instructions below. Be blunt."; then `rejection_prompt`; then the user prompt prefix; then the manuscript. Split the result on `# REJECTION PASS` into `claude_review` and `claude_rejection`.

**Codex (if enabled):** `invoke-codex(system_prompt: review_system, user_prompt: <prefix>, include_manuscript_from: <vault>)`. Hold `codex_review`, `codex_session_id`.

**Pi (if enabled):** `invoke-pi(system_prompt: review_system, user_prompt: <prefix>, include_manuscript_from: <vault>, provider: <pi_provider>, model: <pi_model>)` (omit provider/model if unset). Hold `pi_review`, `pi_session_id`.

**Frank reader (if Pi enabled):** `invoke-pi(system_prompt: frank_system, user_prompt: <prefix>, include_manuscript_from: <vault>, provider: <adversary_provider or pi_provider>, model: <adversary_model or pi_model>)`. Hold `adv_review`, `adv_session_id`.

### 7. Cross-review

Full matrix among the reviewers that ran. System prompt for every rebuttal: `get-prompt("cross-review.md", vault, vars: {MaxNewIssues: 3})`. User prompt: the other reviewers' reviews, labeled (`## Claude's Review`, `## Codex's Review`, `## Pi's Review`, `## Pi (Frank Reader)'s Review`), separated by `---`.

- Claude: `Task` subagent with its own review verbatim, the others, and the cross-review prompt. Hold `claude_rebuttal`.
- Codex: `invoke-codex(..., session_id: <codex_session_id>)`.
- Pi: `invoke-pi(..., session_id: <pi_session_id>)`.
- Frank reader: `invoke-pi(..., session_id: <adv_session_id>)`.

Start all, then collect. If only Claude ran, skip cross-review.

### 8. Synthesis

`Task` subagent with `synthesis_system` and, as the user prompt, every review, the rejection pass, and every rebuttal, each labeled by source. Hold `synthesis`.

### 9. Stage and save

Stage each part as it completes via `stage-review-part(vault, name, content)`:

- `author-note` → `# Author's Note\n\n<author_note>` (if any)
- `changes` → `# Changed Scenes\n\n<changed_scenes_block>\n\nFull diff: \`<diff_path>\``
- `claude-review`, `claude-rejection`, `codex-review`, `pi-review`, `adversary-review` → `# <Source> Review\n\n<text>` (the frank reader's heading is `# Pi (Frank Reader) Review`)
- `claude-rebuttal`, `codex-rebuttal`, `pi-rebuttal`, `adversary-rebuttal` → `# <Source> Cross-Review Rebuttal\n\n<text>`
- `synthesis` → `<synthesis>` (raw)

Then:

```
assemble-review(
  vault: <vault>,
  prefix: "delta-<lineage>",
  synthesis_part: "synthesis",
  raw_parts: "author-note,changes,claude-review,claude-rejection,codex-review,pi-review,adversary-review,claude-rebuttal,codex-rebuttal,pi-rebuttal,adversary-rebuttal",
)
```

Missing parts are skipped automatically.

### 10. Present

Give the saved path and review number. Present the synthesis. Call out the rejection-pass findings and any contested points.

## Notes

- "Since the last review" means since the last review in the same lineage: for a craft delta, the last `/critic:manuscript-craft` or craft delta; for a publication delta, the last `/critic:manuscript-publication` or publication delta. Chapter and scene reviews don't snapshot and don't move the baseline. `list-snapshots(vault, lineage)` shows the lineage's history with the review each snapshot was taken for.
- The changed material is reproduced in the prompt in addition to the full manuscript. That's deliberate: reviewers should be looking at exactly what changed, and a two-sentence summary isn't enough to review prose.
- Issue IDs are `ISSUE-NNN-NN` like every other review, so `/critic:rebuttal` and `/critic:assess` work on them.
- For a whole-book read after several deltas, run `/critic:manuscript-craft` (or `/critic:manuscript-publication` under that frame).
