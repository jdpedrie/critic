---
name: review
description: Chapter-level multi-agent review. Spawns parallel reviewers (Claude subagents, Codex, Pi) for a single chapter with four role lenses (analytical, immersive, structural, adversarial). Use when the user asks to review a specific chapter.
---

# Chapter Review

The vault path is the user's configured vault. Call `read-settings` if you don't already know it.

## Arguments

$ARGUMENTS = chapter name (e.g. `chapter-03` or `chapter-03.md`).

## Setup

This skill does not prompt — it just runs. If the user wants to customize which reviewers run, they should use `/critic:manuscript` instead (which has the interactive setup) or edit the skill.

## Execution

### 1. Load context

- Read the chapter file directly from `<vault>/story/<chapter>.md`
- Read prior chapter summaries from `<vault>/summary/` (up to 2 chapters back). If no summaries exist, fall back to the prior raw chapters from `<vault>/story/`.
- Read `<vault>/style.md` if it exists
- Read `<vault>/issues.md` if it exists
- For full-context roles only: gather `<vault>/world/` and `<vault>/plot/` files

### 2. Compose system prompts

For each role, call:

```
get-prompt(name: "agent-framing.md", vault: <vault>)
+ get-prompt(name: "review-base.md", vault: <vault>, vars: {Role: <role>, MaxIssues: 7})
+ get-prompt(name: "review-<role>.md", vault: <vault>)   // analytical/immersive/structural/adversarial-role
+ get-prompt(name: "verdict.md", vault: <vault>)
```

Roles:
- `analytical` (text-only) → use `review-analytical.md`
- `immersive` (text-only) → use `review-immersive.md`
- `structural` (full context) → use `review-structural.md`
- `adversarial` (full context) → use `review-adversarial-role.md`

### 3. Run reviewers in parallel

Default model mapping:
- analytical → Claude subagent
- immersive → Codex (invoke-codex)
- structural → Claude subagent
- adversarial → Pi (invoke-pi)

For each subagent (analytical, structural): use `Task` with the full system prompt and a user prompt containing the chapter text + style guide + issues + prior context.

For external calls: `invoke-codex` / `invoke-pi` with the system prompt; user prompt holds the chapter + style/issues/prior context inline (no manuscript flag — this is chapter-scoped).

### 4. Cross-review (text pair, full-context pair)

Run pairwise rebuttals using `get-prompt("cross-review.md", vault, vars: {MaxNewIssues: 3})`:
- analytical ↔ immersive
- structural ↔ adversarial

Spawn Claude rebuttals via subagent. For Codex/Pi rebuttals, resume their sessions (use the `session_id` returned from step 3).

### 5. Synthesize

Spawn a `Task` subagent with:
- System prompt: `get-prompt("agent-framing.md")` + `get-prompt("synthesis.md", vars: {ReviewNum: <padded>})` + `get-prompt("verdict.md")`
- User prompt: all four reviews + the two cross-review rebuttals, labeled

Get the review number from `next-review-number`.

### 6. Save

Stage parts (`stage-review-part`), then `assemble-review` with prefix `chapter-<name>-review`.

### 7. Present

Show the synthesis in conversation. Tell the user the saved file path.

## Notes

- Read the chapter from `<vault>/story/`, not from summaries or generated files.
- Issue IDs follow the global counter (review number from `next-review-number`).
- This is heavy — 4 reviews + 2 cross-reviews + synthesis = up to 7 agent calls. The manuscript skill is the same shape; the chapter version is for narrower focus.
