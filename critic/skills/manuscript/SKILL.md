---
name: manuscript
description: Review the full manuscript at the book level using three independent models (Claude, Codex, Gemini), cross-review with session continuity, and synthesis. Use when the user wants feedback on the whole book — arc completion, pacing, character consistency, dangling threads, tonal drift.
---

# Manuscript Review

The vault path for all tool calls is: ${user_config.vault_path}

Run a three-model manuscript review. Three independent reviews (Claude, Codex, Gemini), cross-review with session continuity, synthesis, save. Text-only — no world-building context. Runs non-interactively through all steps.

## Arguments

$ARGUMENTS is optional. If provided, it can specify focus areas (e.g., "focus on pacing" or "just arcs and dangling threads").

## Workflow

Run all steps without stopping for user input.

### Step 0 — Prior Review Summary

Call `summarize-review` with prefix "manuscript-critic" and the vault path.

If a prior review exists, hold the summary for step 1. If "No prior review found.", skip passing it.

### Step 1 — Independent Reviews

Call ALL THREE manuscript review tools in parallel:
- `review-manuscript-claude` with the vault path
- `review-manuscript-codex` with the vault path
- `review-manuscript-gemini` with the vault path
- If a prior review summary exists, pass it as `prior_review_summary` to all three

Each tool returns JSON with `review` and `session_id`. Parse these to extract both fields.

If `review-manuscript-gemini` is not available (tool not registered), run with just Claude and Codex.

### Step 2 — Cross-Review

Call `cross-review-manuscript` with:
- `claude_review`: Claude's review text
- `codex_review`: Codex's review text
- `gemini_review`: Gemini's review text (if available)
- `claude_session_id`: Claude's session ID
- `codex_session_id`: Codex's session ID
- `gemini_session_id`: Gemini's session ID (if available)

Each model resumes its session and rebuts the others' reviews.

### Step 3 — Synthesis

Call the `synthesize` tool with:
- `reviews`: JSON object mapping model names to their review text (e.g., {"claude": "...", "codex": "...", "gemini": "..."})
- `rebuttals`: JSON object mapping model names to their rebuttal text

### Step 4 — Save

Call `save-review` with:
- `vault`: the vault path
- `prefix`: "manuscript-critic"
- `content`: a single markdown file containing:
  1. The synthesis
  2. `---` then `## Claude Review` followed by Claude's review
  3. `---` then `## Codex Review` followed by Codex's review
  4. `---` then `## Gemini Review` followed by Gemini's review (if available)
  5. `---` then the full cross-review output

### Step 5 — Present

Tell the user where the file was saved, then present the synthesis in conversation.

## Important Notes

- Run all steps without asking. Do not stop between rounds.
- This reads the ENTIRE manuscript — story only, no world-building notes or plot outlines.
- The manuscript may be incomplete. The reviewers handle partial manuscripts gracefully.
- The cross-review uses session resumption — each model remembers its own review when challenging the others.
- If Gemini is unavailable, fall back to two-model review (Claude + Codex only).
- All outputs are prose. No JSON parsing needed on tool results except the initial review step (which wraps review + session_id).
