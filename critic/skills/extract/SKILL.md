---
name: extract
description: Extract canon-relevant facts from the manuscript and reconcile them against the Codex (Characters/Locations) and Research (worldbuilding bibles). Maintains .claude/codex-inventory.md. Three modes. Chapter, scene, entity. Every change requires interactive approval.
---

# Canon Extraction

The vault path is the user's configured storyline project. Call `read-settings` if you don't already know it.

This skill never writes Codex files or the inventory without your approval. Every proposed change is presented; you accept, reject, or edit before anything lands on disk.

## $ARGUMENTS: Mode and target

Parse the argument:

- `chapter <N>`. Extract from every scene in chapter N.
- `scene <filename>`. Extract from one scene (with or without `.md`).
- `entity <name>`. Pick an entity and scan every scene that mentions it across the whole project. The name is matched case-insensitively as a substring against scene body text (so `entity Henry` matches "Henry Nelson"); the skill confirms the exact entity with you before running if the match is ambiguous.
- Bare integer → `chapter <N>`.
- Anything else without an explicit prefix → ask the user whether they meant a scene filename or an entity name.

## Common preamble (all modes)

1. **Inventory load**. Read `<vault>/.claude/codex-inventory.md`. If it doesn't exist, treat as empty and create on first write. Hold as `inventory_state`.
2. **Roster**. Call `list-codex-entries(vault: <vault>)`. Hold as `entity_roster` (one name per line).
3. **Research**. Call `read-research(vault: <vault>)`. Hold as `research_block`.
4. **Codex (all entries)**. Call `read-codex(vault: <vault>)` with no `names` filter. Hold as `codex_block`. (You need this in full because extraction reasons about contradictions across entities.)

These four are independent. Call in parallel.

## Slice mode (chapter or scene)

### S1. Fetch the slice

- chapter: `assemble-chapter(vault, chapter: <N>)` → `{text, entities, scene_count}`. Set `slice_label = "Chapter <N>"`.
- scene: `read-scene(vault, scene: <filename>)` → `{text, entities, act, chapter, sequence, title}`. Set `slice_label = "Scene: <title> (Act <a>, Ch <c>, Seq <s>)"`.

### S2. Run extraction subagent

Spawn a `Task` subagent (`subagent_type: "general-purpose"`):

**System prompt**: `get-prompt(name: "extract-slice.md", vault: <vault>)`.

**User prompt**:

```
=== EXISTING CODEX (CHARACTERS & LOCATIONS) ===

<codex_block>

=== RESEARCH (WORLDBUILDING) ===

<research_block>

=== KNOWN ENTITY ROSTER ===

<entity_roster>

=== INVENTORY STATE ===

<inventory_state, or "(empty. No inventory file yet)">

=== SLICE TO EXTRACT FROM ===

Label: <slice_label>

<slice.text>
```

Capture the response as `extraction_report`.

### S3. Present and approve

Show `extraction_report` to the user in conversation, then walk through the action items in order. For each, ask before writing.

**Inventory updates**. For each proposed row delta:

- Show the delta concisely (`Henry Nelson. Update. Last touched: Act 1, Ch 1, Seq 2; no new pending facts`).
- Ask: accept / edit / skip.
- If accepted, edit `<vault>/.claude/codex-inventory.md` in place using `Edit` (find the existing `### <Entity name>:` heading and replace its block; or append a new block under the right kind heading if it's a new entity).

**Implicit-entity candidates**. For each:

- Show the candidate (name, kind, quoted passage).
- Ask: add to Codex (drafts an entry next), add to inventory as `missing`, mark `intentionally-absent`, or skip.

**Codex edits**. For each proposed Codex file change:

- Show the file path and the diff (or full proposed file if creating).
- Ask: accept / edit / skip.
- If accepted, write via `Write` (new files) or `Edit` (modifications). Preserve frontmatter the user has set unless the extraction is explicitly updating it.

### S4. Finalize

After all approved changes are written, show a summary:
- Codex files created/updated (with paths)
- Inventory rows added/updated
- Anything skipped (so the user remembers what they declined)

## Entity mode

### E1. Resolve the entity

If the entity name matches exactly one Codex entry (case-insensitive), use it.

If it's a substring of multiple roster entries, present the candidates and ask which one.

If it's not in the roster at all, ask: "No Codex entry named matching `<name>`. Treat this as an entity that should exist (missing), or were you searching the prose for mentions of a string that isn't an entity?" If they confirm it's an intended entity, proceed with their name.

Hold the resolved name as `entity_name`.

### E2. Gather mentions

Call `find-entity-mentions(vault: <vault>, name: <entity_name>)`. Hold the JSON array as `mentions`.

If `mentions` is empty, tell the user the entity isn't mentioned in any drafted scene, and stop. (Don't write to Codex or inventory if there's nothing to extract.)

Also fetch the current Codex entry, if any:
- Try `read-codex-entry(vault: <vault>, name: <entity_name>)`.
- If it returns `no Codex entry named ...`, hold `current_codex_entry = ""`.

### E3. Run extraction subagent

Spawn a `Task` subagent:

**System prompt**: `get-prompt(name: "extract-entity.md", vault: <vault>)`.

**User prompt**:

```
=== ENTITY ===

<entity_name>

=== EXISTING CODEX ENTRY ===

<current_codex_entry, or "(no Codex entry exists yet)">

=== RESEARCH (WORLDBUILDING) ===

<research_block>

=== INVENTORY STATE ===

<the entity's inventory row, or "(not in inventory yet)">

=== MENTIONS ===

<mentions, JSON verbatim>
```

Capture the response as `extraction_report`.

### E4. Present and approve

Show the report. Walk through:

**Contradictions** (if any). For each, present the conflict and the resolution options (a/b/c from the prompt). Get the user's choice. Do not proceed to the Codex edit until contradictions are resolved.

**Codex entry**. Show the proposed entry. If a Codex file already exists, show the diff against the current one; otherwise show the full new file. Ask: accept / edit / skip.

If accepted: write to `<vault>/Codex/Characters/<entity_name>.md` or `<vault>/Codex/Locations/<entity_name>.md` (ask the user which folder if it's a new entity and the kind isn't obvious from the extraction).

**Inventory**. Show the row. Ask: accept / edit / skip. If accepted, edit `.claude/codex-inventory.md`.

### E5. Finalize

Summary of what was written.

## Inventory file format

`<vault>/.claude/codex-inventory.md` uses sectioned-by-kind format. The skill should produce/preserve this shape:

```markdown
# Codex Inventory

Maintained by `/critic:extract`. Re-runs reconcile. Status values:

- `present`. Full Codex entry exists and reflects the manuscript
- `stub`. Codex entry exists but is sparse
- `missing`. Referenced in prose but no Codex entry yet
- `intentionally-absent`. Author has decided this entity does not need a Codex entry; do not re-suggest

## Characters

### <Entity name>: <status>

- Last touched: Act <A>, Ch <C>, Seq <S> (<scene title>)
- Pending facts:
  - <fact 1>
  - <fact 2>
- Notes: <free text, or none>

(repeat per entity, ordered by first-appearance Act/Chapter/Sequence)

## Locations
...

## Concepts
...

## Other
...
```

When updating, prefer `Edit` to replace one entity's block at a time. Keeps unrelated rows untouched.

When adding a new entity, append under the right kind heading. If a kind section is `(none yet)`, replace that placeholder with the new row.

Never silently delete a row marked `intentionally-absent`. Extract can re-suggest "you marked this absent. Still want to skip it?" but only if the user explicitly opts into a re-review.

## Notes

- The Codex (Characters/Locations) is the source of truth for what canon currently says. Research bibles are also canonical but more narrative. The manuscript prose is the source of new facts.
- The "manuscript wins" rule is per-author-decision. Contradictions flagged by the subagent must be resolved interactively. Don't paper over them.
- Do not auto-write anything. Every change is presented and approved.
- This skill does not invoke other reviewers. For a structural review of the slice, the user runs `/critic:review` separately.
