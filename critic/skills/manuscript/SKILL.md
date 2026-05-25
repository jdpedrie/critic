---
name: manuscript
description: Multi-reviewer manuscript review. Interactive setup (which reviewers, which Pi model, which steps), then non-interactive execution. Use when the user wants honest, publishing-consultant-grade feedback on the full manuscript.
---

# Manuscript Review

The vault path is the user's configured vault. Call `read-settings` first to get `vault_path`. If it isn't set, ask the user for it before doing anything else.

This skill orchestrates the workflow. All Claude work is done by `Task` subagents (you spawn them). External models (Codex, Pi) are reached via `invoke-codex` / `invoke-pi`. Prompts are loaded via `get-prompt`.

## Phase A — Interactive Setup

You (the parent Claude) act as supervisor. Ask the user four things, in order. Keep it tight — one question at a time.

### A1. Which reviewers?

Present this list and ask which to enable:

- **Claude** (subagent — runs in this session, with rejection pass)
- **Codex** (subscription-authenticated CLI)
- **Pi** (https://pi.dev — provider/model picker next)

Default: all three. Accept brief replies ("all", "claude and pi", "skip codex", etc.).

### A2. Pi (constructive) provider/model — only if Pi was selected

Call `pi-list-models`. Present the result. Ask which provider/model to use for the constructive Pi reviewer.

If they say "default" or don't care, omit `provider`/`model` from invoke-pi calls (server uses configured defaults).

### A3. Adversary provider/model — only if adversarial step is on

The adversary is a separate Pi invocation with a different framing. Ask which provider/model to use for it. It should usually differ from A2 — different aesthetic distribution, more pushback. Use the same `pi-list-models` output from A2; no need to call again.

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

## Phase B — Execution (non-interactive)

Do not stop for user input from here on. If anything errors, report it and continue with what's possible.

### B1. Load prior review summary (if step enabled)

Find the most recent `review/NNN-manuscript-critic-*.md` file. Use the parent's built-in `Glob` and `Read` tools. Cut at the sentinel `<!-- RAW AGENT OUTPUTS BELOW — NOT INCLUDED IN FUTURE REVIEW CONTEXT -->`; keep only the synthesis portion above it.

Also: read `issues.md` from the vault root (if present) for known/deferred issues.

### B2. Determine review number

Call `next-review-number` (no args except vault).

### B3. Compose the manuscript-review system prompt

Call `get-prompt` three times and concatenate:
- `get-prompt(name: "agent-framing.md", vault: <vault>)`
- `get-prompt(name: "manuscript.md", vault: <vault>)`
- `get-prompt(name: "verdict.md", vault: <vault>)`

Hold this as `manuscript_system_prompt`.

### B4. Build user prompt prefix

Construct (in this order, only if present):
- Style guide: read `<vault>/style.md` if it exists, prefix with `=== STYLE GUIDE ===`
- Known issues: prefix with `=== KNOWN ISSUES ===`
- Prior review summary: prefix with `=== PRIOR REVIEW SUMMARY ===`

Do NOT include the manuscript in this string. The `invoke-*` tools take `include_manuscript_from: <vault>` to append it server-side.

For Claude subagents, you'll need to read the manuscript yourself and inline it (no `include_manuscript_from` available there).

### B5. Independent reviews (parallel)

Spawn all enabled reviewers in a single turn (parallel tool calls).

**Claude (subagent — review + rejection pass in one shot, if rejection is enabled):**

Use the `Task` tool with `subagent_type: "general-purpose"`. The subagent does both the review and the rejection pass in a single run so it has true context continuity between them.

Subagent prompt structure:
1. The full system prompt (from B3)
2. Instructions: "Do the manuscript review per the system prompt. Output it in full. Then, on a new line, output exactly `# REJECTION PASS` as a separator. Then, immediately do a rejection pass on your own review using the rejection-pass instructions below. Be blunt — you have the reasoning behind your review fresh in mind, use it."
3. The rejection-pass instructions: `get-prompt("rejection-pass.md", vault)`
4. The user-prompt prefix (from B4)
5. The manuscript text (read all chapter files in order from `<vault>/story/`)

If the rejection-pass step is disabled, omit the rejection sections — the subagent does only the review.

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

**Pi (adversarial) — only if adversarial step is on:**

Run in the same parallel turn as the others — not as a separate later phase. Adversarial participates in cross-review as a 4th matrix member.

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

### B6. Cross-review (if step enabled)

Full matrix — each participating reviewer rebuts the others. Spawn in parallel.

For each reviewer, the user prompt for cross-review is the concatenation of all OTHER reviewers' reviews, labeled by source (`## Claude's Review`, `## Codex's Review`, `## Pi's Review`, `## Pi (Adversarial)'s Review`), separated by `---`.

**Claude rebuts (subagent):**

`Task` subagent. Prompt includes:
- Claude's own review (verbatim — the parent passes it back into the subagent for context continuity)
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

**Pi (adversarial) rebuts — if adversarial ran:**

```
invoke-pi(
  system_prompt: get-prompt("cross-review.md", vault, vars: {MaxNewIssues: 3}),
  user_prompt: <other reviews concatenated>,
  session_id: <adv_session_id>,
)
```

Note: the cross-review system prompt is the standard constructive-leaning one for all reviewers. The adversary maintains its harsher stance through session continuity — its prior adversarial review is in its session context.

If a reviewer was skipped in B5, skip its rebuttal here too.

### B7. Synthesis (if step enabled, subagent)

Spawn a `Task` subagent. Give it:
- All primary reviews (Claude / Codex / Pi)
- Claude's rejection pass (if any)
- The adversary's review (if any)
- All cross-review rebuttals (if any) — including the adversary's rebuttal
- The system prompt: concatenate `get-prompt("agent-framing.md")` + `get-prompt("synthesis.md", vars: {ReviewNum: "<padded>"})` + `get-prompt("verdict.md")`

The padded review number is the integer from B2 zero-padded to 3 digits (e.g. 3 → "003"). Compute it yourself.

The subagent returns the synthesis as markdown with `ISSUE-NNN-NN` IDs.

Capture as `synthesis`.

### B8. Stage parts (as you go)

Stage every artifact via `stage-review-part(vault, name, content)`:

- `claude-review` → `# Claude Review\n\n<claude_review>`
- `claude-rejection` → `# Claude Rejection Pass\n\n<claude_rejection>` (if any)
- `codex-review` → `# Codex Review\n\n<codex_review>` (if any)
- `pi-review` → `# Pi (Constructive) Review\n\n<pi_review>` (if any)
- `adversary-review` → `# Pi (Adversary) Review\n\n<adv_review>` (if any)
- `claude-rebuttal` → `# Claude Cross-Review Rebuttal\n\n<claude_rebuttal>` (if any)
- `codex-rebuttal` → `# Codex Cross-Review Rebuttal\n\n<codex_rebuttal>` (if any)
- `pi-rebuttal` → `# Pi (Constructive) Cross-Review Rebuttal\n\n<pi_rebuttal>` (if any)
- `adversary-rebuttal` → `# Pi (Adversary) Cross-Review Rebuttal\n\n<adv_rebuttal>` (if any)
- `synthesis` → `<synthesis>` (raw, no heading — the assembler places it above the sentinel)

Stage parts as each step completes — don't batch at the end.

### B9. Assemble (if save enabled)

```
assemble-review(
  vault: <vault>,
  prefix: "manuscript-critic",
  synthesis_part: "synthesis",
  raw_parts: "claude-review,claude-rejection,codex-review,pi-review,adversary-review,claude-rebuttal,codex-rebuttal,pi-rebuttal,adversary-rebuttal",
)
```

Missing staged parts are skipped automatically.

### B10. Present

Tell the user the saved file path and review number. Then present the synthesis in conversation. After the synthesis, briefly call out:
- The rejection pass findings (if any) — most important corrective
- The adversarial pass findings (if any)
- Any contested points the user should weigh in on

## Notes

- Run Phase B straight through. Do not stop between steps.
- Read chapter files directly from `<vault>/story/` using `Read` and `Glob`. Do not use `summary/` (out of date) or `review/` (those are reviews, not source).
- For step toggles, default to "on" if the user said "all" or didn't specify.
- All reviewers see the same user-prompt prefix (style guide + known issues + prior review summary). The manuscript text is appended by the MCP server for invoke-* calls; you inline it for subagents.
- Issue IDs use the review number from B2 padded to 3 digits.
- The cross-review matrix includes the adversary as a peer. Each reviewer (Claude / Codex / Pi-constructive / Pi-adversary) rebuts every OTHER reviewer's review. The adversary keeps its harsh stance via session continuity (its prior adversarial review is in its session context).
- You — the parent Claude — are the supervisor. Phase A is your only chance to ask the user questions; everything else runs to completion.
