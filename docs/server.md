# Server

The Go MCP server lives in `critic/server/`. It's small. Its job is to expose tools that skills compose into workflows. Anything that's pure file I/O or a wrapped external process lives here. Anything that requires judgment lives in the skills.

## Layout

```
server/
  main.go              entry point, MCP tool registrations
  invoke.go            invoke-claude / invoke-codex / invoke-pi / invoke-status / pi-list-models / get-prompt handlers
  jobs.go              background invocations: job registry, wait budgets, progress heartbeat
  jobs_test.go         job lifecycle, cancellation survival, heartbeat, disk mirroring, pruning
  config.go            config.yaml loader
  settings.go          persistent settings (read-settings / write-setting)
  agent/
    claude.go          Claude CLI wrapper (headless claude -p, real session resume)
    codex.go           Codex CLI wrapper (uses fanwenlin/codex-go-sdk)
    pi.go              Pi CLI wrapper (shells out, replays history)
  prompts/
    embed.go           //go:embed *.md + template rendering
    *.md               embedded prompt templates
  vault/
    vault.go           book note + chapters + scenes + Codex + Research + stage
    review.go          reviews + issues + snapshots + summaries
    vault_test.go      fixture tests for the vault layer
```

Build:

```bash
cd critic/server
go build -o ../bin/critic .
```

The binary serves MCP over stdio.

## Vault layer

`server/vault/` is the only package that knows the file layout. Two files. See [vault.md](vault.md) for the layout itself.

### vault.go

Opening a vault checks for a `Story/` folder and nothing else:

```go
v, err := vault.New(root)  // error if root isn't a directory or has no Story/
```

The Vault struct exposes:

| Method | Returns | What it does |
|--------|---------|--------------|
| `ReadBook()` | `*Book` | The root-level note with `type: book`: title, act labels, description. Falls back to the folder name. |
| `ReadChapters()` | `[]Chapter` | Parses every chapter file in `Story/`, splits scenes on `## ` headings, pairs them with `scenes:` metadata, sorts by chapter number. Errors on duplicate numbers. |
| `AssembleManuscript(b, chapters)` | `string` | `# Title`, `## Act N`, `### Chapter N`, `#### <scene>`, prose. |
| `ReadManuscript()` | `string` | ReadBook + ReadChapters + AssembleManuscript. Errors on an empty `Story/`. |
| `RenderChapter(ch)` | `string` | One chapter with its heading, any preamble, and each scene. |
| `RenderScene(s)` | `string` | One scene as `#### <title>\n\n<body>`. |
| `FindChapter(chapters, n)` | `*Chapter` | Chapter by number. |
| `FindScene(chapters, ref)` | `*Scene` | Scene by `CC-SS` (also `4.2`, `4-2`, a trailing title) or exact title. Errors on an ambiguous title. |
| `AllScenes(chapters)` | `[]Scene` | Scenes in manuscript order. |
| `ReadResearchFiles()` | `map[path]content` | Every `.md` under `Background/` except `Characters/` and `Locations/`. |
| `ListCodexEntries()` | `[]string` | File names (no `.md`) under `Background/Characters/` and `Background/Locations/`. |
| `ReadCodexEntry(name)` | `string` | One entry. Searches Characters/ then Locations/. |
| `ReadCodexEntries(names)` | `map[path]content` | Filtered to `names`; pass nil for all. |
| `SceneEntityNames(scenes)` / `ChapterEntityNames(ch)` | `[]string` | Union of POV, characters, locations. |
| `ReadStyleGuide()` | `string` | `style.md`, then `Background/style.md`, else `""`. |
| `ReadStage()` | `string` | `stage.md` or `""`. |
| `DerivedStage(b, chapters)` | `string` | Stage block from the book note and chapters. |
| `ChapterWordCount(ch)` | `int` | Prose words, computed on every call. |

Wikilinks are stripped from prose. `[[Name]]` becomes `Name`. `[[Path/To/Name]]` becomes `Name`. `[[Name|Alias]]` becomes `Alias`. Frontmatter wikilinks are cleaned the same way, and `[[Name#heading]]` becomes `Name`.

YAML frontmatter is parsed with `gopkg.in/yaml.v3`. Coercion helpers accept int-or-string fields (`chapter: 4` and `chapter: "4"` both work) and int-keyed maps (`acts: {1: The Rim}` decodes as `map[any]any`).

### review.go

Review files, issues, snapshots, summaries, reviewer memory.

| Method | What it does |
|--------|--------------|
| `WriteReview(prefix, content)` | Write `Review/NNN-prefix-timestamp.md` with the next global counter. |
| `NextReviewNumber()` | Scan `Review/` for the highest `NNN-` prefix; return next. |
| `ReadLatestReview(prefix)` | Most recent review file containing the prefix. |
| `ReadLatestReviewSynthesis(prefix)` | Same, but cut at the sentinel. |
| `ReadReviewByNumber(n)` | Load a review by its global number. |
| `WriteStagedPart(name, content)` | Write to `Review/.staging/`. |
| `AssembleReview(prefix, synthesisKey, partKeys)` | Combine staged parts into a final review with the sentinel. Cleans staging. |
| `WriteReviewFile(filename, content)` | Overwrite a review file by name (used by `add-rebuttal`). |
| `ReadIssues()` / `AppendIssue(heading, entry)` | `issues.md` management. |
| `WriteSummary(name, content)` | Per-chapter summary writes. |
| `WriteSnapshot(meta)` | Assemble manuscript, write `.snapshots/<lineage>-<ts>.md` plus a `.json` sidecar with the `SnapshotMeta` (lineage, kind, review, created), return (path, prior_path). The prior is the newest snapshot in the same lineage, skipping orphans (see below). |
| `SnapshotAndDiff(meta)` | `WriteSnapshot` + unified diff against the prior + paired `.diff` file. Returns all four paths plus diff text. |
| `ListSnapshots(lineage)` | Every snapshot, oldest first, with its meta and diff path; `lineage` filters, `""` lists all. |
| `DiffSnapshots(prior, current)` | Unified diff via `diff -u`. |
| `ChangedScenes(prior, current)` | Compare two assembled manuscripts scene by scene. Returns a `ChangeSet`: one `SceneChange` (chapter, scene, added/modified/removed, word counts) per differing scene, plus the full text of the added and modified scenes under their headings. |
| `SnapshotChanges(prior, current)` | `ChangedScenes` over two snapshot files. |

`WriteSnapshot` calls `ReadManuscript`, so a snapshot is exactly what reviewers see. Diffs track meaningful structure: `### Chapter 3:` headers move when you reorder chapters; `#### scene title` headers track scenes.

Snapshots are grouped by lineage. A review diffs only against the newest snapshot in its own lineage: craft against craft, publication against publication, never across. The lineage is in the file name and the sidecar; a snapshot with no sidecar (from before lineages existed) is named `manuscript-*` and reads as publication. Lineage names are lowercase letters, digits, and hyphens; `manuscript` is refused as a lineage name because it's the legacy alias. A prior snapshot whose sidecar records the same or a later review number than the one being written is the orphan of a run that died before assembling its review, and is skipped.

`ChangedScenes` parses that same shape back into sections keyed by chapter number, scene title, and occurrence, and compares bodies. Text between a chapter heading and its first scene heading is the chapter preamble (or its single untitled scene) and is keyed with an empty title. Act and title headings belong to no scene, so relabelling an act is not a scene change. A retitled scene reads as removed plus added; there's no rename detection, and none is needed for pointing a reviewer at what changed.

## MCP tools

All tools live in `main.go`. Every tool takes a `vault` parameter (absolute path to the book folder, the one containing `Story/`) except `pi-list-models`, `get-prompt`, and the settings tools.

### Manuscript and slice assembly

| Tool | Purpose |
|------|---------|
| `assemble-manuscript` | Return the full manuscript, assembled from `Story/`. Used to inline the manuscript for Claude subagents. (Codex and Pi get it via `include_manuscript_from` on their invoke calls.) |
| `assemble-chapter(chapter)` | Return JSON `{text, entities, scene_count, title, file}` for one chapter. |
| `read-scene(scene)` | Return JSON `{id, chapter, scene, chapter_title, title, text, entities}` for one scene. `scene` is `CC-SS` or an exact title. |
| `list-chapters` | One line per chapter: `<n> \| <file> \| <title> \| <scenes> \| <words> \| <status>`. |
| `list-scenes` | One line per scene: `<CC-SS> \| <chapter file> \| <title>`, in manuscript order. |

### Context blocks

| Tool | Purpose |
|------|---------|
| `read-stage` | Return `stage.md` if present, else the auto-derived stage block. |
| `read-style-guide` | Return `style.md` if present, falling back to `Background/style.md`. |
| `read-research` | Concatenate the worldbuilding docs (every `.md` under `Background/` except the Codex folders) with `### <relpath>` headers. |
| `read-codex(names?)` | Concatenate Codex entries (`Background/Characters` + `Background/Locations`) with `### <relpath>` headers. Pass `names` as comma-separated entity names to filter; omit for all. |
| `read-codex-entry(name)` | One Codex entry by name. Used for on-demand Claude subagent lookups. |
| `list-codex-entries` | One entity name per line. Used by `/critic:extract` to give subagents the canonical roster. |
| `find-entity-mentions(name)` | JSON array of `{id, chapter, scene, title, body}` for every scene mentioning `name`. Used for `/critic:extract entity` whole-book scans. |

### Reviews and issues

| Tool | Purpose |
|------|---------|
| `next-review-number` | Next global review counter (used to compute issue ID prefixes). |
| `stage-review-part(name, content)` | Write to `Review/.staging/<name>`. |
| `assemble-review(prefix, synthesis_part, raw_parts)` | Combine staged parts into a final review. Cleans staging. |
| `save-review(prefix, content)` | Legacy single-shot review write. Prefer stage + assemble for large documents. |
| `read-issue(issue_id)` | Find the issue block by ID in its source review file. |
| `add-rebuttal(issue_id, rebuttal)` | Insert an Obsidian callout after the issue. |
| `read-issues` | `issues.md` content. |
| `append-issue(heading, entry)` | Add to `issues.md` under a heading; creates file and heading as needed. |

### Snapshots

| Tool | Purpose |
|------|---------|
| `snapshot-and-diff(lineage, kind?, review?)` | Atomic: write snapshot with sidecar, locate the prior in the lineage, compute diff, save paired `.diff`. Returns JSON `{snapshot_path, prior_path, diff_path, diff_text, changed_scenes, changed_text}`. `changed_scenes` is the scene-level change list; `changed_text` is the full prose of the added and modified scenes. Both are empty when there's nothing to diff. |
| `write-snapshot(lineage, kind?, review?)` | Just write the snapshot and sidecar. |
| `list-snapshots(lineage?)` | JSON `[{path, diff_path, lineage, kind, review, created}]`, oldest first. |
| `diff-snapshots(prior, current)` | Unified diff between two snapshot files. |
| `changed-scenes(prior, current)` | Scene-level comparison of two snapshot files. Returns JSON `{scenes: [{chapter, scene, change, words_before, words_after}], text}`. |

### Invokes

| Tool | Purpose |
|------|---------|
| `invoke-codex` | One-shot or resumed Codex call. Returns `{response, session_id}` when it finishes inside `wait_seconds`, otherwise `{status:"running", job_id}`. Optional `include_manuscript_from` appends the manuscript server-side. |
| `invoke-pi` | Same shape for Pi. Optional `provider`/`model` overrides on new sessions only (resumed sessions stay pinned). |
| `invoke-claude` | Same shape for headless Claude (`claude -p`). Real server-side session resume. Exists for harness-agnostic use: a non-Claude leader (Codex CLI, etc.) dispatches Claude as a reviewer through this tool. Inside cowork, skills use Task subagents instead. |
| `invoke-status(job_id?, wait_seconds?)` | Collect an invocation by id, blocking up to `wait_seconds` for it to finish. With no `job_id`, lists every job the server holds. |
| `pi-list-models` | Wraps `pi --list-models`. |
| `get-prompt(name, vault?, vars?)` | Resolve and render a prompt template. |

### Settings and misc

| Tool | Purpose |
|------|---------|
| `read-settings` | Current settings JSON. |
| `write-setting(key, value)` | Update one setting. Keys are whitelisted (see `/critic:settings`); `frame` must be `craft` or `publication`. |
| `update-memory` | Legacy reviewer-memory hook. Not currently used by any skill. |

## Long invocations

A full-manuscript review runs for minutes. An MCP tool call cannot stay open
that long. Clients cap tool calls, and the cap is shorter than the work, which
is why manuscript reviews used to come back as timeouts while trivial probes
through the same tools returned instantly. The failure had nothing to do with
Codex, or with the size of the manuscript: a 189 KB manuscript with a one-line
question answers in about six seconds. What takes minutes is generating the
review itself.

`jobs.go` decouples the two. Every `invoke-*` call starts a Job on the server's
root context, not the request's, so the work outlives the tool call that asked
for it. The call then waits out a budget (`wait_seconds`, default 60, max 300)
and answers with whatever is true at that moment: the finished result in the
original `{response, session_id}` shape, or `{status:"running", job_id}`. The
caller collects the rest with `invoke-status`, which long-polls, so a review
costs about one tool call per minute of model time rather than a busy-wait.

Two things make this recoverable rather than merely asynchronous. Finished
output is mirrored to `~/Library/Caches/critic/jobs/<job-id>.txt`, so a review
survives the session that ordered it. And `invoke-status` with no `job_id`
lists every job still held (six hours after completion), which is the way back
in when a session loses track of an invocation.

While a call is waiting it emits `notifications/progress` every ten seconds,
against the client's own progress token. This is a liveness signal, not a
timeout extension: clients that enforce an idle timeout reset it on each
notification, but a client with a hard wall-clock cap will still cut the call
off at the cap. That is exactly why the budget exists and defaults well under
any plausible cap. The heartbeat is skipped when the client supplies no
progress token, which the MCP spec requires.

## Agent wrappers

`agent/codex.go` wraps the OpenAI Codex CLI via the `fanwenlin/codex-go-sdk` Go binding. The wrapper reuses one `codex.Codex` client per process. It pins `SandboxMode: ReadOnly`, `ApprovalPolicy: Never`, `SkipGitRepoCheck: true` because the CLI doesn't need to write or ask. It holds `thread_id`s implicitly: `RunSession` starts a new thread and returns its ID; `Resume(threadID, prompt)` continues it. It drops the model flag if `codex_model` is unset, letting the CLI choose what the subscription supports.

`agent/pi.go` wraps the Pi CLI via `os/exec`. The wrapper shells out to `pi -p <prompt> --no-session --no-tools` plus optional `--provider` and `--model`. It builds the prompt by flattening a turn sequence (system, user, assistant) into one string with structured headers, because `pi -p` is one-shot and stateless. It maintains sessions in-memory: a sync-protected map of `pi-N` to turn list. `StartSession` returns `pi-N` and stashes turns. `Resume(pi-N, prompt)` appends and re-flattens. Provider and model are pinned at session creation. Resumes use the original.

`agent/claude.go` wraps the Claude Code CLI in headless print mode. `RunSession` runs `claude -p --output-format json --system-prompt <sys> --tools "" --disable-slash-commands`, passing the user prompt via stdin (manuscripts exceed comfortable argv sizes) and parsing the JSON result for `result` and `session_id`. `Resume` runs `claude -p -r <session-id>`; the system prompt and history persist server-side in the session. The wrapper sets a neutral working directory so CLAUDE.md project auto-discovery doesn't pull unrelated context into the reviewer, and scrubs the `CLAUDECODE` nesting markers from the child environment.

The session asymmetry across the three wrappers: Codex and Claude have real server-side sessions; Pi sessions are an emulation that costs a full message-history re-send each turn. For cross-review (one extra turn per reviewer) all three are fine.

## Prompt system

`server/prompts/embed.go` exposes two functions. `Load(name, vaultPath)` resolves the prompt with no template processing. `Render(name, vaultPath, data)` resolves and executes as a Go `text/template`.

Resolution order:

1. `<vaultPath>/prompts/<name>` (per-project author override)
2. `$CLAUDE_PLUGIN_ROOT/prompts/<name>` (plugin-directory override)
3. Embedded default (via `//go:embed *.md`)

Templates use Go template syntax: `{{.Role}}`, `{{if}}`, etc. The MCP tool `get-prompt` is the skill-facing surface. It takes `vars` as a JSON object that gets parsed and passed as `data`.

The full template catalog is in [prompts.md](prompts.md).

## Settings

Persistent settings live in `${CLAUDE_PLUGIN_DATA}/settings.json`. `config.yaml` in the plugin directory holds defaults. `/critic:settings` overrides them.

`settings.go` reads on every `read-settings` call. There's no caching. The file is small.

## Testing

`vault/vault_test.go` builds a small book in a temp directory and covers:

- `Story/` as the one hard requirement, and book-note discovery with its fallbacks
- Chapter numbering from frontmatter or file name, and the duplicate-number error
- Scene splitting: `## ` starts a scene, `###` and `---` stay in the prose, a headingless chapter is one scene, text above the first heading is a preamble
- Scene metadata pairing by title when the body is reordered
- The exact assembled manuscript, including act and chapter headings and wikilink stripping
- Scene lookup by `CC-SS`, shorthand, old filename, and title, including the ambiguous-title error
- Codex and Research reads from `Background/`, and the style-guide fallback
- Review numbering, staged assembly, and snapshot paths under `Review/`

`jobs_test.go` covers the invocation job runner.

```bash
cd critic/server
go test ./...
```

## What the server isn't

The server doesn't know what a review is structurally. It exposes `stage-review-part` and `assemble-review`. What parts there are and in what order they go is the skill's job.

The server doesn't dispatch reviewers. The skills do. The server just provides the surface (`invoke-codex`, `invoke-pi`, `invoke-claude`, and `Task` is a built-in tool). This is deliberate and settled: orchestration was moved out of Go into skills early on, and it stays there. The invoke tools make every reviewer reachable from any MCP-speaking leader; the playbook that sequences them is always markdown interpreted by the leader.

The server doesn't read prompts on disk. It embeds them. The resolution chain lets authors override, but the source of truth ships in the binary.

This is the deliberate split. Server is plumbing. Skills are workflow.
