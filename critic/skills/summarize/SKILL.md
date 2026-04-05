---
name: summarize
description: Read and summarize each chapter, writing summaries to the summary/ directory. Use when the user wants chapter summaries generated or updated.
---

# Summarize Chapters

Read each chapter and write a summary to `summary/<chapter-name>.md`.

The vault path for all tool calls is: ${user_config.vault_path}

## Workflow

Run non-interactively through all chapters.

### Step 1 — List Chapters

Call `list-chapters` with the vault path to get all chapter names.

### Step 2 — Summarize Each Chapter

For each chapter, sequentially:

1. Call `summarize-chapter` with the vault path and chapter name. This returns the chapter text.

2. Write a summary of the chapter. The summary should include:
   - **Setting**: Where and when the chapter takes place
   - **Characters**: Who appears and their role in the chapter
   - **Events**: What happens, in order — the key plot beats
   - **State changes**: How characters, relationships, or situations change by the end
   - **Threads**: What threads are opened, advanced, or closed
   - A brief note on tone and pacing

   Keep each summary to roughly 200-400 words. Be factual, not evaluative — this is a reference document, not a review.

3. Call `write-summary` with the vault path, chapter name, and the summary content.

4. Report progress: "Saved summary for [chapter name]"

### Step 3 — Report

After all chapters are done, report the total number of chapters summarized.

## Important Notes

- Process chapters one at a time, sequentially. Do not parallelize — this keeps context manageable.
- If a summary already exists, overwrite it with the new version.
- These summaries are reference documents for the author, not reviews. Be neutral and precise.
