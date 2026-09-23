# Vault layout

What the critic reads, where it writes, and what each file means.

## The vault is a book folder

The `vault_path` setting points at a book project: a folder with a `Story/` subfolder in it. That's the only hard requirement. Everything else is optional and picked up when present.

```
<vault_path>/
  <Title>.md         book note (optional)
  Story/             one .md per chapter
  Background/
    Characters/      one .md per character (the Codex)
    Locations/       one .md per location (the Codex)
    *.md             worldbuilding docs, style guide, timeline (the Research)
  stage.md           optional author override for the derived stage block
  style.md           optional style guide (Background/style.md picked up as fallback)
  issues.md          deferred issues (managed by /critic:rebuttal)
  Review/            saved reviews and snapshots
  Review/.snapshots/ manuscript snapshots for diffing
  Review/.staging/   temporary review-part staging (cleaned up after assemble)
  Review/close-read/ per-scene close-read reports
  summary/           per-chapter summaries
  prompts/           optional per-project prompt overrides
  notes/             /critic:assess deep-dive notes
  .claude/codex-inventory.md    Codex tracking, maintained by /critic:extract
```

The critic never writes into `Story/` or `Background/` on its own. Those are the author's. When `/critic:extract` proposes a new `Background/Characters/<Name>.md`, the author approves it first and the orchestrator writes it with its own `Write` tool, not through the MCP server.

## Book note

A root-level `.md` file with `type: book` in its frontmatter carries the book's metadata. It's optional. Without one, the title is the vault folder's name and chapters print without act labels.

| Field | What we use it for |
|-------|--------------------|
| `type: book` | Discovery. We scan the vault root for it. Two book notes is an error. |
| `title` | Manuscript and snapshot heading: `# Noblesse Oblige`. |
| `acts` | Act number to label: `1: The Rim` prints `## Act 1: The Rim`. |

The note's body is the book description. It goes into the derived stage block when there's no `stage.md`.

## Story

`Story/*.md` is the manuscript. One file per chapter. The file name is for the author; the critic orders chapters by number.

A chapter file looks like this:

```markdown
---
chapter: 4
title: Arcadia
act: 1
status: draft
pov: "[[Luma]]"
characters:
  - "[[Luma]]"
  - "[[Henry Nelson]]"
locations:
  - "[[Noblesse Oblige]]"
scenes:
  - title: Old Wounds, New Quiet
    pov: "[[Luma]]"
    location: "[[Noblesse Oblige]]"
    characters: ["[[Henry Nelson]]", "[[Luma]]"]
    conflict: Henry apologizes. He misses Florence.
  - title: Departure for Arcadia
    pov: "[[Luma]]"
---

## Old Wounds, New Quiet

> *The chapter epigraph sits at the top of the first scene.*

Prose.

## Departure for Arcadia

Prose.
```

The chapter number comes from `chapter:`, or from the leading number in the file name when the field is missing. A file with neither is a note, not a chapter, and the critic skips it. Numbers are global across acts. Two files claiming the same number is an error: keeping either one would silently drop the other from every review.

Each `## ` heading starts a scene. Exactly two hashes. `###` and deeper stay inside the prose, and so do `---` rules. Text above the first scene heading belongs to the chapter itself. A chapter with no scene headings is one untitled scene.

Scenes are addressed `CC-SS`: chapter, then position in the chapter. `04-02` is the second scene of chapter 4. These are the same prefixes the old one-file-per-scene layout used, so every review and note that cites `04-02` still resolves. `read-scene` also takes `4.2`, `4-2`, a trailing title (`04-02 Departure for Arcadia`), or an exact scene title.

The `scenes:` list carries per-scene metadata: `title`, `pov`, `location`, `characters`, and anything else the author wants to keep (`conflict`, `emotion`, `intensity`, `status`, `notes`). The critic reads `pov`, `location`, and `characters` to prefilter Codex entries. Entries pair with body sections by title, then by position, so moving a scene in the body keeps its cast attached.

Chapter-level `pov`, `characters`, and `locations` are the union for the chapter. Chapter reviews prefilter Codex entries with them.

The critic strips wikilinks from prose before reviewers see it. `[[Henry Nelson]]` becomes `Henry Nelson`. `[[Path/To/Note|Display]]` becomes `Display`. Word counts are computed from the prose on every call; nothing stores them.

## Codex

`Background/Characters/<Name>.md` and `Background/Locations/<Name>.md` are per-entity reference files. The file name without `.md` is the canonical entity name. That's what chapter frontmatter wikilinks resolve to.

The critic doesn't enforce a schema on Codex entries. Whatever frontmatter and body the author chooses, the reviewer sees verbatim.

The Codex is inlined for slice-scoped work only. `/critic:review` and `/critic:close-read` filter to the entities named in the slice's metadata: reviewing scene `01-01` pulls only the four characters and one location its `scenes:` entry names. `/critic:extract` reads the full Codex because it reasons about contradictions across entities. The manuscript skills and `/critic:delta` inline no Codex at all; manuscript reviewers read as readers, without insider reference material.

Claude subagents also get `read-codex-entry(name)` for on-demand lookups. Codex CLI and Pi don't. They get whatever the orchestrator inlines.

## Research

Every other `.md` file under `Background/` is the Research: free-form worldbuilding. The critic concatenates them (recursively, skipping `Characters/` and `Locations/`) and inlines them for slice reviews, close-reads (canon category), and extraction. The manuscript skills and `/critic:delta` exclude them. Whole-book reviewers judge what's on the page, and a worldbuilding bible both bloats their context and lets them paper over gaps a reader would hit.

The critic doesn't parse Research structurally. It's prose. Non-markdown files (a `.mermaid` diagram, an `.html` map) are ignored.

## stage.md (optional)

A hand-written stage block at `<vault>/stage.md` describes where the manuscript is in the drafting process and what the book is trying to be:

```markdown
# Current Stage

This is approximately 3/4 of the first act of a planned three-act novel.
Target final length: ~120,000 words. Current: ~30,000 words.

## What this project is

A book I write for my own satisfaction, when I feel like it. No deadline,
no submission target. Judge the pages, not the pace.

## What I want this book to be

- A slow-burn mystery where the strangeness is felt before it's explained
- A relationship under power asymmetry that neither party fully sees
- Prose that stays close and physical; no narrator editorialising

## What this draft is currently trying to do

- Establish the central mystery
- Develop the Henry/Luma relationship
- Set up the financial pressure

## Not yet attempted, by design

- Resolution of any major thread
- Mid-book reversals
- Character arcs completing
```

If `stage.md` exists, the critic injects it verbatim. If not, the server derives a stage block from the book note and the chapter files: chapters, scenes, word counts, status. The derived block is honest about size and says nothing about intent, which is why a hand-written one is better.

The stage block always goes first in the user prompt because it calibrates everything else. Reviewers are told it's authoritative. In the craft frame the "what this project is" and "what I want this book to be" sections are what the reviewers measure the pages against, and a line like "judge the pages, not the pace" is honoured.

## style.md (optional)

`<vault>/style.md` is the project's prose-discipline guide. Register, tense, POV, punctuation, anything the author wants reviewers to enforce. If it lives at `Background/style.md` instead, `read-style-guide` falls back to that.

The style block goes after the stage block in the user prompt, marked `=== STYLE GUIDE ===`.

If the style guide invites market framing (comparable titles, acquisition-editor reactions), publication-frame reviewers take it up and craft-frame reviewers are told to disregard the invitation.

## issues.md (optional)

`<vault>/issues.md` is the deferred-issue log. The author defers an issue via `/critic:rebuttal <issue-id>` (option: defer). The skill appends to this file under a heading the author chooses (e.g. `## Chapter 5`).

Future reviewers see this file under `=== KNOWN ISSUES ===` with instructions: these issues have been acknowledged but deferred; do not re-raise them unless the issue has materially escalated.

In the craft frame every unaddressed prior issue is treated this way whether or not it's in `issues.md`. The file still earns its keep because a deferral with a reason tells reviewers what the author decided, not just that they decided.

## Review/

Saved reviews and snapshots. The critic owns this folder.

```
Review/
  NNN-prefix-YYYY-MM-DD-HHMMSS.md   saved reviews, global NNN counter
  .snapshots/
    <lineage>-YYYY-MM-DD-HHMMSS.md     assembled manuscript
    <lineage>-YYYY-MM-DD-HHMMSS.json   which review took it: lineage, kind, review number, time
    <lineage>-YYYY-MM-DD-HHMMSS.diff   unified diff against the prior snapshot in the lineage
  .staging/
    <part-name>                     temporary; cleaned up after assemble-review
  close-read/
    <run-id>/
      index.md                      links + one-line summaries per scene
      <CC-SS>-<slug>.md             one report per scene
```

Snapshots are how diff-aware reviews work. Each `/critic:delta`, `/critic:manuscript-craft`, or `/critic:manuscript-publication` run assembles the manuscript from `Story/`, writes it as a snapshot, and diffs against the prior one in the same lineage.

A snapshot belongs to a lineage, named in its file name and in its `.json` sidecar. The sidecar also records the review kind and number the snapshot was taken for. There are two lineages: `craft` (written by `/critic:manuscript-craft` and craft-frame deltas) and `publication` (written by `/critic:manuscript-publication` and publication-frame deltas). "Previous snapshot" means the newest in the same lineage, so a craft review diffs against the last craft review and never against a publication one, and the reverse. Switching frames starts a fresh baseline: the first review in a lineage has nothing to diff against and says so. Snapshots from before lineages existed are named `manuscript-*` and count as publication, since that's the review that took them.

A prior snapshot recorded for the same or a later review number is the orphan of a run that died before its review was assembled (the number was then reused). It's skipped, so the diff still covers everything since the last review that landed. `list-snapshots` shows a lineage's history.

The server also compares the two snapshots scene by scene: the manuscript reviews show reviewers the list of added, modified, and removed scenes as `=== CHANGES SINCE LAST REVIEW ===`, and the delta review shows them the full text of the added and modified scenes as `=== CHANGED MATERIAL ===`.

Review file prefixes: `delta-craft`, `delta-publication`, `manuscript-craft`, and `manuscript-critic` (the publication frame, kept so older reviews sort with new ones), plus `chapter-<N>-review` and `scene-<id>-<slug>-review` from `/critic:review`. Prior-review context follows the same grouping: a craft review loads only craft-lineage reviews, a publication review only publication-lineage ones.

The assembled shape is `# Title`, `## Act N: <label>`, `### Chapter N: <title>`, `#### <scene title>`, prose. It's the shape the storyline plugin exported, kept on purpose: snapshots taken before the move to chapter files diff cleanly against snapshots taken after, and the diff shows prose changes, not a reformat.

## summary/

`/critic:summarize` writes one summary per chapter to `summary/chapter-<NN>.md` (chapter number zero-padded). These are reference documents for the author. The critic itself doesn't consume them; reviews read the chapter files directly.

## .claude/codex-inventory.md

Maintained by `/critic:extract`. Tracks which Codex entries exist, which are stubs, which are missing, and which are intentionally absent. Sectioned by kind (Characters, Locations, Concepts, Other), with per-entity blocks:

```markdown
### Mark Andersen: stub
- Last touched: 03-01 (The Bar)
- Pending facts:
  - Master of the brig Harrow; hires Henry to escort his convoy
- Notes: flagged 2026-06-12
```

Status values: `present`, `stub`, `missing`, `intentionally-absent`. `intentionally-absent` means the author has decided not to write a Codex entry for this entity. Extract is told not to re-suggest one.

## prompts/ (optional)

Per-project prompt overrides. Drop a file at `<vault>/prompts/<name>.md` and the server picks it up ahead of the embedded default. Useful for tuning reviewer framing or output format per project.

See [prompts.md](prompts.md) for the full template catalog.

## notes/ (optional)

`/critic:assess` writes deep-dive notes here when the author chooses to save the outcome of a conversation. One file per issue: `notes/ISSUE-003-04.md`.
