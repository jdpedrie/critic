# Workflows

Example sessions, in roughly the order an author would hit them on a new project.

## Day 1: set up a vault

You have a book folder at `/Users/me/obsidian/Vault/MyNovel/` with a `Story/` folder holding a few drafted chapters. `Background/` may be sparse or empty. That's fine for now.

```
/critic:settings vault_path /Users/me/obsidian/Vault/MyNovel
/critic:settings frame craft
/critic:settings pi_provider google
/critic:settings pi_model gemini-2.5-pro
/critic:settings adversary_provider openai
/critic:settings adversary_model gpt-5
```

`frame craft` is for a book with no publisher in the picture: reviewers judge the pages against what you say the book is trying to be. Leave it unset (publication) if you're heading for submission and want reviewers judging against a saleable finished book.

Write a `stage.md` at the project root. The auto-derived block works, but it only knows chapter counts. A hand-written one tells reviewers what the book is for and what you want it to be, which is what the craft frame measures against:

```markdown
# Current Stage

This is the first 30,000 words of a planned 120,000-word novel. Three-act structure.

## What this project is

A book I write for my own satisfaction, when I feel like it. No deadline,
no submission target. Judge the pages, not the pace.

## What I want this book to be

- A slow-burn mystery where the strangeness is felt before it's explained
- A relationship under power asymmetry that neither party fully sees

## What this draft is currently trying to do

- Establish the central mystery in Act 1
- Develop the protagonist's relationship with their attendant
- Set up the financial pressure that drives Act 2

## Not yet attempted, by design

- Resolution of any major thread
- Mid-book reversals
- Character arcs completing
```

This block goes first in every reviewer's prompt. Without it, reviewers will judge the manuscript as a finished book and tell you things you already know ("the arc doesn't resolve") instead of what you need to know ("the foundation Henry's arc would rest on isn't there yet").

## The first manuscript review

```
/critic:manuscript-craft
```

Interactive. The skill asks which reviewers (default: all three), which Pi model (the skill runs `pi --list-models` and shows you), which frank-reader model (usually pick a different provider than the primary Pi for different bias and more useful pushback), and which steps to run (default: everything on).

Then it runs straight through. Roughly 5 to 10 minutes for a 30,000-word manuscript with all reviewers and the cross-review matrix. The output is one review file at `Review/001-manuscript-craft-YYYY-MM-DD-HHMMSS.md`. This run also writes the first snapshot in the craft lineage, which is the baseline every later craft review diffs against.

Read the synthesis. Each issue has a stable ID like `ISSUE-001-04`. Decide what to do with each.

`/critic:manuscript-publication` is the same run under the publication frame. It keeps its own snapshot lineage: publication reviews diff against publication snapshots, craft reviews against craft ones, and neither ever sees the other's. If you've been running publication reviews and switch to craft, the first craft review has no baseline and says so; the next one diffs from there.

## Responding to a review

For each issue you don't immediately fix, run `/critic:rebuttal <issue-id>`. Three outcomes.

Rebut: "Intentional. The ambiguity resolves in chapter 7." The rebuttal is added as an Obsidian callout inline in the review file. Future reviewers see it and respect it.

Defer: "Acknowledged, will tighten in revision." The issue goes to `issues.md`. Future reviewers see it but are told not to re-raise unless it has materially escalated.

Accept: you'll fix it. No tool calls.

In the craft frame there's a fourth outcome: do nothing. Reviewers treat an unaddressed issue as your sequencing decision and won't count how long it's been open. Rebuttals and deferrals still help, because they say why.

If you want to dig into an issue before deciding:

```
/critic:assess ISSUE-001-04
/critic:assess 001-04 Is Henry actually passive in the chapter, or only in this scene?
```

The orchestrator pulls only the slices it needs and discusses with you. Conversational. You can save the outcome to `notes/ISSUE-001-04.md` for future reference.

## After drafting more chapters

Now you have a new chapter and a revised scene. Three options, from light to heavy.

### Option A: delta review with author note

```
/critic:delta added the chapel scene to give Byrne some interiority; chapter 5 is new
```

The skill snapshots the manuscript, works out which scenes were added, modified, or removed since the last review in the same lineage (the last craft delta or `/critic:manuscript-craft`), and gives reviewers the full manuscript for context with those scenes as the target. The note goes in as `=== AUTHOR'S NOTE FOR THIS REVIEW ===`. The report says what you set out to do, whether it landed, whether the new material is as good as the best of the existing manuscript, and what it cost elsewhere, then issues with IDs and a verdict on the changes.

Non-interactive. This is the review to run often.

If the note cites an issue ID (`trying to answer ISSUE-003-02`), that issue block is pulled from the prior review and shown to reviewers so they can judge whether the change answers it. Nothing else from the prior review is carried.

### Option B: chapter review on what's new

```
/critic:review chapter 5
```

Same shape as the manuscript reviews, scoped to one chapter. Four-role review (analytical, immersive, structural, adversarial), cross-review pairs, synthesis. Saves to `Review/NNN-chapter-5-review-...`. Uses the `frame` setting.

Right when one chapter deserves four sets of eyes on its own.

### Option C: manuscript review with author note

```
/critic:manuscript-craft tightened Act 1; added the chapel scene to give Byrne some interiority; chapter 5 is new
```

The whole-book read. It loads the prior craft-frame review as context, snapshots and diffs, and opens with the same four questions the delta asks before reading the foundations, pacing, character work, threads, tone, and prose as a whole.

Heaviest of the three. Once an act, or when the changes are spread across the book.

## After significantly editing a chapter

You restructured chapter 3. Moved a scene, cut a beat, changed Henry's response. Worth checking what breaks downstream.

```
/critic:downstream chapter 3
```

Reads chapter 3 plus every scene after it. Looks for continuity breaks, invalidated setups, dialogue references that no longer work, timeline issues, canon drift. Reports issues grouped by affected scene in manuscript order, critical first.

Nothing is saved. The orchestrator discusses with you. Spawn `/critic:review` on affected scenes if any of the findings are significant.

## Polishing a chapter

You've finished structural work on a chapter and want a copy-edit / line-edit pass before declaring it done.

```
/critic:close-read chapter 4
```

One subagent per scene. Each one reads only that scene. Looks for typos, prose-level issues, micro-structure, canon adherence, style violations. Output is quote-and-fix, grouped by category, ordered by appearance.

Per-scene reports under `Review/close-read/<run-id>/`, plus an `index.md`.

For the whole manuscript before external submission:

```
/critic:close-read all
```

Subagents per scene, in waves of 8. Takes longer. Produces 30 to 50 per-scene reports.

## Keeping the Codex honest

Prose introduces new characters and locations. The Codex needs to keep up.

```
/critic:extract chapter 4
```

Walks every entity in chapter 4. For each: lists new facts asserted, confirmed facts, contradictions with existing Codex/Research. Flags implicit-entity candidates (proper-noun phrases the chapter uses that aren't in the Codex roster yet). Proposes inventory updates and Codex edits.

Every change requires approval. Walk through with the orchestrator: accept, edit, or skip per proposal. Applied changes go to `Background/Characters/<Name>.md` or `Background/Locations/<Name>.md` (created or modified) and `.claude/codex-inventory.md` (tracking row added or updated).

If a contradiction comes up (the prose says Luma has brown eyes; the Codex says gray), the skill makes you resolve it before the Codex write proceeds.

For a consolidated entry for one character whose details are scattered:

```
/critic:extract entity Henry Nelson
```

The server finds every scene that mentions Henry. The subagent walks them in manuscript order, builds a consolidated entry, and proposes it. Useful when you've drafted enough scenes that you want a refreshed character entry that reflects everything the manuscript has actually established.

## A focused outside opinion

You're stuck on a question. Not big enough for a review. Just want a second voice.

```
/critic:consult Is the trust fund a real threat or just lampshading?
/critic:consult The Customs scene runs 1200 words. Is it carrying its weight?
```

Short focused calls to Codex and Pi. Returns both responses labeled by source. The orchestrating Claude (you, in the cowork session) adds its own take.

No saved file. Conversational.

## A typical revision cycle

In rough order:

1. Draft new scenes or revise existing ones.
2. `/critic:downstream` on the chapters you edited, to catch immediate breakage.
3. `/critic:extract` on chapters with new entities, to keep the Codex current.
4. `/critic:delta` with a note describing what you tried to do.
5. For each flagged issue, `/critic:rebuttal` or `/critic:assess`, or just fix it, or leave it.
6. `/critic:close-read` on the chapters you've stabilised, to polish.
7. Move on to the next round of drafting.
8. Once an act, `/critic:manuscript-craft` for the whole-book read.

Rebuttals persist. Deferrals persist. Issue IDs are stable. In the publication frame reviews track what's been addressed; in the craft frame they don't, and what you decide to leave alone stays yours.

## What not to do

Don't run a manuscript review after every small edit. It's expensive and the whole-book sections won't move. `/critic:delta` is built for small changes; the manuscript reviews are for the whole.

Don't ignore the verdict labels. "Worth fixing" is different from "Needs rethinking". Reviewers calibrate them. If you keep getting "Needs rethinking" you're either ignoring foundational issues or your `stage.md` isn't telling reviewers what they need to know.

Don't expect the craft frame to be softer. It removes the market and the ledger, not the rejection pass, the frank reader, or the cross-review. If it reads softer, check that `stage.md` isn't asking for less than you want.

Don't hand-edit `.claude/codex-inventory.md`. Let `/critic:extract` reconcile. If you must hand-edit, only update notes. Re-running extract on a slice that includes the row will see the change.

Don't conflate `/critic:close-read` and `/critic:review`. Close-read is the typo and prose pass. Review is the structural and narrative pass. The framings are opposite.

Don't expect the synthesis to be right. It's a useful aggregation. It's not gospel. The raw reviewer outputs are below the sentinel in the same file if you want to check what individual reviewers actually said.

## Configuration tweaks worth knowing

Switch frames: `/critic:settings frame craft` or `frame publication`. Affects `/critic:review`, `/critic:delta`, `/critic:downstream`, and `/critic:consult`. The manuscript skills are named by frame, so both are always available.

Per-project prompt override: drop a file at `<vault>/prompts/<name>.md` and the server picks it up ahead of the embedded default. Useful if your project's reviewers should have a different register (e.g. literary fiction vs. genre). Check the other frame's prompts first; switching frames is cheaper than maintaining an override.

Disable a reviewer: `/critic:settings codex_enabled false` if you don't want Codex in the pool. Useful if the Codex CLI isn't auth'd or you don't want to pay for it on every review.

Change the Pi provider mid-project: `/critic:settings pi_provider anthropic` switches the Pi side of the pipeline. Old reviews are unaffected. New reviews use the new provider.
