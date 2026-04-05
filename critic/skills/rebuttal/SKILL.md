---
name: rebuttal
description: Add an author rebuttal to a specific issue in a review, or defer it to issues.md. Conversational — helps refine the response before adding it.
---

# Rebuttal

Add an author rebuttal to a review issue, or defer it for later. This is a conversational skill — help the author decide how to handle the issue and articulate their response clearly.

The vault path for all tool calls is: ${user_config.vault_path}

## Arguments

$ARGUMENTS should be an issue ID (e.g., "ISSUE-003-01" or "003-01"), optionally followed by the author's initial reasoning.

Examples:
- `/critic:rebuttal ISSUE-003-01 This is intentional — the ambiguity resolves in chapter 7`
- `/critic:rebuttal 003-02`

## Workflow

### Step 1 — Read the Issue

Call `read-issue` with the issue ID to retrieve the full issue text.

Present the issue to the author so they can see exactly what they're rebutting.

### Step 2 — Discuss and Decide

If the author provided reasoning in $ARGUMENTS, use it as the starting point.
If not, ask what their position is.

Based on the discussion, the author will want one of three outcomes:

**A. Rebut** — The author disagrees with the issue or is making a deliberate choice.
**B. Defer** — The author acknowledges the issue but wants to address it later.
**C. Accept** — The author agrees and will fix it (no rebuttal needed — just acknowledge).

If the issue seems like something the author acknowledges but isn't ready to fix yet, **suggest deferring it to issues.md**. For example:
- "This sounds like you agree it needs work but want to handle it later. Want me to add it to your issues list instead of rebutting it?"
- The author might say "yeah, punt that one" or "add it to the backlog"

### Step 3A — Rebut

Help the author refine their rebuttal:
- It should be clear and specific
- If it's a matter of author discretion (style choice, deliberate ambiguity, pacing preference), say so — these are not open to continued debate once stated
- If it references future plans ("this resolves in chapter 7"), note that

Present the final rebuttal text and ask: **"Add this rebuttal?"**

Once confirmed, call `add-rebuttal` with the issue ID and the finalized rebuttal text.

### Step 3B — Defer

Determine the right heading:
- Use "General" for manuscript-wide issues (pacing, tone, arc concerns)
- Use a chapter name (e.g., "Chapter 2") for chapter-specific issues

Format the entry with the issue ID and a brief description:

```
### ISSUE-003-02: Sam's introduction runs long
Acknowledged — will tighten in a future revision. Keeping the core exchange with Luma.
```

Call `append-issue` with the heading and entry text.

Also add a brief rebuttal to the review file noting the deferral:
Call `add-rebuttal` with something like: "Acknowledged. Deferred to issues.md for future revision."

### Step 3C — Accept

Just acknowledge. No tool calls needed — the author will fix it in their own time.

## Important Notes

- This skill IS conversational. Help the author think through their response.
- Rebuttals will be visible to future reviewers and should be authoritative.
- Deferred issues in issues.md will be visible to future reviewers with instructions to only re-raise if the issue has escalated.
- Suggest deferring when the author's response is along the lines of "yeah I know, I'll get to it" — that's a deferral, not a rebuttal.
