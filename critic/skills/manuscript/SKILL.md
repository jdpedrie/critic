---
name: manuscript
description: Multi-reviewer manuscript review. Interactive setup (which reviewers, which Pi model, which steps), then non-interactive execution. Use when the user wants honest, publishing-consultant-grade feedback on the full manuscript.
---

# Manuscript Review

The vault path is the user's configured vault. Call `read-settings` first to get `vault_path`. If it isn't set, ask the user for it before doing anything else.

This skill orchestrates the workflow. All Claude work is done by `Task` subagents (you spawn them). External models (Codex, Pi) are reached via `invoke-codex` / `invoke-pi`. Prompts are loaded via `get-prompt`.

## $ARGUMENTS: Author's note

If $ARGUMENTS is non-empty, treat it as an **author's note** for this review: a short statement of what the author was trying to accomplish with the changes since the last review. Hold it as `author_note` and inject it into the user prompt in B5 under `=== AUTHOR'S NOTE FOR THIS REVIEW ===`. It also gets staged as part of the saved review file (B9).

If $ARGUMENTS is empty, no note is included. Proceed normally.

Examples:
- `/critic:manuscript tightened the chapter 4 cafe scene and added the chapel scene to give Byrne some interiority`
- `/critic:manuscript trying to fix the agency problem from the last review. Added a moment where Henry actually chooses`

The note is distinct from `stage.md`:
- `stage.md` describes the long-running project state (act 1, target length, what hasn't been attempted).
- The author's note describes the intent of *this specific revision*.

Reviewers are told to assess whether the changes achieved the stated intent. They are NOT required to agree. They can say "the chapter 4 scene is tighter but introduces a new pacing issue at the chapel."

## Phase A: Interactive Setup

You (the parent Claude) act as supervisor. Ask the user four things, in order. Keep it tight. One question at a time.

### A1. Which reviewers?

Present this list and ask which to enable:

- **Claude** (subagent. Runs in this session, with rejection pass)
- **Codex** (subscription-authenticated CLI)
- **Pi** (https://pi.dev. Provider/model picker next)

Default: all three. Accept brief replies ("all", "claude and pi", "skip codex", etc.).

### A2. Pi (constructive) provider/model: only if Pi was selected

Call `pi-list-models`. Present the result. Ask which provider/model to use for the constructive Pi reviewer.

If they say "default" or don't care, omit `provider`/`model` from invoke-pi calls (server uses configured defaults).

### A3. Adversary provider/model: only if adversarial step is on

The adversary is a separate Pi invocation with a different framing. Ask which provider/model to use for it. It should usually differ from A2. Different aesthetic distribution, more pushback. Use the same `pi-list-models` output from A2; no need to call again.

Accept "same as primary" if the user wants it identical (we still run a separate session with the adversarial prompt).

### A4. Which steps?

Present these toggles. Default in parens.

- Prior-review summary load (on)
- Rejection pass after Claude's review (on)
- Adversarial pass (on, only if Pi enabled)
- Cross-review round, including the adversary as a 4th matrix participant (on)
- Synthesis (on)
- Save to `review/` (on)

Accept "all", "default", or specific opt-outs ("skip cross-review", "no adversarial").

After A4, restate the chosen configuration in one short paragraph and run.

## Phase B: Execution (non-interactive)

Do not stop for user input from here on. If anything errors, report it and continue with what's possible.

### B1. Load prior review summary (if step enabled)

Find the most recent `review/NNN-manuscript-critic-*.md` file. Use the parent's built-in `Glob` and `Read` tools. Cut at the sentinel `<!-- RAW AGENT OUTPUTS BELOW. NOT INCLUDED IN FUTURE REVIEW CONTEXT -->`; keep only the synthesis portion above it.

Also: read `issues.md` from the vault root (if present) for known/deferred issues.

### B2. Determine review number

Call `next-review-number` (no args except vault).

### B3. Snapshot the manuscript and diff against the prior snapshot

Call `snapshot-and-diff(vault: <vault>, prefix: "manuscript")`. The tool atomically:
- Assembles the full manuscript from `Scenes/` (sorted act → chapter → sequence, in the same Markdown format the storyline plugin's `Export project` command produces) and writes it to `review/.snapshots/manuscript-<timestamp>.md`
- Finds the prior `manuscript-*.md` snapshot
- If a prior exists and the content differs, computes a unified diff and saves it as a paired `manuscript-<timestamp>.diff` file alongside the snapshot
- Returns JSON `{snapshot_path, prior_path, diff_path, diff_text}` (all vault-relative; empty strings where not applicable).

If `diff_text` is empty (first manuscript run ever, or no substantive changes): skip to B4. Hold `diff_summary` as `""` and `diff_full_path` as `""`.

If `diff_text` is non-empty:
1. Read `diff_text` yourself. Write a concise summary. A couple sentences per chapter that actually changed, plus a one-line note on structural shifts (new chapters, reorders, large rewrites). The summary is what reviewers see; the full diff file is already on disk at `diff_path` for the user to inspect.
2. Hold the summary as `diff_summary`. Hold `diff_path` as `diff_full_path`.

If the diff is whitespace-only or trivially small, the summary can be a single line: "No substantive changes since the last review."

### B4. Compose the manuscript-review system prompt

Call `get-prompt` three times and concatenate:
- `get-prompt(name: "agent-framing.md", vault: <vault>)`
- `get-prompt(name: "manuscript.md", vault: <vault>)`
- `get-prompt(name: "verdict.md", vault: <vault>)`

Hold this as `manuscript_system_prompt`.

### B5. Build user prompt prefix

Construct (in this order, only the sections that apply). The stage block goes
first because it calibrates everything else; the worldbuilding context follows
so reviewers have it in hand before they see prior-review or diff context; the
author's note (if present) sits next to the changes-since-last-review block
since the two are paired.

- **Current draft stage**: call `read-stage(vault: <vault>)`, prefix with `=== CURRENT DRAFT STAGE ===`. If the author has written `<vault>/stage.md` it's used verbatim; otherwise the server synthesizes a stage description from the storyline project frontmatter (acts/chapters/labels) and scene metadata. This tells reviewers what fraction of the book they're seeing. CRITICAL: include this block first. Reviewers must calibrate their entire assessment against it.
- **Style guide**: call `read-style-guide(vault: <vault>)`, prefix with `=== STYLE GUIDE ===`. Skip if empty. The tool checks `<vault>/style.md` first, then falls back to `<vault>/Research/style.md`.
- **Worldbuilding (Research)**: call `read-research(vault: <vault>)`, prefix with `=== WORLDBUILDING (RESEARCH) ===`. Concatenated contents of `<vault>/Research/`. Skip if empty.
- **Codex (Characters & Locations)**: call `read-codex(vault: <vault>)` (no `names` filter. Manuscript-level review wants every entity), prefix with `=== CODEX (CHARACTERS & LOCATIONS) ===`. Per-entity reference files. Skip if empty.
- **Known issues**: call `read-issues(vault: <vault>)`, prefix with `=== KNOWN ISSUES ===`. Skip if empty.
- **Prior review summary** (if loaded in B1): prefix with `=== PRIOR REVIEW SUMMARY ===`
- **Diff summary** (if generated in B3): prefix with `=== CHANGES SINCE LAST REVIEW ===` and tell the reader: "This summarizes what the author actually changed since the previous review. Use it to assess whether prior issues were addressed and what the changes introduced. The full diff is on disk at `<diff_full_path>` if you need it (but reviewers don't have a way to fetch it; only the orchestrator and the user can read it)."
- **Author's note** (if $ARGUMENTS was non-empty): prefix with `=== AUTHOR'S NOTE FOR THIS REVIEW ===` followed by the verbatim note, then tell the reader: "This is what the author says they were trying to accomplish with this revision. Assess whether the changes achieved that intent. You are not required to agree. If the intent was achieved but introduced new problems, say so."

Do NOT include the manuscript in this string. The `invoke-*` tools take `include_manuscript_from: <vault>` to append the assembled manuscript server-side.

For Claude subagents, fetch the manuscript yourself via `assemble-manuscript(vault: <vault>)` and inline it (no `include_manuscript_from` available there).

### B6. Independent reviews (parallel)

Spawn all enabled reviewers in a single turn (parallel tool calls).

**Claude (subagent. Review + rejection pass in one shot, if rejection is enabled):**

Use the `Task` tool with `subagent_type: "general-purpose"`. The subagent does both the review and the rejection pass in a single run so it has true context continuity between them.

Subagent prompt structure:
1. The full system prompt (from B4)
2. Instructions: "Do the manuscript review per the system prompt. Output it in full. Then, on a new line, output exactly `# REJECTION PASS` as a separator. Then, immediately do a rejection pass on your own review using the rejection-pass instructions below. Be blunt. You have the reasoning behind your review fresh in mind, use it."
3. The rejection-pass instructions: `get-prompt("rejection-pass.md", vault)`
4. The user-prompt prefix (from B5)
5. The manuscript text. Call `assemble-manuscript(vault: <vault>)` and inline the response

If the rejection-pass step is disabled, omit the rejection sections. The subagent does only the review.

When the subagent returns, split on `# REJECTION PASS`:
- Everything before → `claude_review`
- Everything after → `claude_rejection` (if rejection enabled)

**Codex:**

```
invoke-codex(
  system_prompt: manuscript_system_prompt,
  user_prompt: <user-prompt prefix from B4>,
  include_manuscript_from: <vault>,
)
```

Returns `{response, session_id}`. Store both as `codex_review` and `codex_session_id`.

**Pi (constructive):**

```
invoke-pi(
  system_prompt: manuscript_system_prompt,
  user_prompt: <user-prompt prefix from B4>,
  include_manuscript_from: <vault>,
  provider: <chosen in A2, or omit>,
  model: <chosen in A2, or omit>,
)
```

Returns `{response, session_id}`. Store both as `pi_review` and `pi_session_id`.

**Pi (adversarial). Only if adversarial step is on:**

Run in the same parallel turn as the others. Not as a separate later phase. Adversarial participates in cross-review as a 4th matrix member.

```
invoke-pi(
  system_prompt: get-prompt("adversarial.md"),
  user_prompt: <user-prompt prefix from B4>,
  include_manuscript_from: <vault>,
  provider: <chosen in A3, or omit>,
  model: <chosen in A3, or omit>,
)
```

This is a fresh Pi session (different system prompt) with its own session_id. Store as `adv_review` and `adv_session_id`.

If a reviewer errors, note it and continue. Require at least one reviewer to succeed.

### B7. Cross-review (if step enabled)

Full matrix. Each participating reviewer rebuts the others. Spawn in parallel.

For each reviewer, the user prompt for cross-review is the concatenation of all OTHER reviewers' reviews, labeled by source (`## Claude's Review`, `## Codex's Review`, `## Pi's Review`, `## Pi (Adversarial)'s Review`), separated by `---`.

**Claude rebuts (subagent):**

`Task` subagent. Prompt includes:
- Claude's own review (verbatim. The parent passes it back into the subagent for context continuity)
- The other reviews labeled by source (everyone except Claude)
- The prompt from `get-prompt("cross-review.md", vault, vars: {MaxNewIssues: 3})`

Capture as `claude_rebuttal`.

**Codex rebuts:**

```
invoke-codex(
  system_prompt: get-prompt("cross-review.md", vault, vars: {MaxNewIssues: 3}),
  user_prompt: <other reviews concatenated>,
  session_id: <codex_session_id>,
)
```

**Pi (constructive) rebuts:**

```
invoke-pi(
  system_prompt: get-prompt("cross-review.md", vault, vars: {MaxNewIssues: 3}),
  user_prompt: <other reviews concatenated>,
  session_id: <pi_session_id>,
)
```

**Pi (adversarial) rebuts. If adversarial ran:**

```
invoke-pi(
  system_prompt: get-prompt("cross-review.md", vault, vars: {MaxNewIssues: 3}),
  user_prompt: <other reviews concatenated>,
  session_id: <adv_session_id>,
)
```

Note: the cross-review system prompt is the standard constructive-leaning one for all reviewers. The adversary maintains its harsher stance through session continuity. Its prior adversarial review is in its session context.

If a reviewer was skipped in B5, skip its rebuttal here too.

### B8. Synthesis (if step enabled, subagent)

Spawn a `Task` subagent. Give it:
- All primary reviews (Claude / Codex / Pi)
- Claude's rejection pass (if any)
- The adversary's review (if any)
- All cross-review rebuttals (if any). Including the adversary's rebuttal
- The system prompt: concatenate `get-prompt("agent-framing.md")` + `get-prompt("synthesis.md", vars: {ReviewNum: "<padded>"})` + `get-prompt("verdict.md")`

The padded review number is the integer from B2 zero-padded to 3 digits (e.g. 3 → "003"). Compute it yourself.

The subagent returns the synthesis as markdown with `ISSUE-NNN-NN` IDs.

Capture as `synthesis`.

### B9. Stage parts (as you go)

Stage every artifact via `stage-review-part(vault, name, content)`:

- `author-note` → `# Author's Note\n\n<author_note>` (only if $ARGUMENTS was non-empty)
- `diff-summary` → `# Changes Since Last Review\n\n<diff_summary>\n\nFull diff: \`<diff_full_path>\`` (only if a diff was produced in B3)
- `claude-review` → `# Claude Review\n\n<claude_review>`
- `claude-rejection` → `# Claude Rejection Pass\n\n<claude_rejection>` (if any)
- `codex-review` → `# Codex Review\n\n<codex_review>` (if any)
- `pi-review` → `# Pi (Constructive) Review\n\n<pi_review>` (if any)
- `adversary-review` → `# Pi (Adversary) Review\n\n<adv_review>` (if any)
- `claude-rebuttal` → `# Claude Cross-Review Rebuttal\n\n<claude_rebuttal>` (if any)
- `codex-rebuttal` → `# Codex Cross-Review Rebuttal\n\n<codex_rebuttal>` (if any)
- `pi-rebuttal` → `# Pi (Constructive) Cross-Review Rebuttal\n\n<pi_rebuttal>` (if any)
- `adversary-rebuttal` → `# Pi (Adversary) Cross-Review Rebuttal\n\n<adv_rebuttal>` (if any)
- `synthesis` → `<synthesis>` (raw, no heading. The assembler places it above the sentinel)

Stage parts as each step completes. Don't batch at the end.

### B10. Assemble (if save enabled)

```
assemble-review(
  vault: <vault>,
  prefix: "manuscript-critic",
  synthesis_part: "synthesis",
  raw_parts: "author-note,diff-summary,claude-review,claude-rejection,codex-review,pi-review,adversary-review,claude-rebuttal,codex-rebuttal,pi-rebuttal,adversary-rebuttal",
)
```

Missing staged parts are skipped automatically.

### B11. Present

Tell the user the saved file path and review number. Then present the synthesis in conversation. After the synthesis, briefly call out:
- The rejection pass findings (if any). Most important corrective
- The adversarial pass findings (if any)
- Any contested points the user should weigh in on

## Notes

- Run Phase B straight through. Do not stop between steps.
- The source of truth is the storyline project at `<vault>`. Get the manuscript via `assemble-manuscript`; do not read `Scenes/` files individually or fall back to `summary/` (out of date) or `review/` (those are reviews, not source).
- For step toggles, default to "on" if the user said "all" or didn't specify.
- All reviewers see the same user-prompt prefix (stage + style + research + codex + known issues + prior review summary + diff + author note). The manuscript text is appended by the MCP server for invoke-* calls (via `include_manuscript_from`); you inline it explicitly for Claude subagents via `assemble-manuscript`.
- Issue IDs use the review number from B2 padded to 3 digits.
- The cross-review matrix includes the adversary as a peer. Each reviewer (Claude / Codex / Pi-constructive / Pi-adversary) rebuts every OTHER reviewer's review. The adversary keeps its harsh stance via session continuity (its prior adversarial review is in its session context).
- You. The parent Claude. Are the supervisor. Phase A is your only chance to ask the user questions; everything else runs to completion.
