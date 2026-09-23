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

// Layout of a book project. Root is the folder the user configures as the
// vault; everything critic reads or writes lives under it.
//
//	<Title>.md    optional book note (frontmatter `type: book`, `title`, `acts`)
//	Story/        one .md per chapter; scenes are `## <title>` sections
//	Background/   worldbuilding docs, plus Characters/ and Locations/ entries
//	Review/       saved reviews, .snapshots/, .staging/, close-read/
//	stage.md, style.md, issues.md, summary/, prompts/   (all optional)
const (
	StoryDir      = "Story"
	BackgroundDir = "Background"
	ReviewDir     = "Review"
)

// entryDirs hold one file per character or location. The rest of Background/
// is free-form worldbuilding.
var entryDirs = []string{
	filepath.Join(BackgroundDir, "Characters"),
	filepath.Join(BackgroundDir, "Locations"),
}

// Vault is a book project rooted at Root.
type Vault struct {
	Root string
}

// Book is the project-level metadata from the optional book note.
type Book struct {
	Title       string
	Description string
	ActLabels   map[int]string
}

// Chapter is one file under Story/.
type Chapter struct {
	Path       string
	Filename   string // basename without .md
	Number     int
	Act        int // 0 when the book doesn't use acts
	Title      string
	Status     string
	POV        []string // cleaned wikilinks
	Characters []string // cleaned wikilinks
	Locations  []string // cleaned wikilinks
	Preamble   string   // prose before the first scene heading, if any
	Scenes     []Scene
}

// Scene is one `## <title>` section of a chapter file. A chapter with no
// scene headings is a single untitled scene.
type Scene struct {
	Chapter      int
	ChapterTitle string
	Index        int // 1-based position within the chapter
	Title        string
	POV          string   // cleaned wikilink
	Characters   []string // cleaned wikilinks
	Location     string   // cleaned wikilink
	Body         string   // trimmed; wikilinks intact
}

// ID is the scene's address, `CC-SS`. It matches the filename prefixes of the
// storyline layout this replaced, so older reviews and notes still resolve.
func (s Scene) ID() string { return fmt.Sprintf("%02d-%02d", s.Chapter, s.Index) }

// New opens the book project at root. The only structural requirement is a
// Story/ folder; everything else is optional.
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
	story, err := os.Stat(filepath.Join(root, StoryDir))
	if err != nil || !story.IsDir() {
		return nil, fmt.Errorf("no %s/ folder in %s; critic expects one file per chapter under %s/", StoryDir, root, StoryDir)
	}
	return &Vault{Root: root}, nil
}

// ReadBook loads the book note: the single root-level .md whose frontmatter
// has `type: book`. Without one, the title falls back to the root folder's
// name and no act labels are known.
func (v *Vault) ReadBook() (*Book, error) {
	entries, err := os.ReadDir(v.Root)
	if err != nil {
		return nil, fmt.Errorf("read vault root: %w", err)
	}
	var matches []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(v.Root, e.Name()))
		if err != nil {
			continue
		}
		if fm, _ := extractFrontmatter(string(data)); fm != nil && coerceString(fm["type"]) == "book" {
			matches = append(matches, e.Name())
		}
	}

	switch len(matches) {
	case 0:
		return &Book{Title: filepath.Base(v.Root), ActLabels: map[int]string{}}, nil
	case 1:
	default:
		return nil, fmt.Errorf("multiple book notes (`type: book`) in %s: %s", v.Root, strings.Join(matches, ", "))
	}

	data, err := os.ReadFile(filepath.Join(v.Root, matches[0]))
	if err != nil {
		return nil, fmt.Errorf("read book note: %w", err)
	}
	fm, body := extractFrontmatter(string(data))
	b := &Book{
		Title:       coerceString(fm["title"]),
		Description: strings.TrimSpace(body),
		ActLabels:   coerceIntKeyedStringMap(fm["acts"]),
	}
	if b.Title == "" {
		b.Title = strings.TrimSuffix(matches[0], ".md")
	}
	return b, nil
}

// ReadChapters loads every chapter under Story/, sorted by chapter number.
// Numbering is global across acts; `act:` only groups chapters under act
// headings. A file is a chapter if its frontmatter sets `chapter:` or its
// filename starts with a number; other notes in Story/ are ignored. Two files
// claiming the same number is an error, because keeping either one silently
// would drop the other from every review.
func (v *Vault) ReadChapters() ([]Chapter, error) {
	dir := filepath.Join(v.Root, StoryDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read %s/: %w", StoryDir, err)
	}

	var chapters []Chapter
	claimed := make(map[int]string)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		ch, ok := parseChapter(path, string(data))
		if !ok {
			continue
		}
		if prev, dup := claimed[ch.Number]; dup {
			return nil, fmt.Errorf("chapter %d is claimed by both %q and %q", ch.Number, prev, e.Name())
		}
		claimed[ch.Number] = e.Name()
		chapters = append(chapters, ch)
	}

	sort.SliceStable(chapters, func(i, j int) bool { return chapters[i].Number < chapters[j].Number })
	return chapters, nil
}

var leadingNumberRE = regexp.MustCompile(`^(\d+)\s*(.*)$`)

// sceneHeadingRE matches a scene heading. Exactly two hashes: `#` is free
// for a chapter-level title and `###`+ stay inside the prose.
var sceneHeadingRE = regexp.MustCompile(`^## (.+?)\s*$`)

type sceneMeta struct {
	title      string
	pov        string
	characters []string
	location   string
	used       bool
}

func parseChapter(path, content string) (Chapter, bool) {
	filename := strings.TrimSuffix(filepath.Base(path), ".md")
	fm, body := extractFrontmatter(content)
	if fm == nil {
		fm = map[string]any{}
	}

	ch := Chapter{Path: path, Filename: filename}
	fileNum := leadingNumberRE.FindStringSubmatch(filename)
	if _, set := fm["chapter"]; set {
		ch.Number = coerceIntDefault(fm["chapter"], -1)
	} else if fileNum != nil {
		ch.Number = coerceIntDefault(fileNum[1], -1)
	} else {
		return Chapter{}, false
	}
	if ch.Number < 0 {
		return Chapter{}, false
	}

	ch.Title = strings.TrimSpace(coerceString(fm["title"]))
	if _, set := fm["title"]; !set && fileNum != nil {
		ch.Title = strings.TrimSpace(fileNum[2])
	}
	ch.Act = coerceInt(fm["act"])
	ch.Status = coerceString(fm["status"])
	ch.POV = cleanWikilinks(fm["pov"])
	ch.Characters = cleanWikilinks(fm["characters"])
	ch.Locations = cleanWikilinks(fm["locations"])

	var metas []*sceneMeta
	if list, ok := fm["scenes"].([]any); ok {
		for _, item := range list {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			metas = append(metas, &sceneMeta{
				title:      strings.TrimSpace(coerceString(m["title"])),
				pov:        cleanWikilink(coerceString(m["pov"])),
				characters: cleanWikilinks(m["characters"]),
				location:   cleanWikilink(coerceString(m["location"])),
			})
		}
	}

	type section struct {
		title string
		lines []string
	}
	var preamble []string
	var sections []*section
	for _, line := range strings.Split(body, "\n") {
		if m := sceneHeadingRE.FindStringSubmatch(line); m != nil {
			sections = append(sections, &section{title: m[1]})
			continue
		}
		if len(sections) == 0 {
			preamble = append(preamble, line)
		} else {
			cur := sections[len(sections)-1]
			cur.lines = append(cur.lines, line)
		}
	}
	if len(sections) == 0 {
		// No scene headings: the whole chapter is one untitled scene.
		sections = []*section{{lines: preamble}}
		preamble = nil
	}
	ch.Preamble = strings.TrimSpace(strings.Join(preamble, "\n"))

	// Frontmatter scene metadata pairs with body sections by title, falling
	// back to position, so reordering scenes in the body doesn't misattribute
	// POV or cast.
	for i, sec := range sections {
		s := Scene{
			Chapter:      ch.Number,
			ChapterTitle: ch.Title,
			Index:        i + 1,
			Title:        sec.title,
			Body:         strings.TrimSpace(strings.Join(sec.lines, "\n")),
		}
		var meta *sceneMeta
		for _, m := range metas {
			if !m.used && m.title != "" && strings.EqualFold(m.title, sec.title) {
				meta = m
				break
			}
		}
		if meta == nil && i < len(metas) && !metas[i].used {
			meta = metas[i]
		}
		if meta != nil {
			meta.used = true
			s.POV = meta.pov
			s.Characters = meta.characters
			s.Location = meta.location
		}
		if s.POV == "" && len(ch.POV) == 1 {
			s.POV = ch.POV[0]
		}
		ch.Scenes = append(ch.Scenes, s)
	}
	return ch, true
}

// AllScenes flattens chapters into manuscript-ordered scenes.
func AllScenes(chapters []Chapter) []Scene {
	var out []Scene
	for _, ch := range chapters {
		out = append(out, ch.Scenes...)
	}
	return out
}

// FindChapter returns the chapter with the given number.
func FindChapter(chapters []Chapter, number int) (*Chapter, bool) {
	for i := range chapters {
		if chapters[i].Number == number {
			return &chapters[i], true
		}
	}
	return nil, false
}

var sceneRefRE = regexp.MustCompile(`^(\d+)\s*[-.]\s*(\d+)\b`)

// FindScene resolves a scene reference: an ID (`04-02`, `4-2`, `4.2`,
// optionally followed by a title, as in the old scene filenames) or an exact,
// case-insensitive scene title.
func FindScene(chapters []Chapter, ref string) (*Scene, error) {
	ref = strings.TrimSuffix(strings.TrimSpace(ref), ".md")
	if ref == "" {
		return nil, fmt.Errorf("scene reference is empty")
	}
	if m := sceneRefRE.FindStringSubmatch(ref); m != nil {
		chNum, idx := coerceInt(m[1]), coerceInt(m[2])
		ch, ok := FindChapter(chapters, chNum)
		if !ok {
			return nil, fmt.Errorf("no chapter %d", chNum)
		}
		if idx < 1 || idx > len(ch.Scenes) {
			return nil, fmt.Errorf("chapter %d has %d scene(s); no scene %d", chNum, len(ch.Scenes), idx)
		}
		return &ch.Scenes[idx-1], nil
	}

	var hits []*Scene
	for ci := range chapters {
		for si := range chapters[ci].Scenes {
			if strings.EqualFold(chapters[ci].Scenes[si].Title, ref) {
				hits = append(hits, &chapters[ci].Scenes[si])
			}
		}
	}
	switch len(hits) {
	case 0:
		return nil, fmt.Errorf("no scene titled %q; use an ID like 04-02 (see list-scenes)", ref)
	case 1:
		return hits[0], nil
	default:
		ids := make([]string, len(hits))
		for i, h := range hits {
			ids[i] = h.ID()
		}
		return nil, fmt.Errorf("scene title %q is ambiguous: %s", ref, strings.Join(ids, ", "))
	}
}

// AssembleManuscript renders the book as `# Title`, `## Act N: <label>`,
// `### Chapter N: <title>`, `#### <scene title>`, body. This is the storyline
// plugin's export shape, kept so snapshots taken before and after the move
// off storyline diff cleanly. Bodies have wikilinks stripped to display text.
func (v *Vault) AssembleManuscript(b *Book, chapters []Chapter) string {
	var out strings.Builder
	fmt.Fprintf(&out, "# %s\n\n", b.Title)

	currentAct := -1
	for _, ch := range chapters {
		if ch.Act != currentAct {
			currentAct = ch.Act
			if ch.Act > 0 {
				if label := b.ActLabels[ch.Act]; label != "" {
					fmt.Fprintf(&out, "## Act %d: %s\n\n", ch.Act, label)
				} else {
					fmt.Fprintf(&out, "## Act %d\n\n", ch.Act)
				}
			}
		}
		out.WriteString(RenderChapter(ch))
	}
	return out.String()
}

// RenderChapter renders one chapter: heading, any preamble, then each scene.
func RenderChapter(ch Chapter) string {
	var out strings.Builder
	if ch.Title != "" {
		fmt.Fprintf(&out, "### Chapter %d: %s\n\n", ch.Number, ch.Title)
	} else {
		fmt.Fprintf(&out, "### Chapter %d\n\n", ch.Number)
	}
	if ch.Preamble != "" {
		out.WriteString(stripWikilinks(ch.Preamble))
		out.WriteString("\n\n")
	}
	for _, s := range ch.Scenes {
		writeScene(&out, s)
	}
	return out.String()
}

// RenderScene renders one scene: `#### <title>` (omitted when untitled) and
// its body.
func RenderScene(s Scene) string {
	var out strings.Builder
	writeScene(&out, s)
	return strings.TrimRight(out.String(), "\n") + "\n"
}

func writeScene(out *strings.Builder, s Scene) {
	if s.Title != "" {
		fmt.Fprintf(out, "#### %s\n\n", s.Title)
	}
	if body := stripWikilinks(s.Body); body != "" {
		out.WriteString(body)
	} else {
		out.WriteString("*No content yet.*")
	}
	out.WriteString("\n\n")
}

// ReadManuscript loads the book and chapters and assembles them.
func (v *Vault) ReadManuscript() (string, error) {
	b, err := v.ReadBook()
	if err != nil {
		return "", err
	}
	chapters, err := v.ReadChapters()
	if err != nil {
		return "", err
	}
	if len(chapters) == 0 {
		return "", fmt.Errorf("no chapters found in %s/", StoryDir)
	}
	return v.AssembleManuscript(b, chapters), nil
}

// ReadResearchFiles reads the worldbuilding docs: every .md under
// Background/ except the Characters/ and Locations/ entries. Keyed by
// vault-relative path.
func (v *Vault) ReadResearchFiles() (map[string]string, error) {
	skip := make(map[string]bool, len(entryDirs))
	for _, d := range entryDirs {
		skip[filepath.Join(v.Root, d)] = true
	}
	result := make(map[string]string)
	err := filepath.Walk(filepath.Join(v.Root, BackgroundDir), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if skip[path] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
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

// ListCodexEntries returns the name (filename without .md) of every entry in
// Background/Characters/ and Background/Locations/.
func (v *Vault) ListCodexEntries() ([]string, error) {
	var names []string
	for _, sub := range entryDirs {
		entries, err := os.ReadDir(filepath.Join(v.Root, sub))
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

// ReadCodexEntry reads one entry by name (filename without .md), searching
// Characters/ then Locations/. Returns os.ErrNotExist if absent.
func (v *Vault) ReadCodexEntry(name string) (string, error) {
	name = strings.TrimSuffix(name, ".md")
	for _, sub := range entryDirs {
		data, err := os.ReadFile(filepath.Join(v.Root, sub, name+".md"))
		if err == nil {
			return string(data), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
	}
	return "", os.ErrNotExist
}

// ReadCodexEntries reads entries for the named entities, matched
// case-insensitively against filenames. Empty `names` returns every entry.
// Keyed by vault-relative path.
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
	for _, sub := range entryDirs {
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
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			result[filepath.Join(sub, e.Name())] = string(data)
		}
	}
	return result, nil
}

// SceneEntityNames returns the union of POV, character, and location names
// on the given scenes. Used to prefilter Background entries for reviewers.
func SceneEntityNames(scenes []Scene) []string {
	var u nameSet
	for _, s := range scenes {
		u.add(s.POV, s.Location)
		u.add(s.Characters...)
	}
	return u.sorted()
}

// ChapterEntityNames is SceneEntityNames plus the chapter-level lists, which
// may name people or places no single scene's metadata does.
func ChapterEntityNames(ch Chapter) []string {
	var u nameSet
	u.add(ch.POV...)
	u.add(ch.Characters...)
	u.add(ch.Locations...)
	u.add(SceneEntityNames(ch.Scenes)...)
	return u.sorted()
}

type nameSet map[string]bool

func (u *nameSet) add(names ...string) {
	if *u == nil {
		*u = make(nameSet)
	}
	for _, n := range names {
		if n = strings.TrimSpace(n); n != "" {
			(*u)[n] = true
		}
	}
}

func (u nameSet) sorted() []string {
	out := make([]string, 0, len(u))
	for n := range u {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// ReadStyleGuide returns style.md from the project root, falling back to
// Background/style.md. Empty if neither exists.
func (v *Vault) ReadStyleGuide() string {
	for _, rel := range []string{"style.md", filepath.Join(BackgroundDir, "style.md")} {
		if data, err := os.ReadFile(filepath.Join(v.Root, rel)); err == nil {
			return string(data)
		}
	}
	return ""
}

// ReadStage reads stage.md from the project root, if present. Authors
// override the derived stage block by writing this file.
func (v *Vault) ReadStage() string {
	data, err := os.ReadFile(filepath.Join(v.Root, "stage.md"))
	if err != nil {
		return ""
	}
	return string(data)
}

// DerivedStage describes the draft from the book note and chapters. Used
// when stage.md is absent.
func (v *Vault) DerivedStage(b *Book, chapters []Chapter) string {
	var out strings.Builder
	fmt.Fprintf(&out, "# Current Stage\n\n")
	fmt.Fprintf(&out, "Project: %s\n", b.Title)
	if b.Description != "" {
		fmt.Fprintf(&out, "\n%s\n", b.Description)
	}

	scenes, words := 0, 0
	for _, ch := range chapters {
		scenes += len(ch.Scenes)
		words += ChapterWordCount(ch)
	}
	fmt.Fprintf(&out, "\nDrafted so far: %d chapter(s), %d scene(s), ~%d words.\n\n", len(chapters), scenes, words)

	if len(chapters) > 0 {
		fmt.Fprintf(&out, "## Chapters drafted\n\n")
		for _, ch := range chapters {
			head := fmt.Sprintf("Chapter %d", ch.Number)
			if ch.Title != "" {
				head += ": " + ch.Title
			}
			if ch.Act > 0 {
				if label := b.ActLabels[ch.Act]; label != "" {
					head = fmt.Sprintf("Act %d (%s), %s", ch.Act, label, head)
				} else {
					head = fmt.Sprintf("Act %d, %s", ch.Act, head)
				}
			}
			fmt.Fprintf(&out, "- %s: %d scene(s), ~%d words", head, len(ch.Scenes), ChapterWordCount(ch))
			if ch.Status != "" {
				fmt.Fprintf(&out, ", status %s", ch.Status)
			}
			fmt.Fprintln(&out)
		}
		fmt.Fprintln(&out)
	}
	return out.String()
}

// WordCount counts whitespace-separated words.
func WordCount(text string) int { return len(strings.Fields(text)) }

// ChapterWordCount counts the chapter's prose: preamble plus scene bodies,
// not headings.
func ChapterWordCount(ch Chapter) int {
	n := WordCount(ch.Preamble)
	for _, s := range ch.Scenes {
		n += WordCount(s.Body)
	}
	return n
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

// stripWikilinks replaces every `[[Link]]` with its display text:
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

// cleanWikilink turns a wikilink-wrapped name into a plain entity name.
// Handles `[[Name]]`, `[[Name|Alias]]`, `[[Path/To/Name]]`, `[[Name#heading]]`,
// and YAML-quoted variants.
func cleanWikilink(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
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

// coerceIntKeyedStringMap accepts YAML maps with either integer or string
// keys and returns map[int]string. yaml.v3 decodes an all-integer-keyed
// mapping as map[any]any, so both shapes occur in practice.
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
