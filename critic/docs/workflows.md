# Workflows

Example sessions, in roughly the order an author would hit them on a new project.

## Day 1: set up a vault

You have an obsidian-storyline project at `/Users/me/obsidian/Vault/MyNovel/` with a `MyNovel.md` project file and a `Scenes/` folder with a few drafted scenes. Codex and Research may be sparse or empty. That's fine for now.

```
/critic:settings vault_path /Users/me/obsidian/Vault/MyNovel
/critic:settings pi_provider google
/critic:settings pi_model gemini-2.5-pro
```

Optional but recommended: write a `stage.md` at the project root describing where the manuscript is. The auto-derived block works, but a hand-written one is more honest. Two paragraphs are enough:

```markdown
# Current Stage

This is the first 30,000 words of a planned 120,000-word novel. Three-act structure.

What this draft is currently trying to do:
- Establish the central mystery in Act 1
- Develop the protagonist's relationship with their attendant
- Set up the financial pressure that drives Act 2

NOT yet attempted, by design:
- Resolution of any major thread
- Mid-book reversals
- Character arcs completing

Looking for feedback on:
- Whether the foundations support what's coming
- Issues that will compound if not addressed now
```

This block goes first in every reviewer's prompt. Without it, reviewers will judge the manuscript as a finished book and tell you things you already know ("the arc doesn't resolve") instead of what you need to know ("the foundation Henry's arc would rest on isn't there yet").

## The first manuscript review

```
/critic:manuscript
```

Interactive. The skill asks which reviewers (default: all three), which Pi model (the skill runs `pi --list-models` and shows you), which adversary model (usually pick a different provider than the primary Pi for different bias and more useful pushback), and which steps to run (default: everything on).

Then it runs straight through. Roughly 5 to 10 minutes for a 30,000-word manuscript with all reviewers and the cross-review matrix. The output is one review file at `review/001-manuscript-critic-YYYY-MM-DD-HHMMSS.md`.

Read the synthesis. Each issue has a stable ID like `ISSUE-001-04`. Decide what to do with each.

## Responding to a review

For each issue you don't immediately fix, run `/critic:rebuttal <issue-id>`. Three outcomes.

Rebut: "Intentional. The ambiguity resolves in chapter 7." The rebuttal is added as an Obsidian callout inline in the review file. Future reviewers see it and respect it.

Defer: "Acknowledged, will tighten in revision." The issue goes to `issues.md`. Future reviewers see it but are told not to re-raise unless it has materially escalated.

Accept: you'll fix it. No tool calls.

If you want to dig into an issue before deciding:

```
/critic:assess ISSUE-001-04
/critic:assess 001-04 Is Henry actually passive in the chapter, or only in this scene?
```

The orchestrator pulls only the slices it needs and discusses with you. Conversational. You can save the outcome to `notes/ISSUE-001-04.md` for future reference.

## After drafting more chapters

Now you have a few new chapters drafted. Two options.

### Option A: manuscript review with author note

```
/critic:manuscript tightened Act 1 per the prior review; added the chapel scene to give Byrne some interiority; chapter 5 is new
```

The skill loads the prior review's synthesis (so reviewers see what's been flagged before, plus any rebuttals). It snapshots the manuscript and diffs against the previous snapshot. It inlines the author note as `=== AUTHOR'S NOTE FOR THIS REVIEW ===` and asks reviewers to assess whether the intent was achieved. Then it runs the full pipeline.

Heavier than a chapter review, but right when changes are spread across the book.

### Option B: chapter review on what's new

```
/critic:review chapter 5
```

Same shape as manuscript, scoped to one chapter. Four-role review (analytical, immersive, structural, adversarial), cross-review pairs, synthesis. Saves to `review/NNN-chapter-5-review-...`.

Lighter than the manuscript review, focused on the chapter that changed.

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

Per-scene reports under `review/close-read/<run-id>/`, plus an `index.md`.

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

Every change requires approval. Walk through with the orchestrator: accept, edit, or skip per proposal. Applied changes go to `Codex/Characters/<Name>.md` or `Codex/Locations/<Name>.md` (created or modified) and `.claude/codex-inventory.md` (tracking row added or updated).

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
4. `/critic:manuscript` with an author note describing what you tried to do. Maybe weekly or monthly, not daily.
5. For each flagged issue, `/critic:rebuttal` or `/critic:assess`, or just fix it.
6. `/critic:close-read` on the chapters you've stabilised, to polish.
7. Move on to the next round of drafting.

The system is designed to compound. Rebuttals persist. Deferrals persist. Issue IDs are stable. Reviews don't re-litigate what you've already decided unless something has materially changed.

## What not to do

Don't run `/critic:manuscript` after every small edit. It's expensive and the diff against a tiny change isn't informative. Run when you have a chunk of meaningful change to assess.

Don't ignore the verdict labels. "Worth fixing" is different from "Needs rethinking". Reviewers calibrate them. If you keep getting "Needs rethinking" you're either ignoring foundational issues or your `stage.md` isn't telling reviewers what they need to know.

Don't hand-edit `.claude/codex-inventory.md`. Let `/critic:extract` reconcile. If you must hand-edit, only update notes. Re-running extract on a slice that includes the row will see the change.

Don't conflate `/critic:close-read` and `/critic:review`. Close-read is the typo and prose pass. Review is the structural and narrative pass. The framings are opposite.

Don't expect the synthesis to be right. It's a useful aggregation. It's not gospel. The raw reviewer outputs are below the sentinel in the same file if you want to check what individual reviewers actually said.

## Configuration tweaks worth knowing

Per-project prompt override: drop a file at `<vault>/prompts/<name>.md` and the server picks it up ahead of the embedded default. Useful if your project's reviewers should have a different register (e.g. literary fiction vs. genre).

Disable a reviewer: `/critic:settings codex_enabled false` if you don't want Codex in the pool. Useful if the Codex CLI isn't auth'd or you don't want to pay for it on every review.

Change the Pi provider mid-project: `/critic:settings pi_provider anthropic` switches the Pi side of the pipeline. Old reviews are unaffected. New reviews use the new provider.
