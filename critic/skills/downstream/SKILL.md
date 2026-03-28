---
name: downstream
description: Assess downstream effects of editing a chapter. Use when the user has edited a chapter and wants to know what breaks in later chapters.
---

# Downstream Assessment

The vault path for all tool calls is: ${user_config.vault_path}

Assess what breaks in later chapters after editing a chapter.

## Arguments

$ARGUMENTS should be a chapter name (e.g., "chapter-03").

## Workflow

1. Call the `assess-downstream` tool with the vault path (current working directory) and chapter name.

2. Present results as readable text:

   **Summary**: The overall assessment.

   **Downstream Issues** (grouped by severity):
   For each issue:
   - Which chapter is affected
   - What breaks and why (referencing the edit that caused it)
   - Quote from the affected chapter
   - Suggested fix

   **Safe Chapters**: List chapters that appear unaffected.

3. After presenting, offer:
   - "Want to look at any specific affected chapter?"
   - "Should I run a full review on any of the affected chapters?"

## Important Notes

- This tool reads from the edited chapter through the END of the manuscript. For long manuscripts this is a large context call.
- Present issues in severity order: critical first.
- Quote the downstream evidence so the user can verify.
