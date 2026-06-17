---
name: help
description: Show information about the critic plugin. Available skills, configuration, prompt overrides.
disable-model-invocation: true
---

# Critic Plugin Help

Present the following information to the user in a clear, readable format.

## Skills

| Skill | Description |
|-------|-------------|
| `/critic:manuscript [author note]` | Full manuscript review with interactive setup. Claude (subagent, with optional rejection pass), Codex, Pi, optional adversarial pass, cross-review matrix, synthesis, save. |
| `/critic:review chapter <N>` / `scene <file>` | Four-role review (analytical, immersive, structural, adversarial) scoped to one chapter or one scene. |
| `/critic:close-read scene <file>` / `chapter <N>` / `all` | Line-editor / copy-editor pass. Subagent per scene; quote-and-fix output covering typos, prose, micro-structure, canon adherence, style. Saves per-scene reports under `review/close-read/<run-id>/`. |
| `/critic:downstream chapter <N>` / `scene <file>` | Assess what breaks in scenes after the edit point. |
| `/critic:extract chapter <N>` / `scene <file>` / `entity <name>` | Reconcile prose against the Codex + Research; maintains `.claude/codex-inventory.md`. Every change requires approval. |
| `/critic:summarize` | Generate a per-chapter summary for every chapter into `summary/`. |
| `/critic:consult <question>` | Quick second opinions from Codex and Pi on a focused question. |
| `/critic:rebuttal <issue-id>` | Rebut, defer, or accept a review issue. Conversational. |
| `/critic:assess <issue-id> [question]` | Deep-dive investigation of one issue. Pulls only the slices it needs. Conversational. |
| `/critic:settings` | View and update plugin settings. |
| `/critic:help` | This help text. |

## Configuration

### Settings

Run `/critic:settings` to view or change settings. Settings persist across sessions.

```
/critic:settings                             # view all
/critic:settings vault_path /path/to/vault   # set vault path
/critic:settings codex_enabled false         # disable a provider
/critic:settings pi_provider google
/critic:settings pi_model gemini-2.5-pro
```

All available settings:

| Setting | Description |
|---------|-------------|
| `vault_path` | Absolute path to your Obsidian vault |
| `codex_enabled` | Enable/disable Codex (true/false) |
| `codex_model` | Codex model (leave empty to let the Codex CLI pick) |
| `openai_api_key` | OpenAI API key (omit for Codex CLI login) |
| `pi_enabled` | Enable/disable Pi |
| `pi_provider` | Default Pi provider: anthropic, openai, google |
| `pi_model` | Default Pi model (skills may override per-call) |

Claude is always available (it runs as a subagent within this cowork session). There's nothing to configure for Claude. Its model is whatever cowork is running on.

Settings can also be set in `config.yaml` in the plugin directory. Settings from `/critic:settings` override `config.yaml`.

### Prompt Overrides

All reviewer prompts can be customized without rebuilding. Prompts are loaded from (in order):

1. `<vault>/prompts/<name>.md` (per-project override)
2. `<plugin>/prompts/<name>.md` (plugin-level default)
3. Compiled-in default (fallback built into the binary)

To customize a prompt, copy it from the plugin's `prompts/` directory to `<vault>/prompts/` and edit. The server picks up the change on the next tool call.

Available prompt files:

| File | Used by | Variables |
|------|---------|-----------|
| `agent-framing.md` | All reviewers | None |
| `verdict.md` | All reviewers | None |
| `review-base.md` | Chapter review | `{{.Role}}`, `{{.MaxIssues}}` |
| `review-analytical.md` | Analytical reader role | None |
| `review-immersive.md` | Immersive reader role | None |
| `review-structural.md` | Structural analyst role | None |
| `review-adversarial-role.md` | Adversarial critic role | None |
| `cross-review.md` | Cross-review pass | `{{.MaxNewIssues}}` |
| `synthesis.md` | Synthesis | `{{.ReviewNum}}` (zero-padded to 3 digits) |
| `manuscript.md` | Manuscript review | None |
| `rejection-pass.md` | Claude's rejection pass | None |
| `adversarial.md` | Adversarial/Grok rejection | None |
| `extract-slice.md` | Canon extraction. Chapter/scene mode | None |
| `extract-entity.md` | Canon extraction. Entity mode | None |
| `close-read.md` | Line-edit / copy-edit pass | None |
| `downstream.md` | Downstream assessment | None |

Variables use Go template syntax. `{{.ReviewNum}}` becomes e.g. `004`.

## Vault Structure

The critic plugin requires a [storyline](https://github.com/PixeroJan/obsidian-storyline) project at the configured `vault_path`. The project base folder contains:

```
<vault_path>/
  <Title>.md      storyline project file (frontmatter: type: storyline, definedActs, actLabels, definedChapters, chapterLabels)
  Scenes/         one .md file per scene with frontmatter (type: scene, act, chapter, sequence, title, characters, location, ...)
  Codex/
    Characters/   one .md file per character
    Locations/    one .md file per location
  Research/       worldbuilding bibles (free-form markdown)
  Exports/        storyline's own exports (the plugin produces these; critic doesn't write here)
  review/         saved reviews (numbered: 003-manuscript-critic-...)
  review/close-read/<run-id>/  per-scene close-read reports
  summary/        per-chapter summaries (generated by /critic:summarize)
  prompts/        prompt overrides (optional)
  stage.md        author override for the auto-derived stage block (optional)
  style.md        style guide (optional; Research/style.md also picked up as fallback)
  issues.md       deferred issues (optional, managed by /critic:rebuttal)
  .claude/codex-inventory.md   Codex tracking (maintained by /critic:extract)
```

Scenes sort by `act → chapter → sequence` from frontmatter. The critic plugin assembles them in that order for snapshots and reviews. Matches storyline's "Export project" output.

## Review Files

Reviews are saved as `NNN-prefix-timestamp.md` with globally sequential numbering across all review types. Issues are numbered `ISSUE-NNN-NN` where NNN matches the review number.

Author rebuttals are added via `/critic:rebuttal ISSUE-NNN-NN` and appear as Obsidian callouts inline in the review file. Future reviewers see and respect rebuttals.

Deferred issues go to `issues.md` and are passed to manuscript reviewers with instructions to only re-raise if the issue has escalated.
