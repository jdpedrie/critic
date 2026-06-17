# Prompts

Every prompt template the critic ships with. All live in `critic/server/prompts/*.md`, embedded into the binary via `//go:embed`.

## Resolution

When a skill calls `get-prompt(name)`, the server resolves in this order:

1. `<vault>/prompts/<name>.md` (per-project author override)
2. `$CLAUDE_PLUGIN_ROOT/prompts/<name>.md` (plugin-directory override)
3. The embedded default

To customise a prompt for one project, copy it from `critic/server/prompts/` to `<vault>/prompts/` and edit. The server picks up the change on the next tool call.

## Template variables

Templates use Go `text/template` syntax. Variables are passed by skills as a JSON object via the `vars` parameter on `get-prompt`.

| File | Variables |
|------|-----------|
| `review-base.md` | `{{.Role}}`, `{{.MaxIssues}}` |
| `cross-review.md` | `{{.MaxNewIssues}}` |
| `synthesis.md` | `{{.ReviewNum}}` (zero-padded to 3 digits) |
| All others | None |

## The catalog

### agent-framing.md

Used by all reviewers. The publishing-consultant framing. "You're advising a literary agent who has decided to represent this author. Surface issues are out of scope. Focus on structural integrity, narrative momentum, character work, voice, premise delivery." Explicitly says: don't hunt for typos. The agent already decided to represent the author.

Goes into the system prompt first. Sets the role and the disposition.

### manuscript.md

The manuscript-review instructions. What the reviewer is being asked to assess (foundations, momentum, premise delivery, voice), how to organise their report (verdict, biggest risk, biggest strength, strongest passages, foundation test), what to weight against the draft stage.

Used by `/critic:manuscript`. Goes after `agent-framing.md` in the system prompt.

### verdict.md

The verdict structure every primary reviewer must emit at the end of their review. The verdict label itself is one of: On track, Worth fixing, Needs rethinking, Not working. The biggest risk in the foundations is one specific issue, visible in what's been written. The biggest strength so far is one specific thing to protect during revision. Strongest passages are three quoted sentences that are working, with explanation. The foundation test predicts: if the author keeps building at this level, what's the most likely outcome?

Verdicts are calibrated against the stage block, not against a hypothetical finished book. The four labels are explicit and the prompt explains each.

Used by `/critic:manuscript` and `/critic:review`. Goes last in the system prompt.

### review-base.md

The four-role review prompt skeleton, with variables for role and max-issues. Used by `/critic:review`.

```
get-prompt("review-base.md", vars: {Role: "analytical", MaxIssues: 7})
```

### review-analytical.md / review-immersive.md / review-structural.md / review-adversarial-role.md

The four role-specific overlays used with `review-base.md` by `/critic:review`. Each one stacks an explicit lens on top of the base prompt.

The analytical role is text-craft focused, work-on-the-page. No outside context.

The immersive role is reader-experience focused. Does the prose deliver the experience the genre promises?

The structural role is full-context structural. With access to Codex and Research, what does the slice's structural choice reveal about the book?

The adversarial role is full-context contrarian. Aggressively skeptical of what's on the page. Assumes nothing.

### cross-review.md

The rebuttal prompt used in the cross-review matrix. Each reviewer sees the others' reviews and is asked to agree where they're right, disagree where they're wrong, and surface new issues the others missed (capped at `{{.MaxNewIssues}}`).

The constructive variant. Used by all four cross-review roles. The adversary maintains its harsh stance through session continuity (its prior adversarial review is in its session context).

### synthesis.md

Used by the synthesis subagent. Instructions for producing the final ranked-issue report with stable `ISSUE-{{.ReviewNum}}-NN` IDs. Tells the synthesiser how to reconcile contradictions, attribute issues to reviewers, group related issues, rank by significance, and format each issue block.

### rejection-pass.md

The rejection pass. Claude's second turn on its own review. The instructions tell the subagent to be blunt: knock out issues that don't survive scrutiny, sharpen the ones that do, surface anything the constructive pass softened.

Used by `/critic:manuscript` as part of the Claude-subagent's combined review + rejection pass. The subagent gets its own review verbatim in context and is told to rebut itself.

### adversarial.md

The Pi adversary's system prompt. A different framing from the constructive reviewers. Assume the author is trying to flatter the reviewer. Assume the prose is hiding its problems. Assume "literary" is doing work it shouldn't have to do.

Used by `/critic:manuscript` as the adversary's system prompt. The adversary participates in cross-review as a fourth matrix member, keeping its harsh stance via session continuity.

### close-read.md

The line-editor and copy-editor framing. Explicit out-of-scope rules (no plot, no scene structure, no character arc; those belong to `/critic:manuscript`). Five categories: TYPO, PROSE, STRUCTURE (micro), CANON, STYLE. Voice-preservation rules and the when-in-doubt-describe-don't-draft principle. Output format (quote-and-fix, category-grouped, ordered by appearance).

Used by `/critic:close-read`. This is the opposite role from the publishing-consultant reviewers. The framing tells the subagent to flag surface issues that the other reviewers are told to ignore.

### extract-slice.md

Canon-extraction prompt for chapter and scene modes. Walks every entity in the slice. For each: lists new facts asserted (with quoted passages), confirmed facts, and contradictions with existing Codex/Research. Flags implicit-entity candidates (proper-noun phrases not in the roster). Produces inventory deltas and recommended Codex edits.

Used by `/critic:extract chapter <N>` and `/critic:extract scene <file>`.

### extract-entity.md

Canon-extraction prompt for entity mode. Walks every scene that mentions one named entity. Identifies what the manuscript has established about them across the book in narrative order. Distinguishes new from confirmed from contradicted facts. Drafts a consolidated Codex entry plus an inventory update.

Used by `/critic:extract entity <name>`.

### downstream.md

The downstream-assessment prompt. The first slice in the user prompt is the edited slice (chapter or scene). Everything after is the downstream text that might be affected. The reviewer looks for continuity breaks, invalidated setups, character-state errors, dialogue references, timeline issues, and canon drift. Output is organised by affected scene in manuscript order, critical first.

Used by `/critic:downstream`.

## How prompts compose

The skills compose prompts by concatenation. A typical manuscript-review system prompt is:

```
get-prompt("agent-framing.md", vault: <vault>)
+ get-prompt("manuscript.md", vault: <vault>)
+ get-prompt("verdict.md", vault: <vault>)
```

A typical chapter-review system prompt for the analytical role is:

```
get-prompt("agent-framing.md", vault: <vault>)
+ get-prompt("review-base.md", vault: <vault>, vars: {Role: "analytical", MaxIssues: 7})
+ get-prompt("review-analytical.md", vault: <vault>)
+ get-prompt("verdict.md", vault: <vault>)
```

The close-read system prompt is just:

```
get-prompt("close-read.md", vault: <vault>)
```

A single composed file, because it's a self-contained role with its own framing.

The cross-review system prompt is just `cross-review.md` (with the `MaxNewIssues` var). The previous review goes in the user prompt. The system prompt only carries the cross-review instructions.

## Override patterns

A few useful per-project overrides.

The `stage.md` at the vault root overrides the auto-derived stage block. Not a prompt template per se. Loaded by `read-stage` directly. Use this when the auto-derived block is too generic for your project's actual situation.

An override at `<vault>/prompts/agent-framing.md` gives reviewers a different role. If you want reviewers to act as line editors instead of publishing consultants, this is the lever (though `/critic:close-read` already exists for that role).

An override at `<vault>/prompts/verdict.md` lets you use different verdict labels or extra fields. If you want a fifth label or a different rubric, change it here.

An override at `<vault>/prompts/synthesis.md` lets you use a different issue ID format, different ranking criteria, or different report structure.

The resolution order means overrides win silently. If a reviewer suddenly behaves oddly, check the override directory.

## What's not here

`review-base.md` is a template skeleton, not a standalone prompt. It expects to be composed with a `review-<role>.md`.

There's no separate "scene review" prompt. Scene reviews use `review-base.md` plus a role overlay just like chapter reviews. The scope is set by what's in the user prompt, not the system prompt.

The orchestrating Claude inside cowork doesn't use a critic prompt. Its instructions are in `SKILL.md` files.
