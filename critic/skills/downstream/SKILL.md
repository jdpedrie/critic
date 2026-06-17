---
name: downstream
description: Assess downstream effects of editing a chapter or scene. Reads the edited slice and every scene after it; flags continuity breaks, invalidated setups, character-state errors, dialogue references to removed content, and timeline issues.
---

# Downstream Assessment

The vault path is the user's configured storyline project. Call `read-settings` if you don't already know it.

## $ARGUMENTS: Target

Parse the argument:

- `chapter <N>`. Treat chapter N as the edit point; assess every scene from chapter N+1 onward.
- `scene <filename>`. Treat that scene as the edit point; assess every scene that comes after it in manuscript order.
- Bare integer → `chapter <N>`.
- Anything else → `scene <arg>`.

## Workflow

### 1. Determine the edit point and downstream slice

Call `list-scenes(vault: <vault>)`. Parse the lines (`<act>/<chapter>/<sequence> | <filename> | <title>`).

- **Chapter mode**: the edit point is "chapter N". Downstream = every scene whose `chapter:` > N (within the same act; if later acts exist, include those too).
- **Scene mode**: the edit point is the named scene. Downstream = every scene that sorts after it in (act, chapter, sequence) order.

Fetch the edit-point text:
- Chapter mode: `assemble-chapter(vault, chapter: N)` → `edit_text`
- Scene mode: `read-scene(vault, scene: <filename>)` → `edit_text`

Fetch downstream text:
- For each downstream scene filename, call `read-scene(vault, scene: <filename>)` in parallel batches of 8. Concatenate the returned `text` blocks in manuscript order, with `## <Act A, Ch C, Seq S>: <title>` headers between them. Hold as `downstream_text`.

### 2. Pull grounding context

In parallel:
- `read-style-guide(vault: <vault>)` → `style_block`
- `read-research(vault: <vault>)` → `research_block`
- `read-codex(vault: <vault>)` (no `names` filter. The edit may touch any entity) → `codex_block`

### 3. Compose prompts

```
system = get-prompt(name: "agent-framing.md", vault: <vault>)
       + get-prompt(name: "downstream.md", vault: <vault>)

user = optional sections (skip if empty):
       "=== STYLE GUIDE ===\n\n<style_block>\n\n"
     + "=== RESEARCH (WORLDBUILDING) ===\n\n<research_block>\n\n"
     + "=== CODEX (CHARACTERS & LOCATIONS) ===\n\n<codex_block>\n\n"
     + "=== EDITED SLICE ===\n\n"
     + "<chapter/scene label>\n\n<edit_text>\n\n"
     + "=== DOWNSTREAM SCENES ===\n\n<downstream_text>"
```

### 4. Run

Spawn a `Task` subagent (`subagent_type: "general-purpose"`) with the system + user prompts. It reads carefully and reports issues grouped by affected scene.

### 5. Present

Show the assessment in conversation, grouped by affected scene (Act/Chapter/Sequence + title). Lead with the most disruptive issues. For each:

- Quote the downstream passage that's affected.
- Explain what in the edited slice caused it.
- Suggest a concrete fix (which lines, which scene).

Offer to dig deeper into any specific issue or to spawn a `/critic:review` on a flagged scene.

## Notes

- This reads the edited slice through end-of-manuscript. Token use is proportional to how late in the manuscript the edit lands. An Act 1 chapter edit reads a lot of downstream prose. Claude handles it.
- Do not read scene files directly. Go through `read-scene` so wikilinks are stripped consistently with what reviewers see elsewhere.
- For scoped continuity checks (e.g., "did anything break in chapter 5 specifically"), the user can re-run with `chapter <N>` set to the boundary they care about, or use `/critic:assess` on a specific issue.
