---
name: close-read
description: Line-editor / copy-editor pass over one scene, one chapter, or every scene in the project. Spawns a Claude subagent per scene (parallel, batched for `all`) to surface typos, prose-level issues, micro-structure problems, canon contradictions, and style-guide violations. This is NOT the publishing-consultant review. Use /critic:review for that.
---

# Close Read

The vault path is the user's configured storyline project. Call `read-settings` if you don't already know it.

The close-read role is **explicitly different** from `/critic:manuscript` and `/critic:review`. Those reviewers are instructed not to hunt for typos. This skill IS the typo / prose / line-edit pass. Don't conflate the roles when reporting back to the user.

## $ARGUMENTS: Target

Parse the argument:

- `scene <filename>`. One scene. Filename with or without `.md`. Example: `/critic:close-read scene 01-01 Customs at Fontenoy`.
- `chapter <N>`. Every scene whose `chapter:` frontmatter equals N. Example: `/critic:close-read chapter 3`.
- `all`. Every scene in the project, batched in waves of 8.
- Bare integer → `chapter <N>`.
- Anything else → `scene <arg>`.

Hold the parsed mode for use below.

## Execution

### 1. Enumerate scenes

- **scene mode**: call `read-scene(vault: <vault>, scene: <filename>)` once. The response gives you `{text, entities, act, chapter, sequence, title}` for that single scene. Treat it as a list of one.
- **chapter mode**: call `assemble-chapter(vault: <vault>, chapter: <N>)` to confirm the chapter exists; then call `list-scenes(vault: <vault>)` and filter to lines whose `chapter` segment matches N. For each matching scene, you'll fetch its text via `read-scene` in step 3.
- **all mode**: call `list-scenes(vault: <vault>)`. Every returned line is in scope.

`list-scenes` returns one line per scene as `<act>/<chapter>/<sequence> | <filename> | <title>`. Parse those.

### 2. Set up the run

Generate a run ID: timestamp formatted `YYYY-MM-DD-HHMMSS` (use the parent's date helpers). Output directory: `<vault>/review/close-read/<run-id>/`. Create it.

Call these once, in parallel. They're the same for every subagent in the run:

- `read-style-guide(vault: <vault>)` → `style_block`
- `get-prompt(name: "close-read.md", vault: <vault>)` → `close_read_system_prompt`

If `style_block` is empty, the close-read prompt instructs the subagent to skip the STYLE category entirely; no special handling required from you.

### 3. Spawn one subagent per scene

**Batching**:
- scene mode (1 scene): no batching, just spawn it.
- chapter mode (typically 2–6 scenes): spawn all in parallel in a single turn.
- all mode (potentially 38+ scenes): spawn in **waves of 8**. After each wave finishes, write its files (step 4), then spawn the next wave. This keeps token use predictable and lets the user see progress.

For each scene in the current wave, in parallel:

1. Fetch the slice and its entities:
   ```
   read-scene(vault: <vault>, scene: <filename>)
   ```
   → `slice_text`, `slice_entities`.

2. Pull the codex entries for those entities:
   ```
   read-codex(vault: <vault>, names: <slice_entities comma-joined>)
   ```
   → `slice_codex` (may be empty for scenes with no frontmatter entities).

3. Spawn the subagent (`Task` tool, `subagent_type: "general-purpose"`):

   System prompt = `close_read_system_prompt`.

   User prompt = the following blocks, in order, omitting empty ones:

   ```
   === STYLE GUIDE ===

   <style_block>

   === CODEX (CHARACTERS & LOCATIONS) ===

   <slice_codex>

   === SCENE UNDER REVIEW ===

   Filename: <filename>
   Act <act>, Chapter <chapter>, Sequence <sequence>
   Title: <title>

   <slice_text>
   ```

   Tell the subagent its slice label is `Scene: <title>` (Act <act>, Ch <chapter>, Seq <sequence>) so the report's header is consistent.

   The subagent has the `read-codex-entry` MCP tool available for ad-hoc Codex lookups. The system prompt already explains when to use it.

4. Capture the response as that scene's report.

### 4. Write per-scene files

For each completed subagent, write the report to:

```
<vault>/review/close-read/<run-id>/<act>-<chapter>-<sequence>-<scene-slug>.md
```

Where `<scene-slug>` is the filename (without `.md`) with spaces replaced by `-` and characters outside `[A-Za-z0-9-]` stripped. Pad act/chapter/sequence to two digits each so the directory sorts in manuscript order.

Use the parent's `Write` tool directly. No MCP needed; this is straight file I/O at known paths.

### 5. Build the index

After all subagents have returned (or after the last wave in `all` mode), write `<vault>/review/close-read/<run-id>/index.md`:

```
# Close Read: <run-id>

Mode: <scene | chapter <N> | all>
Scenes reviewed: <count>

## Scenes

- [<act>/<chapter>/<sequence>: <title>](./<filename>): <one-line summary from the report's opening paragraph>
- ...
```

The one-line summary comes from the first paragraph of each per-scene report. If a report has no opening paragraph (just sections), use "no opening summary".

Order index entries by act → chapter → sequence.

### 6. Present

Tell the user:
- Total scenes reviewed and the run directory path.
- The aggregate counts across all scenes (e.g., "12 typos, 47 prose issues, 3 canon contradictions, 8 style notes").
- If any scene had a canon contradiction, flag those scenes by name. Those are the ones that need editor attention rather than just author preference.

Do not paste full reports in conversation. The per-scene files are on disk. If the user wants to see one, they can open it.

## Notes

- Close-read is a separate role from `/critic:manuscript` and `/critic:review`. Do not invoke or chain to those skills from here. The output goes to its own directory tree, not to `review/NNN-*.md`.
- Subagent per scene is the architecture choice. A narrow attention surface catches more granular issues than one large subagent reading everything.
- Suggested rewrites are constrained by the prompt to preserve voice and follow the style guide. If the user reports the suggestions feel "AI-smoothed", the fix is in `prompts/close-read.md` (the voice constraints section).
- `all` mode on a 38-scene project is roughly 5 waves of 8. Each wave takes whatever a single subagent takes. They're parallel within the wave.
