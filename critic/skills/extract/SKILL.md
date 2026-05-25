---
name: extract
description: Extract ground-truth facts from a chapter and diff against the world-state notes. Use when the user wants to update canon or check what new facts a chapter introduces.
---

# Canon Extraction

The vault path is the user's configured vault. Call `read-settings` if needed.

## Arguments

$ARGUMENTS = chapter name (e.g. `chapter-03`).

## Workflow

### 1. Load context

- Read the chapter file from `<vault>/story/<chapter>.md` directly via Read
- Read all `<vault>/world/` files (recursively) via Glob + Read
- Read `<vault>/style.md` if it exists (passes through to the prompt)

### 2. Compose the prompt

```
system = get-prompt("agent-framing.md", vault: <vault>)
       + get-prompt("canon-extraction.md", vault: <vault>)

user = "=== STYLE GUIDE ===\n\n<style.md contents>\n\n"  (if present)
     + "=== EXISTING CANON ===\n\n<all world/ files concatenated with file headers>\n\n"
     + "=== CHAPTER TO EXTRACT FROM ===\n\n<chapter text>"
```

### 3. Run

Spawn a `Task` subagent (general-purpose) with the system + user prompts. The subagent's output is the extraction report.

### 4. Present

Show the extraction report in the conversation. Group facts by status:
- **New facts** — ask the user which to add to canon; help draft the entries
- **Confirmed** — summarize briefly
- **Contradictions** — for each, show chapter passage vs. canon, ask the user how to resolve

### 5. Apply (interactive)

For accepted updates, help the user draft frontmatter + content for the world files. Use Obsidian-style frontmatter so Bases queries work:

```markdown
---
type: character
name: Kael
status: active
introduced: chapter-01
last_updated: chapter-03
---

Body text here.
```

Do not auto-apply — always confirm with the user before writing.

## Notes

- Precision over recall. If you're not confident a fact is in the chapter, don't include it.
- Read the chapter directly. Do not use `summary/` files (out of date).
