package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Vault represents a StoryLine project. Root is the project base folder —
// the directory that contains Scenes/, Codex/, Research/, and where we
// keep review/, summary/, snapshots/, stage.md, style.md.
type Vault struct {
	Root         string // project base folder
	ProjectFile  string // absolute path to <Title>.md
	ProjectTitle string // basename without .md
}

// Project mirrors the storyline plugin's project frontmatter — the parts the
// critic system cares about (acts, chapters, labels, descriptions).
type Project struct {
	Title               string
	Description         string
	Language            string
	DefinedActs         []int
	DefinedChapters     []int
	ActLabels           map[int]string
	ChapterLabels       map[int]string
	ActDescriptions     map[int]string
	ChapterDescriptions map[int]string
}

// Scene is a parsed scene file from Scenes/.
type Scene struct {
	Path       string // absolute path
	Filename   string // basename without .md
	Title      string
	Act        int      // 0 if unset
	Chapter    int      // 0 if unset
	Sequence   int      // 9999 if unset
	POV        string   // cleaned wikilink
	Characters []string // cleaned wikilinks
	Location   string   // cleaned wikilink
	Body       string   // post-frontmatter, trimmed, wikilinks stripped to display name
	Wordcount  int      // from frontmatter, 0 if absent
}

// New opens a storyline vault rooted at `root`. The root must be a directory
// containing exactly one `<Title>.md` file with `type: storyline` frontmatter
// whose derived base folder equals `root`. Returns an error if zero or
// multiple matching projects are found.
func New(root string) (*Vault, error) {
	if root == "" {
		return nil, fmt.Errorf("vault root is empty")
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("stat vault root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("vault root %q is not a directory", root)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read vault root: %w", err)
	}

	var matches []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(root, e.Name())
		if isStorylineProjectFile(path) {
			matches = append(matches, path)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no storyline project (.md with `type: storyline`) found in %s", root)
	case 1:
		return &Vault{
			Root:         root,
			ProjectFile:  matches[0],
			ProjectTitle: strings.TrimSuffix(filepath.Base(matches[0]), ".md"),
		}, nil
	default:
		return nil, fmt.Errorf("multiple storyline projects found in %s; expected exactly one", root)
	}
}

func isStorylineProjectFile(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	fm, _ := extractFrontmatter(string(data))
	if fm == nil {
		return false
	}
	t, _ := fm["type"].(string)
	return t == "storyline"
}

// ReadProject loads the project frontmatter and description body.
func (v *Vault) ReadProject() (*Project, error) {
	data, err := os.ReadFile(v.ProjectFile)
	if err != nil {
		return nil, fmt.Errorf("read project file: %w", err)
	}
	fm, body := extractFrontmatter(string(data))
	if fm == nil {
		return nil, fmt.Errorf("project file %s has no frontmatter", v.ProjectFile)
	}

	p := &Project{
		Title:               coerceString(fm["title"]),
		Description:         strings.TrimSpace(body),
		Language:            coerceString(fm["language"]),
		DefinedActs:         coerceIntSlice(fm["definedActs"]),
		DefinedChapters:     coerceIntSlice(fm["definedChapters"]),
		ActLabels:           coerceIntKeyedStringMap(fm["actLabels"]),
		ChapterLabels:       coerceIntKeyedStringMap(fm["chapterLabels"]),
		ActDescriptions:     coerceIntKeyedStringMap(fm["actDescriptions"]),
		ChapterDescriptions: coerceIntKeyedStringMap(fm["chapterDescriptions"]),
	}
	if p.Title == "" {
		p.Title = v.ProjectTitle
	}
	return p, nil
}

// ReadScenes loads every scene under Scenes/ and returns them sorted
// act → chapter → sequence (mirroring the plugin's export order).
// Files without `type: scene` frontmatter are silently skipped.
func (v *Vault) ReadScenes() ([]Scene, error) {
	dir := filepath.Join(v.Root, "Scenes")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read Scenes/: %w", err)
	}

	var scenes []Scene
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		fm, body := extractFrontmatter(string(data))
		if fm == nil {
			continue
		}
		if t, _ := fm["type"].(string); t != "scene" {
			continue
		}

		filename := strings.TrimSuffix(e.Name(), ".md")
		title := coerceString(fm["title"])
		if title == "" {
			title = filename
		}

		scenes = append(scenes, Scene{
			Path:       path,
			Filename:   filename,
			Title:      title,
			Act:        coerceInt(fm["act"]),
			Chapter:    coerceInt(fm["chapter"]),
			Sequence:   coerceIntDefault(fm["sequence"], 9999),
			POV:        cleanWikilink(coerceString(fm["pov"])),
			Characters: cleanWikilinks(fm["characters"]),
			Location:   cleanWikilink(coerceString(fm["location"])),
			Body:       strings.TrimSpace(body),
			Wordcount:  coerceInt(fm["wordcount"]),
		})
	}

	sort.SliceStable(scenes, func(i, j int) bool {
		if scenes[i].Act != scenes[j].Act {
			return scenes[i].Act < scenes[j].Act
		}
		if scenes[i].Chapter != scenes[j].Chapter {
			return scenes[i].Chapter < scenes[j].Chapter
		}
		return scenes[i].Sequence < scenes[j].Sequence
	})

	return scenes, nil
}

// AssembleManuscript emits scenes in the storyline plugin's Markdown export
// format: `# Title`, then `## Act N: <label>`, `### Chapter N: <label>`,
// `#### <scene title>`, body. Bodies have wikilinks stripped to display
// names. Matches ExportService.buildManuscriptMd byte-for-byte for the
// default option set (includeSceneTitles=true, numberScenes=false).
func (v *Vault) AssembleManuscript(p *Project, scenes []Scene) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", p.Title)

	var currentAct, currentChapter int = -1, -1
	for _, s := range scenes {
		if s.Act != currentAct {
			currentAct = s.Act
			currentChapter = -1
			if label := p.ActLabels[s.Act]; label != "" {
				fmt.Fprintf(&b, "## Act %d: %s\n\n", s.Act, label)
			} else {
				fmt.Fprintf(&b, "## Act %d\n\n", s.Act)
			}
		}
		if s.Chapter != currentChapter {
			currentChapter = s.Chapter
			if label := p.ChapterLabels[s.Chapter]; label != "" {
				fmt.Fprintf(&b, "### Chapter %d: %s\n\n", s.Chapter, label)
			} else {
				fmt.Fprintf(&b, "### Chapter %d\n\n", s.Chapter)
			}
		}
		fmt.Fprintf(&b, "#### %s\n\n", s.Title)
		if body := stripWikilinks(s.Body); body != "" {
			b.WriteString(body)
			b.WriteString("\n\n")
		} else {
			b.WriteString("*No content yet.*\n\n")
		}
	}
	return b.String()
}

// RenderChapter assembles one chapter as `### Chapter N: <label>\n\n` followed
// by `#### <scene title>\n\n<body>` for each scene in sequence order. Bodies
// have wikilinks stripped, matching AssembleManuscript's per-scene shape.
func RenderChapter(p *Project, chapter int, scenes []Scene) string {
	var b strings.Builder
	if label := p.ChapterLabels[chapter]; label != "" {
		fmt.Fprintf(&b, "### Chapter %d: %s\n\n", chapter, label)
	} else {
		fmt.Fprintf(&b, "### Chapter %d\n\n", chapter)
	}
	sorted := append([]Scene(nil), scenes...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Sequence < sorted[j].Sequence })
	for _, s := range sorted {
		fmt.Fprintf(&b, "#### %s\n\n", s.Title)
		if body := stripWikilinks(s.Body); body != "" {
			b.WriteString(body)
			b.WriteString("\n\n")
		} else {
			b.WriteString("*No content yet.*\n\n")
		}
	}
	return b.String()
}

// RenderScene returns `#### <title>\n\n<body>` for one scene, with wikilinks
// stripped.
func RenderScene(s Scene) string {
	body := stripWikilinks(s.Body)
	if body == "" {
		body = "*No content yet.*"
	}
	return fmt.Sprintf("#### %s\n\n%s\n", s.Title, body)
}

// ReadManuscript is a convenience: load project + scenes + assemble.
func (v *Vault) ReadManuscript() (string, error) {
	p, err := v.ReadProject()
	if err != nil {
		return "", err
	}
	scenes, err := v.ReadScenes()
	if err != nil {
		return "", err
	}
	return v.AssembleManuscript(p, scenes), nil
}

// ReadResearchFiles reads every .md file under Research/ keyed by vault-
// relative path. Returns an empty map if the directory doesn't exist.
func (v *Vault) ReadResearchFiles() (map[string]string, error) {
	return v.readMarkdownTree("Research")
}

// ListCodexEntries returns the names (filename without .md) of every entry
// in Codex/Characters/ and Codex/Locations/.
func (v *Vault) ListCodexEntries() ([]string, error) {
	var names []string
	for _, sub := range []string{"Codex/Characters", "Codex/Locations"} {
		dir := filepath.Join(v.Root, sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			names = append(names, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	sort.Strings(names)
	return names, nil
}

// ReadCodexEntry reads a single Codex entry by name (filename without .md).
// Searches Characters/ then Locations/. Returns os.ErrNotExist if absent.
func (v *Vault) ReadCodexEntry(name string) (string, error) {
	name = strings.TrimSuffix(name, ".md")
	for _, sub := range []string{"Codex/Characters", "Codex/Locations"} {
		path := filepath.Join(v.Root, sub, name+".md")
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
	}
	return "", os.ErrNotExist
}

// ReadCodexEntries reads Codex entries for the named entities. Names are
// matched case-insensitively against filenames. If `names` is empty, all
// entries are returned. Returns a map keyed by vault-relative path.
func (v *Vault) ReadCodexEntries(names []string) (map[string]string, error) {
	want := make(map[string]bool, len(names))
	for _, n := range names {
		n = strings.TrimSuffix(strings.TrimSpace(n), ".md")
		if n != "" {
			want[strings.ToLower(n)] = true
		}
	}
	includeAll := len(want) == 0

	result := make(map[string]string)
	for _, sub := range []string{"Codex/Characters", "Codex/Locations"} {
		dir := filepath.Join(v.Root, sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".md")
			if !includeAll && !want[strings.ToLower(name)] {
				continue
			}
			path := filepath.Join(dir, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			rel := filepath.Join(sub, e.Name())
			result[rel] = string(data)
		}
	}
	return result, nil
}

// SceneEntityNames returns the union of all character/POV/location names
// referenced in the given scenes' frontmatter. Useful for prefiltering
// Codex entries before sending to reviewers.
func SceneEntityNames(scenes []Scene) []string {
	seen := make(map[string]bool)
	var out []string
	add := func(n string) {
		n = strings.TrimSpace(n)
		if n == "" || seen[n] {
			return
		}
		seen[n] = true
		out = append(out, n)
	}
	for _, s := range scenes {
		add(s.POV)
		add(s.Location)
		for _, c := range s.Characters {
			add(c)
		}
	}
	sort.Strings(out)
	return out
}

func (v *Vault) readMarkdownTree(sub string) (map[string]string, error) {
	dir := filepath.Join(v.Root, sub)
	result := make(map[string]string)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return nil
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(v.Root, path)
		result[rel] = string(data)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return result, err
	}
	return result, nil
}

// ReadStyleGuide reads style.md from the project root, if present.
func (v *Vault) ReadStyleGuide() string {
	data, err := os.ReadFile(filepath.Join(v.Root, "style.md"))
	if err != nil {
		return ""
	}
	return string(data)
}

// ReadStage reads stage.md from the project root, if present. Authors
// override the auto-derived stage block by writing this file.
func (v *Vault) ReadStage() string {
	data, err := os.ReadFile(filepath.Join(v.Root, "stage.md"))
	if err != nil {
		return ""
	}
	return string(data)
}

// DerivedStage builds a stage description from project frontmatter and
// scene metadata. Used as the default when stage.md is absent.
func (v *Vault) DerivedStage(p *Project, scenes []Scene) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Current Stage\n\n")
	fmt.Fprintf(&b, "Project: %s\n", p.Title)
	if p.Description != "" {
		fmt.Fprintf(&b, "\n%s\n", p.Description)
	}

	scenesByChapter := make(map[int][]Scene)
	wordsByChapter := make(map[int]int)
	chaptersByAct := make(map[int]map[int]bool)
	totalWords := 0
	for _, s := range scenes {
		scenesByChapter[s.Chapter] = append(scenesByChapter[s.Chapter], s)
		wordsByChapter[s.Chapter] += s.Wordcount
		totalWords += s.Wordcount
		if chaptersByAct[s.Act] == nil {
			chaptersByAct[s.Act] = make(map[int]bool)
		}
		chaptersByAct[s.Act][s.Chapter] = true
	}

	fmt.Fprintf(&b, "\nDrafted so far: %d scenes, ~%d words.\n\n", len(scenes), totalWords)

	if len(p.DefinedActs) > 0 {
		fmt.Fprintf(&b, "## Acts\n\n")
		for _, act := range p.DefinedActs {
			label := p.ActLabels[act]
			if label != "" {
				fmt.Fprintf(&b, "- **Act %d — %s**", act, label)
			} else {
				fmt.Fprintf(&b, "- **Act %d**", act)
			}
			if chs := chaptersByAct[act]; len(chs) > 0 {
				fmt.Fprintf(&b, " — %d chapter(s) with drafted scenes", len(chs))
			} else {
				fmt.Fprintf(&b, " — not yet drafted")
			}
			fmt.Fprintln(&b)
			if desc := p.ActDescriptions[act]; desc != "" {
				fmt.Fprintf(&b, "  %s\n", desc)
			}
		}
		fmt.Fprintln(&b)
	}

	if len(p.DefinedChapters) > 0 {
		fmt.Fprintf(&b, "## Chapters drafted\n\n")
		for _, ch := range p.DefinedChapters {
			label := p.ChapterLabels[ch]
			scenesIn := len(scenesByChapter[ch])
			words := wordsByChapter[ch]
			head := fmt.Sprintf("Chapter %d", ch)
			if label != "" {
				head = fmt.Sprintf("Chapter %d: %s", ch, label)
			}
			if scenesIn == 0 {
				fmt.Fprintf(&b, "- %s — not yet drafted\n", head)
			} else {
				fmt.Fprintf(&b, "- %s — %d scene(s), ~%d words\n", head, scenesIn, words)
			}
		}
		fmt.Fprintln(&b)
	}

	return b.String()
}

// ─── Frontmatter / wikilink utilities ─────────────────────────────────────

var frontmatterRE = regexp.MustCompile(`(?s)^---\r?\n(.*?)\r?\n---\r?\n?(.*)$`)

// extractFrontmatter parses a markdown file with YAML frontmatter. Returns
// (frontmatterMap, body). On any parse error or missing frontmatter, returns
// (nil, content).
func extractFrontmatter(content string) (map[string]any, string) {
	m := frontmatterRE.FindStringSubmatch(content)
	if m == nil {
		return nil, content
	}
	var fm map[string]any
	if err := yaml.Unmarshal([]byte(m[1]), &fm); err != nil {
		return nil, content
	}
	return fm, m[2]
}

// wikilinkRE matches `[[anything-not-]]]`. Captures the inner text.
var wikilinkRE = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

// stripWikilinks replaces every `[[Link]]` with its display text. Mirrors
// ExportService.stripWikiLinks:
//   - `[[Alias|Display]]` → `Display`
//   - `[[Path/To/Note]]`  → `Note` (last path segment)
//   - `[[Simple]]`        → `Simple`
func stripWikilinks(text string) string {
	return wikilinkRE.ReplaceAllStringFunc(text, func(match string) string {
		inner := match[2 : len(match)-2]
		if i := strings.LastIndex(inner, "|"); i >= 0 {
			return strings.TrimSpace(inner[i+1:])
		}
		if i := strings.LastIndex(inner, "/"); i >= 0 {
			return strings.TrimSpace(inner[i+1:])
		}
		return strings.TrimSpace(inner)
	})
}

// cleanWikilink turns a wikilink-wrapped name into a plain entity name,
// mirroring MetadataParser.cleanWikilink. Handles `[[Name]]`, `[[Name|Alias]]`,
// `[[Path/To/Name]]`, `[[Name#heading]]`, and YAML-quoted variants.
func cleanWikilink(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Strip stray YAML quotes.
	if (strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`)) ||
		(strings.HasPrefix(s, `'`) && strings.HasSuffix(s, `'`)) {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	if !strings.HasPrefix(s, "[[") || !strings.HasSuffix(s, "]]") {
		return s
	}
	inner := s[2 : len(s)-2]
	if i := strings.Index(inner, "|"); i >= 0 {
		return strings.TrimSpace(inner[i+1:])
	}
	if i := strings.Index(inner, "#"); i >= 0 {
		inner = inner[:i]
	}
	if i := strings.LastIndex(inner, "/"); i >= 0 {
		inner = inner[i+1:]
	}
	return strings.TrimSpace(inner)
}

// cleanWikilinks normalizes a frontmatter value that may be a string, a list,
// or nil into a deduplicated list of cleaned entity names.
func cleanWikilinks(v any) []string {
	if v == nil {
		return nil
	}
	var raw []any
	switch x := v.(type) {
	case []any:
		raw = x
	case string:
		raw = []any{x}
	default:
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	for _, item := range raw {
		s := cleanWikilink(coerceString(item))
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// ─── YAML coercion helpers ────────────────────────────────────────────────

func coerceString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case int:
		return fmt.Sprintf("%d", x)
	case int64:
		return fmt.Sprintf("%d", x)
	case float64:
		return fmt.Sprintf("%g", x)
	case bool:
		return fmt.Sprintf("%t", x)
	default:
		return fmt.Sprintf("%v", x)
	}
}

func coerceInt(v any) int {
	return coerceIntDefault(v, 0)
}

func coerceIntDefault(v any, def int) int {
	switch x := v.(type) {
	case nil:
		return def
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case string:
		var n int
		if _, err := fmt.Sscanf(strings.TrimSpace(x), "%d", &n); err == nil {
			return n
		}
		return def
	default:
		return def
	}
}

func coerceIntSlice(v any) []int {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	var out []int
	for _, item := range arr {
		out = append(out, coerceInt(item))
	}
	return out
}

// coerceIntKeyedStringMap accepts YAML maps with either integer or string
// keys and returns map[int]string. Used for definedActs labels etc.
func coerceIntKeyedStringMap(v any) map[int]string {
	out := make(map[int]string)
	switch m := v.(type) {
	case map[string]any:
		for k, val := range m {
			var n int
			if _, err := fmt.Sscanf(k, "%d", &n); err == nil {
				out[n] = coerceString(val)
			}
		}
	case map[any]any:
		for k, val := range m {
			out[coerceInt(k)] = coerceString(val)
		}
	}
	return out
}

// PageCount returns the number of pages for a text (words / 300, rounded up).
func PageCount(text string) int {
	words := len(strings.Fields(text))
	pages := words / 300
	if words%300 > 0 {
		pages++
	}
	return pages
}

// TotalWordCount sums scene wordcounts. Falls back to counting words in
// bodies if frontmatter wordcount is absent.
func TotalWordCount(scenes []Scene) int {
	total := 0
	for _, s := range scenes {
		if s.Wordcount > 0 {
			total += s.Wordcount
		} else {
			total += len(strings.Fields(s.Body))
		}
	}
	return total
}
