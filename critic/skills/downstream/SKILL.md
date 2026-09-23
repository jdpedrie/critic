---
name: downstream
description: Assess downstream effects of editing a chapter or scene. Reads the edited slice and everything after it; flags continuity breaks, invalidated setups, character-state errors, dialogue references to removed content, and timeline issues.
---

# Downstream Assessment

The vault path is the user's configured book project (the folder containing `Story/`). Call `read-settings` if you don't already know it.

## $ARGUMENTS: Target

Parse the argument:

- `chapter <N>`. Treat chapter N as the edit point; assess every chapter after it.
- `scene <id>`. Treat that scene (address `CC-SS`, or exact title) as the edit point; assess the rest of its chapter and every chapter after it.
- Bare integer → `chapter <N>`.
- Anything else → `scene <arg>`.

## Workflow

### 1. Determine the edit point and downstream slice

Call `list-scenes(vault: <vault>)`. Parse the lines (`<id> | <chapter file> | <title>`). IDs are `CC-SS`: chapter, then position within the chapter.

- **Chapter mode**: the edit point is chapter N. Downstream = every chapter numbered above N.
- **Scene mode**: the edit point is the named scene. Downstream = the scenes after it in the same chapter, then every later chapter.

Fetch the edit-point text:
- Chapter mode: `assemble-chapter(vault, chapter: N)` → `edit_text`
- Scene mode: `read-scene(vault, scene: <id>)` → `edit_text`

Fetch downstream text, in manuscript order:
- Scene mode only: for each later scene in the edit point's chapter, `read-scene(vault, scene: <id>)`.
- Every later chapter: `assemble-chapter(vault, chapter: <M>)`. Call these in parallel batches of 8.

Concatenate in manuscript order. Chapter text already carries `### Chapter M` and `#### <scene title>` headings; put a `## <id> <title>` header in front of each scene you fetched individually, and label chapter scenes by ID when you cite them (list-scenes gives you the mapping). Hold as `downstream_text`.

### 2. Pull grounding context

In parallel:
- `read-style-guide(vault: <vault>)` → `style_block`
- `read-research(vault: <vault>)` → `research_block`
- `read-codex(vault: <vault>)` (no `names` filter. The edit may touch any entity) → `codex_block`

### 3. Compose prompts

`<framing>` is `craft-framing.md` when the `frame` setting (from `read-settings`) is `craft`, otherwise `agent-framing.md`.

```
system = get-prompt(name: <framing>, vault: <vault>)
       + get-prompt(name: "downstream.md", vault: <vault>)

user = optional sections (skip if empty):
       "=== STYLE GUIDE ===\n\n<style_block>\n\n"
     + "=== RESEARCH (WORLDBUILDING) ===\n\n<research_block>\n\n"
     + "=== CODEX (CHARACTERS & LOCATIONS) ===\n\n<codex_block>\n\n"
     + "=== EDITED SLICE ===\n\n"
     + "<chapter/scene label>\n\n<edit_text>\n\n"
     + "=== DOWNSTREAM ===\n\n<downstream_text>"
```

### 4. Run

Spawn a `Task` subagent (`subagent_type: "general-purpose"`) with the system + user prompts. It reads carefully and reports issues grouped by affected scene.

### 5. Present

Show the assessment in conversation, grouped by affected scene (scene ID + title). Lead with the most disruptive issues. For each:

- Quote the downstream passage that's affected.
- Explain what in the edited slice caused it.
- Suggest a concrete fix (which lines, which scene).

Offer to dig deeper into any specific issue or to spawn a `/critic:review` on a flagged scene.

## Notes

- This reads the edited slice through end-of-manuscript. Token use is proportional to how late in the manuscript the edit lands. An Act 1 chapter edit reads a lot of downstream prose. Claude handles it.
- Do not read chapter files directly. Go through `read-scene` and `assemble-chapter` so frontmatter is left out and wikilinks are stripped, consistent with what reviewers see elsewhere.
- For scoped continuity checks (e.g., "did anything break in chapter 5 specifically"), the user can re-run with `chapter <N>` set to the boundary they care about, or use `/critic:assess` on a specific issue.
