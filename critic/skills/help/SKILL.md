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
| `/critic:delta [author note]` | Iterative review of the changes since the last review. Full manuscript as context, changed scenes as the target, judged against the author's note. Non-interactive. The frequent one. |
| `/critic:manuscript-craft [author note]` | Whole-manuscript review, craft frame: consulting readers for the author's editor, judged against the author's intent. No market, no ledger of prior issues, no process commentary. Interactive setup. |
| `/critic:manuscript-publication [author note]` | Whole-manuscript review, publication frame: consultants to the literary agent, judged against a saleable finished book, prior issues tracked. Interactive setup. |
| `/critic:review chapter <N>` / `scene <id>` | Four-role review (analytical, immersive, structural, adversarial) scoped to one chapter or one scene. Frame from the `frame` setting. |
| `/critic:close-read scene <id>` / `chapter <N>` / `all` | Line-editor / copy-editor pass. Subagent per scene; quote-and-fix output covering typos, prose, micro-structure, canon adherence, style. Saves per-scene reports under `Review/close-read/<run-id>/`. |
| `/critic:downstream chapter <N>` / `scene <id>` | Assess what breaks after the edit point. |
| `/critic:extract chapter <N>` / `scene <id>` / `entity <name>` | Reconcile prose against the Codex + Research; maintains `.claude/codex-inventory.md`. Every change requires approval. |
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
| `vault_path` | Absolute path to your book project (the folder containing `Story/`) |
| `frame` | `craft` or `publication` (default). Picks the reviewer framing for `/critic:review`, `/critic:delta`, `/critic:downstream`, `/critic:consult`. |
| `codex_enabled` | Enable/disable Codex (true/false) |
| `codex_model` | Codex model (leave empty to let the Codex CLI pick) |
| `openai_api_key` | OpenAI API key (omit for Codex CLI login) |
| `pi_enabled` | Enable/disable Pi |
| `pi_provider` | Default Pi provider: anthropic, openai, google |
| `pi_model` | Default Pi model (skills may override per-call) |
| `adversary_provider` | Pi provider for the frank reader in `/critic:delta` (empty = `pi_provider`) |
| `adversary_model` | Pi model for the frank reader in `/critic:delta` (empty = `pi_model`) |
| `claude_enabled` | Enable/disable the `invoke-claude` tool |
| `claude_model` | Model for `invoke-claude` (empty = CLI default) |

Inside cowork, Claude is always available as a Task subagent regardless of these settings; its model there is whatever cowork is running on. The `claude_*` settings affect only the `invoke-claude` MCP tool, which lets a non-Claude leader (e.g. Codex CLI with this MCP server registered) dispatch Claude as a reviewer.

Settings can also be set in `config.yaml` in the plugin directory. Settings from `/critic:settings` override `config.yaml`.

### Prompt Overrides

All reviewer prompts can be customized without rebuilding. Prompts are loaded from (in order):

1. `<vault>/prompts/<name>.md` (per-project override)
2. `<plugin>/prompts/<name>.md` (plugin-level default)
3. Compiled-in default (fallback built into the binary)

To customize a prompt, copy it from the plugin's `prompts/` directory to `<vault>/prompts/` and edit. The server picks up the change on the next tool call.

Available prompt files:

Two frames. The publication frame (`agent-framing.md`, `manuscript.md`, `adversarial.md`, `rejection-pass.md`, `synthesis.md`, `verdict.md`) advises a literary agent. The craft frame (`craft-framing.md`, `manuscript-craft.md`, `adversarial-craft.md`, `rejection-pass-craft.md`, `synthesis-craft.md`, `verdict-craft.md`) advises the author's developmental editor. `delta.md` and `synthesis-delta.md` are the changes-only review and are used under either framing.

| File | Used by | Variables |
|------|---------|-----------|
| `agent-framing.md` | Publication frame, all reviewers | None |
| `craft-framing.md` | Craft frame, all reviewers | None |
| `verdict.md` | Publication frame | None |
| `verdict-craft.md` | Craft frame | None |
| `review-base.md` | Chapter review | `{{.Role}}`, `{{.MaxIssues}}` |
| `review-analytical.md` | Analytical reader role | None |
| `review-immersive.md` | Immersive reader role | None |
| `review-structural.md` | Structural analyst role | None |
| `review-adversarial-role.md` | Adversarial critic role | None |
| `cross-review.md` | Cross-review pass | `{{.MaxNewIssues}}` |
| `synthesis.md` | Synthesis, publication frame | `{{.ReviewNum}}` (zero-padded to 3 digits) |
| `synthesis-craft.md` | Synthesis, craft frame | `{{.ReviewNum}}` |
| `synthesis-delta.md` | Synthesis for `/critic:delta` | `{{.ReviewNum}}` |
| `manuscript.md` | Manuscript review, publication frame | None |
| `manuscript-craft.md` | Manuscript review, craft frame | None |
| `delta.md` | Changes-only review (`/critic:delta`) | `{{.MaxIssues}}` |
| `rejection-pass.md` | Claude's rejection pass, publication frame | None |
| `rejection-pass-craft.md` | Claude's rejection pass, craft frame | None |
| `adversarial.md` | Pi adversary, publication frame | None |
| `adversarial-craft.md` | Pi frank reader, craft frame | None |
| `extract-slice.md` | Canon extraction. Chapter/scene mode | None |
| `extract-entity.md` | Canon extraction. Entity mode | None |
| `close-read.md` | Line-edit / copy-edit pass | None |
| `downstream.md` | Downstream assessment | None |

Variables use Go template syntax. `{{.ReviewNum}}` becomes e.g. `004`.

## Vault Structure

The configured `vault_path` is a book project folder. Only `Story/` is required.

```
<vault_path>/
  <Title>.md      book note (optional): frontmatter `type: book`, `title`, `acts` (act number → label)
  Story/          one .md file per chapter
  Background/
    Characters/   one .md file per character (the Codex)
    Locations/    one .md file per location (the Codex)
    *.md          worldbuilding docs, style guide, timeline (the Research)
  Review/         saved reviews (numbered: 003-delta-craft-..., 004-manuscript-craft-..., 005-manuscript-critic-...)
  Review/.snapshots/           manuscript snapshots per lineage (craft-*, publication-*), each with a .json sidecar naming the review it was taken for, and .diff files
  Review/close-read/<run-id>/  per-scene close-read reports
  summary/        per-chapter summaries (generated by /critic:summarize)
  prompts/        prompt overrides (optional)
  stage.md        author override for the derived stage block (optional)
  style.md        style guide (optional; Background/style.md also picked up as fallback)
  issues.md       deferred issues (optional, managed by /critic:rebuttal)
  .claude/codex-inventory.md   Codex tracking (maintained by /critic:extract)
```

### Chapter files

Each file in `Story/` is one chapter. Frontmatter carries `chapter` (the number; falls back to the file name's leading number), `title`, `act`, `status`, `pov`, `characters`, `locations`, and a `scenes:` list of per-scene metadata (`title`, `pov`, `location`, `characters`, `conflict`, `emotion`, `intensity`, `status`, `notes`). The body is the prose, one `## <scene title>` heading per scene. Anything above the first scene heading (an epigraph, say) belongs to the chapter. A chapter with no scene headings is one untitled scene.

Chapter numbers are global across acts and must be unique; two files with the same number is an error. Scenes are addressed `CC-SS`: chapter, then position within the chapter (`04-02` is the second scene of chapter 4). `list-scenes` shows the mapping. Per-scene metadata pairs with body sections by title, so reordering scenes in the body keeps their POV and cast attached.

Critic assembles chapters in number order as `# Title`, `## Act N: <label>`, `### Chapter N: <title>`, `#### <scene title>`, body. Snapshots use the same shape, so the diff between reviews shows only prose changes. Snapshots are grouped by lineage (craft, publication): a review diffs only against the previous snapshot in its own lineage, so switching frames starts a fresh baseline.

## Review Files

Reviews are saved as `NNN-prefix-timestamp.md` with globally sequential numbering across all review types. Issues are numbered `ISSUE-NNN-NN` where NNN matches the review number.

Author rebuttals are added via `/critic:rebuttal ISSUE-NNN-NN` and appear as Obsidian callouts inline in the review file. Future reviewers see and respect rebuttals.

Deferred issues go to `issues.md` and are passed to manuscript reviewers with instructions to only re-raise if the issue has escalated.

In the craft frame, silence is deferral: reviewers treat every unaddressed prior issue as the author's sequencing decision and never report what remains open. Explicit rebuttals and deferrals still help, because they tell reviewers why.
