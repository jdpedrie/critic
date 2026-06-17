# Vault layout

What the critic reads, where it writes, and what each file means.

## The vault is a storyline project

The `vault_path` setting points at an [obsidian-storyline](https://github.com/PixeroJan/obsidian-storyline) project base folder. That's the folder containing the project's `<Title>.md` file (or the folder where it sits as a sibling, depending on storyline layout), plus the `Scenes/`, `Codex/`, `Research/` subfolders.

Concretely:

```
<vault_path>/
  <Title>.md         storyline project file
  Scenes/            one .md per scene
  Codex/
    Characters/      one .md per character
    Locations/       one .md per location
  Research/          worldbuilding bibles (free-form markdown)
  Exports/           storyline's own exports (we never write here)
  System/            storyline metadata
  Notes/             storyline corkboard sticky notes
  Archive/           storyline archived scenes
  ── critic adds: ──
  stage.md           optional author override for the auto-derived stage block
  style.md           optional style guide (Research/style.md picked up as fallback)
  issues.md          deferred issues (managed by /critic:rebuttal)
  review/            saved reviews and snapshots
  review/.snapshots/ manuscript snapshots for diffing
  review/.staging/   temporary review-part staging (cleaned up after assemble)
  review/close-read/ per-scene close-read reports
  summary/           per-chapter summaries
  prompts/           optional per-project prompt overrides
  notes/             /critic:assess deep-dive notes
  .claude/codex-inventory.md    Codex tracking, maintained by /critic:extract
```

The critic never writes into `Scenes/`, `Codex/`, `Research/`, `Exports/`, `System/`, `Notes/`, or `Archive/` directly. Those are storyline's territory. The critic writes proposals to disk after the author approves them. For example, `/critic:extract` may write a new `Codex/Characters/<Name>.md`, but the proposal-and-approval flow is explicit and the file content is written through the author's `Write` tool, not through the MCP server.

## Project file

`<Title>.md` carries the project metadata. The critic needs these frontmatter fields:

| Field | What we use it for |
|-------|--------------------|
| `type: storyline` | Discovery. We scan the vault root for the project file. |
| `title` | Display name and snapshot title. |
| `definedActs` | Auto-derived stage block. |
| `definedChapters` | Auto-derived stage block. |
| `actLabels` | Manuscript export: `## Act 1: The Rim`. |
| `chapterLabels` | Manuscript export: `### Chapter 1: Customs at Fontenoy`. |
| `actDescriptions` | Auto-derived stage block (optional). |
| `chapterDescriptions` | Auto-derived stage block (optional). |
| `language` | Hint for the reviewer (not currently load-bearing). |

The body of the project file is treated as the project description and folded into the auto-derived stage block.

## Scenes

`Scenes/*.md` files are the source of truth for prose. Filename is whatever the author named it. Sort order comes from frontmatter, not from filenames.

Required frontmatter: `type: scene`, `act` (integer), `chapter` (integer), `sequence` (integer). Scenes within a chapter sort by sequence.

Read but not required: `title` (defaults to filename if absent), `pov` (wikilink or plain name, wikilinks stripped), `characters` (list of wikilinks, stripped to plain names), `location` (wikilink or plain name), `wordcount` (used for the auto-derived stage block).

The `status` field is ignored. The critic doesn't filter or surface scene status.

The critic strips wikilinks in scene bodies the same way storyline's export does. `[[Henry Nelson]]` becomes `Henry Nelson`. `[[Path/To/Note|Display]]` becomes `Display`.

## Codex

`Codex/Characters/<Name>.md` and `Codex/Locations/<Name>.md` are per-entity reference files. The filename without `.md` is the canonical entity name. That's what scene frontmatter wikilinks resolve to.

The critic doesn't enforce a schema on Codex entries. Whatever frontmatter and body the author chooses, the reviewer sees verbatim.

Two read patterns. Full inlining is used by `/critic:manuscript`, which passes all Codex entries to the reviewers as context. Token cost is real but it's the whole-book pass; everyone might be relevant. Filtered inlining is used by `/critic:review` and `/critic:close-read`, which filter to the entities referenced in the slice's scene frontmatter. Reviewing scene `01-01 Customs at Fontenoy` pulls only the four characters and one location named in its frontmatter.

A third pattern exists for Claude subagents: `read-codex-entry(name)` for on-demand lookups. Codex CLI and Pi don't get this tool. They get whatever the orchestrator decides to inline.

## Research

`Research/*.md` is free-form worldbuilding. The critic concatenates every `.md` file under `Research/` (recursive) and inlines it for every reviewer that gets context, manuscript and slice reviews alike. It's the worldbuilding bible.

The critic doesn't try to parse Research files structurally. They're prose. If the author writes `worldbuilding.md` with sections for politics, technology, and history, the reviewer sees it as one chunk and uses it as context.

`Research/style.md` is also where the style guide can live, if the author hasn't put one at the project root. The `read-style-guide` tool checks the root first and falls back to `Research/style.md`.

## stage.md (optional)

A hand-written stage block at `<vault>/stage.md` describes where the manuscript is in the drafting process:

```markdown
# Current Stage

This is approximately 3/4 of the first act of a planned three-act novel.
Target final length: ~120,000 words. Current: ~30,000 words.

What this draft is currently trying to do:
- Establish the central mystery
- Develop the Henry/Luma relationship
- Set up the financial pressure

NOT yet attempted, by design:
- Resolution of any major thread
- Mid-book reversals
- Character arcs completing

Looking for feedback on:
- Whether the foundations support what's coming
- Issues that will compound if not addressed now
```

If `stage.md` exists, the critic injects it verbatim. If not, the server synthesises a stage block from the project frontmatter (acts defined, chapters with scenes, total wordcount) and uses that.

The stage block always goes first in the user prompt because it calibrates everything else.

## style.md (optional)

`<vault>/style.md` is the project's prose-discipline guide. Conventions for register, tense, POV, punctuation, named-character handling, anything the author wants reviewers to enforce. If the file lives at `Research/style.md` instead, `read-style-guide` falls back to that.

The style block goes after the stage block in the user prompt, marked `=== STYLE GUIDE ===`.

## issues.md (optional)

`<vault>/issues.md` is the deferred-issue log. The author defers an issue via `/critic:rebuttal <issue-id>` (option: defer). The skill appends to this file under a heading the author chooses (e.g. `## Chapter 5`).

Future reviewers see this file under `=== KNOWN ISSUES ===` with instructions: these issues have been acknowledged but deferred; do not re-raise them unless the issue has materially escalated.

## review/

Saved reviews and snapshots. The critic owns this folder.

```
review/
  NNN-prefix-YYYY-MM-DD-HHMMSS.md   saved reviews, global NNN counter
  .snapshots/
    manuscript-YYYY-MM-DD-HHMMSS.md   manuscript snapshot in storyline export format
    manuscript-YYYY-MM-DD-HHMMSS.diff   unified diff against the prior snapshot
  .staging/
    <part-name>                     temporary; cleaned up after assemble-review
  close-read/
    <run-id>/
      index.md                      links + one-line summaries per scene
      <act>-<chapter>-<seq>-<slug>.md   one report per scene
```

Snapshots are how diff-aware reviews work. Each `/critic:manuscript` run writes a fresh snapshot (assembled from `Scenes/` in plugin export format) and diffs against the prior one. The diff text is summarised and shown to reviewers as `=== CHANGES SINCE LAST REVIEW ===`, plus the full diff file's path is mentioned for the orchestrator's reference.

## summary/

`/critic:summarize` writes one summary per chapter to `summary/chapter-<NN>.md` (chapter number zero-padded). These are factual reference documents for the author. The critic itself doesn't consume them; manuscript and slice reviews read scenes directly.

## .claude/codex-inventory.md

Maintained by `/critic:extract`. Tracks which Codex entries exist, which are stubs, which are missing, and which are intentionally absent. Sectioned by kind (Characters, Locations, Concepts, Other), with per-entity blocks:

```markdown
### Mark Andersen: stub
- Last touched: Act 1, Ch 3, Seq 1 (Hiring Sam Perry)
- Pending facts:
  - Bay Area private client, employs Henry on a recovery contract
- Notes: flagged 2026-06-12
```

Status values: `present`, `stub`, `missing`, `intentionally-absent`. The `intentionally-absent` flag means the author has decided not to write a Codex entry for this entity. Extract is told not to re-suggest creating one.

## prompts/ (optional)

Per-project prompt overrides. Drop a file at `<vault>/prompts/<name>.md` and the server picks it up ahead of the embedded default. Useful for tuning reviewer framing or output format per project.

See [prompts.md](prompts.md) for the full template catalog.

## notes/ (optional)

`/critic:assess` writes deep-dive notes here when the author chooses to save the outcome of a conversation. One file per issue: `notes/ISSUE-003-04.md`.

## What we don't read

For completeness, the storyline folders the critic explicitly ignores:

- `Exports/` is storyline's own exports. The critic assembles its own snapshots; we don't read or write here.
- `Notes/` is corkboard sticky notes. Not prose.
- `Archive/` is archived scenes. Not in the active manuscript.
- `System/` is storyline metadata. Doesn't carry author intent.
- `SceneNotes/` is per-scene external note files. Not prose.

If the author wants any of this content available to reviewers, the path is to either inline it manually (paste into `Research/`) or write a per-project prompt override that opens the file directly.
