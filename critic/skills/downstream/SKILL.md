---
name: downstream
description: Assess downstream effects of editing a chapter. Reads the edited chapter and every chapter after it; flags continuity breaks, invalidated setups, character state errors, dialogue references to removed content, and timeline issues.
---

# Downstream Assessment

The vault path is the user's configured vault. Call `read-settings` if needed.

## Arguments

$ARGUMENTS = chapter name (e.g. `chapter-03`).

## Workflow

### 1. Load context

- Read the edited chapter from `<vault>/story/<chapter>.md`
- Glob `<vault>/story/*.md`, sort by filename, read every chapter after the edited one
- Read `<vault>/world/` and `<vault>/plot/` files for grounding
- Read `<vault>/style.md` if present

### 2. Compose the prompt

```
system = get-prompt("agent-framing.md", vault: <vault>)
       + get-prompt("downstream.md", vault: <vault>)

user = "=== STYLE GUIDE ===\n\n<style.md>\n\n"  (if present)
     + "=== CANON ===\n\n<world files>\n\n"  (if present)
     + "=== PLOT ===\n\n<plot files>\n\n"  (if present)
     + "=== EDITED CHAPTER ===\n\n<edited chapter text>\n\n"
     + "=== DOWNSTREAM CHAPTERS ===\n\n<each later chapter, with `--- chapter-NN ---` headers>"
```

### 3. Run

Spawn a `Task` subagent (general-purpose) with the prompts. The subagent reads carefully and reports issues.

### 4. Present

Show the assessment in the conversation, grouped by affected chapter. Lead with critical issues. For each issue:
- Quote the relevant downstream passage
- Explain what in the edited chapter caused it
- Suggest a fix

Offer to dig deeper into any specific issue or to run a chapter review on flagged chapters.

## Notes

- This reads from the edited chapter through end-of-manuscript. For long manuscripts, the context is large but Claude handles it.
- Read chapter files directly. Do not use `summary/`.
