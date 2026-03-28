---
name: extract
description: Extract ground truth facts from a chapter and compare against world state. Use when the user asks to extract canon, check consistency, or update world-building notes.
---

# Canon Extraction

The vault path for all tool calls is: ${user_config.vault_path}

Extract factual assertions from a chapter and diff them against the existing world state.

## Arguments

$ARGUMENTS should be a chapter name (e.g., "chapter-01") or a chapter filename.

## Workflow

### Step 1 — Extract

Call the `extract-canon` tool with the chapter name.

The tool returns a JSON object with a `facts` array. Each fact has:
- `type`: character, relationship, location, timeline, rule
- `entity`: the entity name
- `claim`: what the chapter asserts
- `source_passage`: quote from the chapter
- `canon_status`: confirmed, new, or contradicts
- `canon_reference`: path to the relevant canon file (if any)
- `conflict_detail`: explanation (if contradicts)

### Step 2 — Present Results

Group facts by status and present as readable text:

**New Facts** (not yet in canon):
For each, show the entity, claim, and source passage. Note which canon file it should be added to (or suggest creating a new one).

**Confirmed** (matches existing canon):
Brief summary — "X facts confirmed against existing canon."

**Contradictions** (conflicts with canon):
For each, show:
- What the chapter says (with quote)
- What canon says (with file reference)
- The nature of the conflict

### Step 3 — Author Decisions

Ask the user how to handle each category:
- **New facts**: "Add these to canon? I can propose frontmatter and content for new or updated files."
- **Contradictions**: "Which is correct — the chapter or the existing canon? Should I update the canon file or flag this as a chapter issue?"

### Step 4 — Apply (if requested)

For accepted updates, help the user write or update the canon files directly. Propose Obsidian-compatible markdown with frontmatter properties that work with Bases:

```markdown
---
type: character
name: Entity Name
status: active
introduced: chapter-XX
last_updated: chapter-XX
---

Freeform notes about the entity.
```

## Important Notes

- Present source passages as blockquotes so the user can verify.
- Do NOT auto-apply changes. Always present and ask first.
- Precision over recall — it's better to miss a minor fact than to flag a false contradiction.
