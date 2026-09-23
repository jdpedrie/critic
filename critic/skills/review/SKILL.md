---
name: review
description: Chapter- or scene-level multi-agent review. Spawns parallel reviewers (Claude subagents, Codex, Pi) for one chapter or one scene with four role lenses (analytical, immersive, structural, adversarial). Use when the user asks to review a specific chapter or scene. For a full-manuscript review with interactive setup, use /critic:manuscript-craft or /critic:manuscript-publication; for changes-only review, /critic:delta.
---

# Chapter / Scene Review

The vault path is the user's configured book project (the folder containing `Story/`). Call `read-settings` if you don't already know it.

## $ARGUMENTS: Target slice

Parse the argument:

- `chapter <N>`. Review chapter N (the file in `Story/` whose `chapter:` is N), all its scenes in order. Example: `/critic:review chapter 3`.
- `scene <id>`. Review one scene, by its address `CC-SS` (chapter, then position within the chapter) or its exact title. Example: `/critic:review scene 01-01`. Old scene filenames like `01-01 Customs at Fontenoy` still resolve.
- Bare integer → treat as `chapter <N>`.
- Anything else → treat as `scene <arg>`.

Hold the parsed mode (`chapter` or `scene`) and target (number or scene reference) for use below.

## Setup

This skill does not prompt. It just runs. If the user wants to customize which reviewers run, they should use `/critic:manuscript-craft` or `/critic:manuscript-publication` (interactive setup).

## Frame

Read `frame` from `read-settings`. It picks the framing, synthesis, and verdict prompts:

| `frame` | framing | synthesis | verdict |
|---------|---------|-----------|---------|
| unset or `publication` | `agent-framing.md` | `synthesis.md` | `verdict.md` |
| `craft` | `craft-framing.md` | `synthesis-craft.md` | `verdict-craft.md` |

The role prompts (`review-base.md`, `review-<role>.md`) and `cross-review.md` are the same in both frames. Hold the three names as `framing_prompt`, `synthesis_prompt`, `verdict_prompt`.

## Execution

### 1. Fetch the slice

**Chapter mode**: call `assemble-chapter(vault: <vault>, chapter: <N>)`. Returns JSON `{text, entities, scene_count, title, file}`. Hold:
- `slice_text` = the assembled chapter markdown
- `slice_entities` = the union of POV/character/location names in the chapter's frontmatter
- `slice_label` = `"Chapter <N>: <title>"`, or `"Chapter <N>"` if untitled (use this in user-facing output)

**Scene mode**: call `read-scene(vault: <vault>, scene: <id>)`. Returns JSON `{id, chapter, scene, chapter_title, title, text, entities}`. Hold:
- `slice_text` = the scene markdown
- `slice_entities` = the entities from this one scene
- `slice_label` = `"Scene <id>: <title>"`

### 2. Gather context

Call these in parallel. They're independent reads:

- `read-stage(vault: <vault>)` → `stage_block`
- `read-style-guide(vault: <vault>)` → `style_block`
- `read-research(vault: <vault>)` → `research_block`
- `read-codex(vault: <vault>, names: <slice_entities comma-joined>)` → `codex_block` (filtered to entities in the slice. Saves tokens and keeps reviewers focused)
- `read-issues(vault: <vault>)` → `issues_block`

If a block is empty, skip it in the user prompt (don't emit a header for an empty section).

### 3. Compose system prompts

For each role, concatenate:

```
get-prompt(name: <framing_prompt>, vault: <vault>)
+ get-prompt(name: "review-base.md", vault: <vault>, vars: {Role: <role>, MaxIssues: 7})
+ get-prompt(name: "review-<role>.md", vault: <vault>)   // analytical / immersive / structural / adversarial-role
+ get-prompt(name: <verdict_prompt>, vault: <vault>)
```

Roles:
- `analytical`. Text-craft focus
- `immersive`. Reader-experience focus
- `structural`. Full-context structural focus
- `adversarial`. Full-context contrarian read (prompt file is `review-adversarial-role.md`)

### 4. Build user prompt prefix

Construct (in this order, only sections that apply):

- `=== CURRENT DRAFT STAGE ===\n\n<stage_block>`. Calibrates everything else, include first.
- `=== STYLE GUIDE ===\n\n<style_block>`
- `=== WORLDBUILDING (RESEARCH) ===\n\n<research_block>`
- `=== CODEX (CHARACTERS & LOCATIONS) ===\n\n<codex_block>`. Pre-filtered to entities in this slice.
- `=== KNOWN ISSUES ===\n\n<issues_block>`
- `=== TARGET FOR THIS REVIEW ===\n\n<slice_label>\n\n<slice_text>`. The chapter or scene under review.

Hold this as `user_prompt`. It carries everything the reviewer needs; the slice itself is inline (chapter/scene reviews don't use `include_manuscript_from` because they're scoped, not whole-book).

### 5. Run reviewers in parallel

Default model mapping:
- `analytical` → Claude subagent (`Task` with `subagent_type: "general-purpose"`)
- `immersive` → Codex (`invoke-codex`)
- `structural` → Claude subagent
- `adversarial` → Pi (`invoke-pi`)

For each Claude subagent: prompt = `<system_prompt for that role>\n\n---\n\n<user_prompt>`.

For Codex: `invoke-codex(system_prompt: <system for immersive>, user_prompt: <user_prompt>)`. Capture the session_id for cross-review.

For Pi: `invoke-pi(system_prompt: <system for adversarial>, user_prompt: <user_prompt>)`. Capture the session_id.

**Collecting results.** `invoke-codex` and `invoke-pi` return either
`{status:"done", response, session_id}` or, when the model takes longer than
the call's `wait_seconds` budget, `{status:"running", job_id}`. A running
status is not an error. Poll it with `invoke-status(job_id: <id>,
wait_seconds: 60)` until it reports `done`. Start every reviewer before you
collect any of them, so they run concurrently.


Hold the responses as `analytical_review`, `immersive_review`, `structural_review`, `adversarial_review` (plus session IDs for the external two).

### 6. Cross-review (pairwise rebuttals)

Two pairs:
- analytical ↔ immersive (text-only pair)
- structural ↔ adversarial (full-context pair)

For each side of each pair, the rebuttal prompt is `get-prompt("cross-review.md", vault, vars: {MaxNewIssues: 3})`. The user message is the *other* side's review verbatim, labeled (e.g. `## Immersive's Review\n\n<text>`).

- Claude rebuttals → subagent.
- Codex rebuttal → `invoke-codex(system_prompt: <cross-review prompt>, user_prompt: <other review>, session_id: <codex_session_id>)`.
- Pi rebuttal → `invoke-pi(system_prompt: <cross-review prompt>, user_prompt: <other review>, session_id: <pi_session_id>)`.

Capture as `<role>_rebuttal`.

### 7. Synthesize

`next-review-number(vault: <vault>)` → review number `N`. Pad to 3 digits (e.g. `7 → 007`).

Spawn a `Task` subagent with:
- System: `get-prompt(<framing_prompt>)` + `get-prompt(<synthesis_prompt>, vars: {ReviewNum: "<padded>"})` + `get-prompt(<verdict_prompt>)`
- User: all four reviews + the four rebuttals, each labeled by role.

Capture as `synthesis`.

### 8. Stage and save

Stage every artifact (`stage-review-part`):
- `synthesis` → `<synthesis>` (raw, no heading)
- `analytical-review` → `# Analytical Review\n\n<analytical_review>`
- `immersive-review` → `# Immersive Review\n\n<immersive_review>`
- `structural-review` → `# Structural Review\n\n<structural_review>`
- `adversarial-review` → `# Adversarial Review\n\n<adversarial_review>`
- `analytical-rebuttal` → `# Analytical Cross-Review Rebuttal\n\n<analytical_rebuttal>`
- `immersive-rebuttal` → `# Immersive Cross-Review Rebuttal\n\n<immersive_rebuttal>`
- `structural-rebuttal` → `# Structural Cross-Review Rebuttal\n\n<structural_rebuttal>`
- `adversarial-rebuttal` → `# Adversarial Cross-Review Rebuttal\n\n<adversarial_rebuttal>`

Then assemble:

- Chapter mode: `assemble-review(vault, prefix: "chapter-<N>-review", synthesis_part: "synthesis", raw_parts: "analytical-review,immersive-review,structural-review,adversarial-review,analytical-rebuttal,immersive-rebuttal,structural-rebuttal,adversarial-rebuttal")`
- Scene mode: prefix = `scene-<id>-<title-slug>-review` (e.g. `scene-04-02-departure-for-arcadia-review`). Slugify the title by lowercasing, replacing spaces with `-`, and stripping characters outside `[a-z0-9-]`.

### 9. Present

Show the synthesis in conversation. Note the saved file path and the slice label.

## Notes

- The slice is always inline in the user prompt (no `include_manuscript_from`). Chapter and scene reviews are scoped, not whole-book. The manuscript skill is the one that flags the full manuscript via `include_manuscript_from`.
- Codex is pre-filtered to entities named in the chapter frontmatter for the slice (POV, characters, locations, and each scene's metadata). Reviewers see Henry's character file because Henry's in the scene; they don't see 36 other character files they don't need.
- Issue IDs use the global counter from `next-review-number`, padded to 3 digits.
- This is heavy. 4 reviews + 4 rebuttals + synthesis = up to 9 agent calls. For lighter work on a single scene, `/critic:close-read` is the line-editor variant.
