# Server

The Go MCP server lives in `critic/server/`. It's small. Its job is to expose tools that skills compose into workflows. Anything that's pure file I/O or a wrapped external process lives here. Anything that requires judgment lives in the skills.

## Layout

```
server/
  main.go              entry point, MCP tool registrations
  invoke.go            invoke-codex / invoke-pi / pi-list-models / get-prompt handlers
  config.go            config.yaml loader
  settings.go          persistent settings (read-settings / write-setting)
  agent/
    codex.go           Codex CLI wrapper (uses fanwenlin/codex-go-sdk)
    pi.go              Pi CLI wrapper (shells out, replays history)
  prompts/
    embed.go           //go:embed *.md + template rendering
    *.md               embedded prompt templates
  vault/
    vault.go           project + scenes + Codex + Research + stage
    review.go          reviews + issues + snapshots + summaries
    vault_smoke_test.go    end-to-end test against a real storyline vault
```

Build:

```bash
cd critic/server
go build -o ../bin/critic .
```

The binary serves MCP over stdio.

## Vault layer

`server/vault/` is the only package that knows about the storyline file layout. Two files.

### vault.go

Opening a vault is project discovery. Scan the root for a `<Title>.md` file with `type: storyline` frontmatter, derive the base folder, store the project file path:

```go
v, err := vault.New(root)  // returns *Vault or an error if 0 or >1 project files
```

The Vault struct exposes:

| Method | Returns | What it does |
|--------|---------|--------------|
| `ReadProject()` | `*Project` | Parses `<Title>.md` frontmatter: title, defined acts/chapters, labels, descriptions. |
| `ReadScenes()` | `[]Scene` | Walks `Scenes/`, parses frontmatter, sorts by act/chapter/sequence. |
| `AssembleManuscript(p, scenes)` | `string` | Emits the storyline plugin's manuscript export format byte-for-byte. |
| `ReadManuscript()` | `string` | Convenience: ReadProject + ReadScenes + AssembleManuscript. |
| `RenderChapter(p, n, scenes)` | `string` | One chapter, with `### Chapter N: <label>` and per-scene `#### <title>` headings. |
| `RenderScene(s)` | `string` | One scene as `#### <title>\n\n<body>`. |
| `ReadResearchFiles()` | `map[path]content` | Every `.md` under `Research/`. |
| `ListCodexEntries()` | `[]string` | Filenames (no `.md`) of every Codex entry. |
| `ReadCodexEntry(name)` | `string` | One entry's content. Searches Characters/ then Locations/. |
| `ReadCodexEntries(names)` | `map[path]content` | Filtered to `names`; pass nil for all. |
| `SceneEntityNames(scenes)` | `[]string` | Union of POV, characters, location across scenes. |
| `ReadStyleGuide()` | `string` | `style.md` or `""` (the server's `read-style-guide` tool checks Research/style.md as fallback). |
| `ReadStage()` | `string` | `stage.md` or `""`. |
| `DerivedStage(p, scenes)` | `string` | Synthesised stage block from project + scene metadata. |

Wikilinks are stripped consistently. `[[Name]]` becomes `Name`. `[[Path/To/Name]]` becomes `Name`. `[[Name|Alias]]` becomes `Alias`. `[[Name#heading]]` becomes `Name`.

YAML frontmatter is parsed with `gopkg.in/yaml.v3`. Coercion helpers handle the storyline plugin's tolerance for int-vs-string-valued fields (both `act: 1` and `act: "1"` work).

### review.go

Review files, issues, snapshots, summaries, reviewer memory.

| Method | What it does |
|--------|--------------|
| `WriteReview(prefix, content)` | Write `review/NNN-prefix-timestamp.md` with the next global counter. |
| `NextReviewNumber()` | Scan `review/` for the highest `NNN-` prefix; return next. |
| `ReadLatestReview(prefix)` | Most recent review file containing the prefix. |
| `ReadLatestReviewSynthesis(prefix)` | Same, but cut at the sentinel. |
| `ReadReviewByNumber(n)` | Load a review by its global number. |
| `WriteStagedPart(name, content)` | Write to `review/.staging/`. |
| `AssembleReview(prefix, synthesisKey, partKeys)` | Combine staged parts into a final review with the sentinel. Cleans staging. |
| `WriteReviewFile(filename, content)` | Overwrite a review file by name (used by `add-rebuttal`). |
| `ReadIssues()` / `AppendIssue(heading, entry)` | `issues.md` management. |
| `WriteSummary(name, content)` | Per-chapter summary writes. |
| `WriteSnapshot(prefix)` | Assemble manuscript, write to `.snapshots/`, return (path, prior_path). |
| `SnapshotAndDiff(prefix)` | Write snapshot + compute diff against prior + save paired `.diff` file. Returns all four paths plus diff text. |
| `DiffSnapshots(prior, current)` | Unified diff via `diff -u`. |

Snapshots use the storyline plugin's export format because `WriteSnapshot` calls `ReadManuscript`. Diffs are therefore against meaningful structure (`### Chapter 3:` headers move when you reorder; `#### scene title` headers track scenes).

## MCP tools

All tools live in `main.go`. Every tool takes a `vault` parameter (absolute path to the storyline project base) except `pi-list-models`, `get-prompt`, and the settings tools.

### Manuscript and slice assembly

| Tool | Purpose |
|------|---------|
| `assemble-manuscript` | Return the full manuscript in plugin export format. Used to inline the manuscript for Claude subagents. (Codex and Pi get it via `include_manuscript_from` on their invoke calls.) |
| `assemble-chapter(chapter)` | Return JSON `{text, entities, scene_count}` for one chapter. |
| `read-scene(scene)` | Return JSON `{text, entities, act, chapter, sequence, title}` for one scene. |
| `list-scenes` | Return one line per scene: `<act>/<chapter>/<sequence> | <filename> | <title>`, in manuscript order. |

### Context blocks

| Tool | Purpose |
|------|---------|
| `read-stage` | Return `stage.md` if present, else the auto-derived stage block. |
| `read-style-guide` | Return `style.md` if present, falling back to `Research/style.md`. |
| `read-research` | Concatenate every `.md` under `Research/` with `### <relpath>` headers. |
| `read-codex(names?)` | Concatenate Codex entries (Characters + Locations) with `### <relpath>` headers. Pass `names` as comma-separated entity names to filter; omit for all. |
| `read-codex-entry(name)` | One Codex entry by name. Used for on-demand Claude subagent lookups. |
| `list-codex-entries` | One entity name per line. Used by `/critic:extract` to give subagents the canonical roster. |
| `find-entity-mentions(name)` | JSON array of `{filename, act, chapter, sequence, title, body}` for every scene mentioning `name`. Used for `/critic:extract entity` whole-book scans. |

### Reviews and issues

| Tool | Purpose |
|------|---------|
| `next-review-number` | Next global review counter (used to compute issue ID prefixes). |
| `stage-review-part(name, content)` | Write to `review/.staging/<name>`. |
| `assemble-review(prefix, synthesis_part, raw_parts)` | Combine staged parts into a final review. Cleans staging. |
| `save-review(prefix, content)` | Legacy single-shot review write. Prefer stage + assemble for large documents. |
| `read-issue(issue_id)` | Find the issue block by ID in its source review file. |
| `add-rebuttal(issue_id, rebuttal)` | Insert an Obsidian callout after the issue. |
| `read-issues` | `issues.md` content. |
| `append-issue(heading, entry)` | Add to `issues.md` under a heading; creates file and heading as needed. |

### Snapshots

| Tool | Purpose |
|------|---------|
| `snapshot-and-diff(prefix)` | Atomic: write snapshot, locate prior, compute diff, save paired `.diff`. Returns JSON `{snapshot_path, prior_path, diff_path, diff_text}`. |
| `write-snapshot(prefix)` | Just write the snapshot. |
| `diff-snapshots(prior, current)` | Unified diff between two snapshot files. |

### Invokes

| Tool | Purpose |
|------|---------|
| `invoke-codex` | One-shot or resumed Codex call. Returns `{response, session_id}`. Optional `include_manuscript_from` appends the manuscript server-side. |
| `invoke-pi` | Same shape for Pi. Optional `provider`/`model` overrides on new sessions only (resumed sessions stay pinned). |
| `pi-list-models` | Wraps `pi --list-models`. |
| `get-prompt(name, vault?, vars?)` | Resolve and render a prompt template. |

### Settings and misc

| Tool | Purpose |
|------|---------|
| `read-settings` | Current settings JSON. |
| `write-setting(key, value)` | Update one setting. |
| `update-memory` | Legacy reviewer-memory hook. Not currently used by any skill. |

## Agent wrappers

`agent/codex.go` wraps the OpenAI Codex CLI via the `fanwenlin/codex-go-sdk` Go binding. The wrapper reuses one `codex.Codex` client per process. It pins `SandboxMode: ReadOnly`, `ApprovalPolicy: Never`, `SkipGitRepoCheck: true` because the CLI doesn't need to write or ask. It holds `thread_id`s implicitly: `RunSession` starts a new thread and returns its ID; `Resume(threadID, prompt)` continues it. It drops the model flag if `codex_model` is unset, letting the CLI choose what the subscription supports.

`agent/pi.go` wraps the Pi CLI via `os/exec`. The wrapper shells out to `pi -p <prompt> --no-session --no-tools` plus optional `--provider` and `--model`. It builds the prompt by flattening a turn sequence (system, user, assistant) into one string with structured headers, because `pi -p` is one-shot and stateless. It maintains sessions in-memory: a sync-protected map of `pi-N` to turn list. `StartSession` returns `pi-N` and stashes turns. `Resume(pi-N, prompt)` appends and re-flattens. Provider and model are pinned at session creation. Resumes use the original.

The asymmetry is real. Codex has actual server-side sessions. Pi sessions are an emulation that costs a full message-history re-send each turn. For cross-review (one extra turn per reviewer) this is fine.

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

`vault/vault_smoke_test.go` is an integration test that opens the Noblesse Oblige storyline vault (skipped if not on disk) and exercises:

- Project discovery and frontmatter parsing
- Scene loading and sort order
- Manuscript assembly format (matches storyline export)
- Wikilink stripping in scene bodies
- Codex filtering by entity name
- Research file walking
- Derived stage block contents
- Entity name union across scenes
- RenderChapter and RenderScene output

Run with:

```bash
cd critic/server
go test ./vault/ -run TestStorylineVaultSmoke
```

This is the only test currently in the codebase. New test cases should live next to it.

## What the server isn't

The server doesn't know what a review is structurally. It exposes `stage-review-part` and `assemble-review`. What parts there are and in what order they go is the skill's job.

The server doesn't dispatch reviewers. The skills do. The server just provides the surface (`invoke-codex`, `invoke-pi`, and `Task` is a built-in tool).

The server doesn't read prompts on disk. It embeds them. The resolution chain lets authors override, but the source of truth ships in the binary.

This is the deliberate split. Server is plumbing. Skills are workflow.
