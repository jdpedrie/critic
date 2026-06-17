---
name: summarize
description: Generate a per-chapter summary for every chapter in the storyline project. Writes one file per chapter to <vault>/summary/. Use when the user wants chapter summaries refreshed.
---

# Summarize Chapters

Produce one summary per chapter of the storyline project, written to `<vault>/summary/chapter-<NN>.md`. Run non-interactively.

The vault path is the user's configured storyline project. Call `read-settings` if you don't already know it.

## Workflow

### Step 1: Enumerate chapters

Call `list-scenes(vault: <vault>)`. Parse the `<act>/<chapter>/<sequence>` segment of each line. Build the set of distinct chapter numbers in manuscript order.

### Step 2: Summarize each chapter, sequentially

For each chapter number (in order):

1. Call `assemble-chapter(vault: <vault>, chapter: <N>)`. The response gives `{text, entities, scene_count}`.

2. Compose the summary yourself (no subagent). The summary should cover:
   - Setting: where and when (in-story) the chapter takes place
   - Characters: who appears and their role in the chapter
   - Events: what happens, in order: the key plot beats
   - State changes: how characters, relationships, or situations change by the end
   - Threads: what threads are opened, advanced, or closed
   - A brief note on tone and pacing

   Target 200–400 words. Be factual, not evaluative. This is a reference document, not a review.

3. Write the summary to `<vault>/summary/chapter-<NN>.md` where `<NN>` is the chapter number zero-padded to two digits. Use the parent's `Write` tool directly (no MCP needed). Overwrite any existing file.

4. Report progress: "Saved summary for chapter <N>".

### Step 3: Report

After all chapters are done, report the total count.

## Important Notes

- Process chapters sequentially, not in parallel. Keeps context manageable and makes progress legible.
- Always overwrite. Re-running this skill should produce a fresh set of summaries that reflects the current scenes.
- The summary is factual reference text for the author. No verdicts, no commentary on craft.
- Summaries are NOT used by `/critic:manuscript` or `/critic:review`. Those skills read scenes directly. Summaries exist for your (the author's) own reference.
