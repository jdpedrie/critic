---
name: assess
description: Deep-dive investigation of a single review issue against the storyline manuscript. Conversational. Pulls the issue and the relevant scenes, presents findings, and discusses with the user. Use when the user wants to dig into a flagged issue.
---

# Assess Issue

Conversational deep-dive on a specific review issue. You do the analysis directly in this session. No separate model call. The user can ask follow-up questions and the conversation continues with full context.

The vault path is the user's configured storyline project. Call `read-settings` if you don't already know it.

## $ARGUMENTS: Issue ID and optional direction

$ARGUMENTS should be an issue ID, optionally followed by initial direction.

Examples:

- `/critic:assess ISSUE-003-03`
- `/critic:assess 003-01 Is the trust fund actually load-bearing?`

## Workflow

### Step 1: Load the issue

Call `read-issue` with the issue ID. This returns the issue text from the review file it lives in.

Show the user what issue you're investigating.

### Step 2: Load what you need

Pull only what the issue requires. Don't slurp the whole manuscript.

Tools at your disposal:

- `list-scenes(vault: <vault>)`. Enumerate every scene in manuscript order. Cheap. Use first to know what's there.
- `read-scene(vault: <vault>, scene: <filename>)`. One scene's `#### <title>\n\n<body>` + frontmatter entities.
- `assemble-chapter(vault: <vault>, chapter: <N>)`. All scenes for one chapter, assembled.
- `assemble-manuscript(vault: <vault>)`. The entire book. Use only when the issue genuinely spans everything (e.g., a pattern that recurs).
- `read-codex(vault: <vault>, names: <comma-separated>)`. Codex entries filtered to the entities you care about.
- `read-codex-entry(vault: <vault>, name: <name>)`. One Codex entry.
- `read-research(vault: <vault>)`. Worldbuilding bibles.
- `find-entity-mentions(vault: <vault>, name: <name>)`. Every scene that mentions an entity, with bodies. Useful when the issue is "this character is passive" or "Henry never decides anything".

Pick based on the issue type. See Step 3.

### Step 3: Investigate

**Style or pattern issue** (e.g., "direct emotion naming throughout", "weak verbs", "every scene ends on a sigh"):

- Pull the manuscript (`assemble-manuscript`) or enumerate scenes and pull them in batches.
- Find every occurrence of the pattern.
- Quote each with its scene location (act/chapter/sequence/title).
- Be exhaustive. Exhaustiveness is the value you provide.

**Structural issue** (e.g., "passive protagonist", "mishandled arc", "Henry never chooses"):

- For protagonist/character issues: use `find-entity-mentions` to gather every scene featuring the character.
- For pacing/arc issues: enumerate scenes and pull each chapter's assembly.
- Trace the pattern across the slices where it appears.
- Quote scenes that demonstrate the problem.
- Identify where the fix would need to happen (which scene, what would change).
- Be concrete. \"Make Henry decide something\" is useless; "in scene 05-02, when Andersen offers the contract, Henry could push back instead of accepting" is useful.

**Single-scene or single-chapter issue**:

- Pull just that slice with `read-scene` or `assemble-chapter`.
- Quote the relevant text.
- Discuss the problem in detail.

**Canon issue** (e.g., "Luma's eye color contradicts the worldbuilding"):

- Pull the contested Codex entry (`read-codex-entry`) and the relevant Research file.
- Pull the prose slice(s) where the contradiction surfaces.
- Show both sides and propose the resolution paths.

### Step 4: Present findings

Present your investigation as readable prose. Quote text. Cite scene locations (`Act 1, Ch 1, Seq 2. Customs at Fontenoy`). Be specific.

### Step 5: Discuss

Offer to dig deeper, answer follow-up questions, or move on. The user may want to:

- Look at a specific occurrence in more detail
- Discuss potential fixes
- Compare with other issues
- Decide whether to rebut, defer, or fix

This is a conversation. Keep it going if the user wants to.

### Step 6: Save

Offer to save the outcome of the conversation to the vault for future reference.

Save location: `<vault>/notes/<issue-id>.md` (e.g., `<vault>/notes/ISSUE-003-03.md`). Create the `notes/` directory if it doesn't exist.

## Important Notes

- This is YOU doing the analysis directly, not delegating to another model. You have the full session context and the conversation continuity.
- If the user provides a follow-up question in $ARGUMENTS, address it as part of the investigation.
- Don't overload the context. Read what you need, not everything you can. `assemble-manuscript` is fine for genuine cross-book patterns; per-scene or per-chapter reads are cheaper for everything else.
- Quote the text. Cite scenes. Be specific.
- The user may want to spend a while on this. That's fine.
