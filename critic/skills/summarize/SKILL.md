---
name: summarize
description: Generate a per-chapter summary for every chapter in Story/. Writes one file per chapter to <vault>/summary/. Use when the user wants chapter summaries refreshed.
---

# Summarize Chapters

Produce one summary per chapter in `Story/`, written to `<vault>/summary/chapter-<NN>.md`. Run non-interactively.

The vault path is the user's configured book project (the folder containing `Story/`). Call `read-settings` if you don't already know it.

## Workflow

### Step 1: Enumerate chapters

Call `list-chapters(vault: <vault>)`. Each line starts with the chapter number; the lines are already in manuscript order.

### Step 2: Summarize each chapter, sequentially

For each chapter number (in order):

1. Call `assemble-chapter(vault: <vault>, chapter: <N>)`. The response gives `{text, entities, scene_count, title, file}`.

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
- Always overwrite. Re-running this skill should produce a fresh set of summaries that reflects the current chapters.
- The summary is factual reference text for the author. No verdicts, no commentary on craft.
- Summaries are NOT used by `/critic:delta`, the manuscript skills, or `/critic:review`. Those skills read the chapter files directly. Summaries exist for your (the author's) own reference.
