# Goals

What the critic plugin is for, and what it deliberately isn't.

## The problem

Alignment-trained models flatter prose. Ask Claude or GPT for feedback on a draft chapter and you get a constructive sandwich: nice things, gentle suggestions, an upbeat close. Useful for ego, useless for revision. The author who wants to publish needs the read a literary agent or a senior editor gives. "This character is passive." "The opening burns 3,000 words before the inciting incident." "The prose hits register A here and register B two paragraphs later, pick one."

A second problem: any one model has its blind spots. Claude likes interior monologue. GPT likes neat scenes that resolve. Each has a default register it nudges prose toward. One reviewer is one bias.

A third problem: feedback that doesn't carry across reviews compounds. If the author addresses an issue and the next review re-raises it with no memory of the rebuttal, the feedback loop fights itself.

## What we built

A multi-reviewer system with explicit anti-flattery framing, persistent issue IDs that survive revisions, and a pipeline that scopes the right amount of context to each task. Three discrete tiers of review.

The manuscript review is the full-book pass. Three reviewers run in parallel (Claude, Codex, Pi), plus an optional Pi adversary, then a cross-review matrix where each reviewer rebuts the others, then a synthesis pass that produces ranked issues with stable IDs.

The chapter or scene review has the same shape, narrower scope. Four role lenses (analytical, immersive, structural, adversarial) run on one chapter or one scene.

The close-read is the line-editor and copy-editor pass. Quote-and-fix output for typos, prose-level issues, micro-structure, canon adherence, style. One subagent per scene so attention stays narrow.

The supporting skills cover the rest of the cycle. Extract reconciles prose against the Codex (per-character, per-location files) and Research bibles, maintaining a tracking inventory. Every change requires approval. Downstream assesses what breaks in later scenes after an edit. Assess is a conversational deep-dive on one flagged issue. Rebuttal records the author's response to a flagged issue. Consult gets a short focused second opinion from Codex and Pi on a narrow question. Summarize generates per-chapter reference summaries.

## What it's optimising for

In rough priority order:

1. Honest feedback over polite feedback. The reviewer framing is "publishing consultant advising the agent who already represents the author", not "encouraging friend". The rejection pass and adversarial pass exist to counter alignment bias.

2. Reproducible issue tracking. Every issue gets a stable ID (`ISSUE-NNN-MM`) that points at a specific review file and section. The author can rebut it (Obsidian callout, future reviewers see and respect), defer it (`issues.md`, future reviewers see but only re-raise if it's escalated), or accept and fix.

3. Calibrated to the actual draft stage. Reviewers default to assuming they're reading a finished book. That produces useless verdicts like "the arc doesn't resolve" when the author has only drafted Act 1. The critic injects an explicit stage block, either hand-written (`stage.md`) or auto-derived from project frontmatter, so reviewers know what they're seeing.

4. Scoped context per task. The full-manuscript review inlines everything. A chapter review inlines only the Codex entries for entities in that chapter. A close-read on a scene inlines only that scene's frontmatter entities plus the style guide. Reviewers don't get to wander.

5. Independent reviewers, then forced engagement. The three primary reviewers run in parallel with no knowledge of each other. Then the cross-review matrix forces each one to rebut the others. This surfaces real disagreements (which the synthesis flags) and stops one reviewer's framing from dominating.

## What it deliberately isn't

It is not a generative tool. The critic doesn't draft scenes, doesn't suggest plot fixes, doesn't write prose. Suggestions in close-read are tightly constrained to copy-editor scope. The author writes the book.

It is not an automated revision system. Every Codex write, every inventory update, every persistent change requires explicit author approval. Reviews land on disk because that's data; everything else is a proposal.

It is not a workflow imposer. The author picks which reviewers run on a manuscript pass. The author decides whether to rebut, defer, or accept each issue. The critic produces information; the author decides what to do with it.

It is not a typo hunter, except where it explicitly is. The publishing-consultant reviewers are told not to flag typos. That's `/critic:close-read`'s job and it's a different role with a different prompt. Don't conflate them.

## The bet

The bet is that an author with this tooling produces better revisions per draft than the same author working with a single chat reviewer or with their own re-reads. Better in the sense that compounds across drafts. Rebut once, the issue stops coming back. Defer once, future reviewers know not to re-raise unless it escalates. Let the system build a stable picture of what's working and what isn't so the author can spend their time fixing instead of arguing with the tool.
