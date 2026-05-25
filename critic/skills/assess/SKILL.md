---
name: assess
description: Deep-dive investigation of a single review issue. Conversational — pulls the issue and the relevant manuscript context, presents findings, and discusses with the user. Use when the user wants to dig into a flagged issue.
---

# Assess Issue

Conversational deep-dive on a specific review issue. You do the analysis directly in this session — no separate model call. The user can ask follow-up questions and discuss findings.

The vault path for all tool calls is: ${user_config.vault_path}

## Arguments

$ARGUMENTS should be an issue ID, optionally followed by initial direction.

Examples:
- `/critic:assess ISSUE-003-03`
- `/critic:assess 003-01 Is the trust fund actually load-bearing?`

## Workflow

### Step 1 — Load the Issue

Call `read-issue` with the issue ID. This returns the issue text from the most recent matching review file.

Show the user what issue you're investigating.

### Step 2 — Load What You Need

**Read the actual chapter files directly from `story/` using your built-in Read tool.** Read them in order, by filename. These are the canonical, current text of the manuscript.

Do NOT use:
- `summary/` files — these are derived and may be out of date
- Generated manuscript files in `review/` or anywhere else — also derived
- `summarize-chapter` MCP tool — it just wraps the same files but adds noise

Use:
- Your built-in `Read` tool against `story/chapter-XX.md` directly
- Your built-in `Glob` tool against `story/*.md` to list chapters in order
- Read `world/` and `plot/` files directly if the issue requires that context

For style/pattern issues, read every chapter in order. For structural issues, read the chapters where the pattern manifests. For single-scene issues, read the chapter containing the scene plus immediate context.

### Step 3 — Investigate

Based on the issue type:

**Style or pattern issue** (e.g., "direct emotion naming throughout"):
- Read the chapters
- Find ALL occurrences of the pattern
- Quote each one with chapter and approximate location
- Be exhaustive — this is the value you provide

**Structural issue** (e.g., "passive protagonist", "mishandled arc"):
- Trace the pattern across the chapters where it appears
- Quote scenes that demonstrate the problem
- Identify where the fix would need to happen
- Be concrete — "make Henry decide something" is not useful; "in chapter 5, when X happens, Y could happen instead" is

**Single-scene issue**:
- Focus tightly on that scene
- Quote the relevant text
- Discuss the problem in detail

### Step 4 — Present Findings

Present your investigation as readable prose. Quote text. Cite chapter locations. Be specific.

### Step 5 — Discuss

Offer to dig deeper, answer follow-up questions, or move on. The user may want to:
- Look at a specific occurrence in more detail
- Discuss potential fixes
- Compare with other issues
- Decide whether to rebut, defer, or fix

This is a conversation. Keep it going if the user wants to.

### Step 6 - Save

Offer to save the outcome of the conversation to the vault for future reference.

Save location is `notes/ISSUE-NNN-NN.md`.

## Important Notes

- This is YOU doing the analysis directly, not delegating to another model. You have the full context and the conversation continuity.
- If the user provides a follow-up question in $ARGUMENTS, address it as part of the investigation.
- Don't overload the context. Read what you need, not everything you can.
- Quote the text. Cite chapters. Be specific.
- The user may want to spend a while on this. That's fine.
