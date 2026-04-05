package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Vault provides access to the Obsidian vault structure.
type Vault struct {
	Root string
}

func New(root string) *Vault {
	return &Vault{Root: root}
}

// ReadChapter reads a chapter file by name (e.g., "chapter-01" or "chapter-01.md").
func (v *Vault) ReadChapter(name string) (string, error) {
	if !strings.HasSuffix(name, ".md") {
		name = name + ".md"
	}
	path := filepath.Join(v.Root, "story", name)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read chapter %s: %w", name, err)
	}
	return string(data), nil
}

// ReadPriorChapters reads up to n chapters before the given chapter.
func (v *Vault) ReadPriorChapters(name string, n int) ([]string, error) {
	storyDir := filepath.Join(v.Root, "story")
	entries, err := os.ReadDir(storyDir)
	if err != nil {
		return nil, fmt.Errorf("read story directory: %w", err)
	}

	if !strings.HasSuffix(name, ".md") {
		name = name + ".md"
	}

	var chapters []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			chapters = append(chapters, e.Name())
		}
	}
	sort.Strings(chapters)

	idx := -1
	for i, ch := range chapters {
		if ch == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, nil
	}

	start := idx - n
	if start < 0 {
		start = 0
	}

	var result []string
	for i := start; i < idx; i++ {
		data, err := os.ReadFile(filepath.Join(storyDir, chapters[i]))
		if err != nil {
			return nil, fmt.Errorf("read prior chapter %s: %w", chapters[i], err)
		}
		result = append(result, string(data))
	}
	return result, nil
}

// ReadCanonFiles reads all files in the world/ directory tree.
func (v *Vault) ReadCanonFiles() (map[string]string, error) {
	worldDir := filepath.Join(v.Root, "world")
	files := make(map[string]string)

	err := filepath.Walk(worldDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(v.Root, path)
		files[rel] = string(data)
		return nil
	})
	return files, err
}

// ReadCanonForEntities reads canon files whose content mentions any of the given entity names.
func (v *Vault) ReadCanonForEntities(entities []string) (map[string]string, error) {
	allFiles, err := v.ReadCanonFiles()
	if err != nil {
		return nil, err
	}

	matched := make(map[string]string)
	for path, content := range allFiles {
		lower := strings.ToLower(content)
		for _, entity := range entities {
			if strings.Contains(lower, strings.ToLower(entity)) {
				matched[path] = content
				break
			}
		}
	}
	return matched, nil
}

// ReadPlotFiles reads all files in the plot/ directory tree.
func (v *Vault) ReadPlotFiles() (map[string]string, error) {
	plotDir := filepath.Join(v.Root, "plot")
	files := make(map[string]string)

	err := filepath.Walk(plotDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
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
		files[rel] = string(data)
		return nil
	})
	return files, err
}

// ReadAllChapters reads all chapter files in order.
func (v *Vault) ReadAllChapters() ([]NamedChapter, error) {
	storyDir := filepath.Join(v.Root, "story")
	entries, err := os.ReadDir(storyDir)
	if err != nil {
		return nil, fmt.Errorf("read story directory: %w", err)
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	var chapters []NamedChapter
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(storyDir, name))
		if err != nil {
			return nil, fmt.Errorf("read chapter %s: %w", name, err)
		}
		chapters = append(chapters, NamedChapter{
			Name:    strings.TrimSuffix(name, ".md"),
			Content: string(data),
		})
	}
	return chapters, nil
}

// ReadChaptersFrom reads all chapters starting from the named chapter (inclusive).
func (v *Vault) ReadChaptersFrom(name string) ([]NamedChapter, error) {
	all, err := v.ReadAllChapters()
	if err != nil {
		return nil, err
	}

	if !strings.HasSuffix(name, ".md") {
		name = name + ".md"
	}
	baseName := strings.TrimSuffix(name, ".md")

	for i, ch := range all {
		if ch.Name == baseName {
			return all[i:], nil
		}
	}
	return nil, fmt.Errorf("chapter %s not found", baseName)
}

// ReadPriorSummaries reads summaries for all chapters before the given chapter.
// Returns them in order. Missing summaries are skipped.
func (v *Vault) ReadPriorSummaries(name string) ([]NamedChapter, error) {
	if !strings.HasSuffix(name, ".md") {
		name = name + ".md"
	}
	baseName := strings.TrimSuffix(name, ".md")

	chapters, err := v.ListChapterNames()
	if err != nil {
		return nil, err
	}

	var summaries []NamedChapter
	for _, ch := range chapters {
		if ch == baseName {
			break
		}
		path := filepath.Join(v.Root, "summary", ch+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			continue // no summary for this chapter, skip
		}
		summaries = append(summaries, NamedChapter{Name: ch, Content: string(data)})
	}
	return summaries, nil
}

// NamedChapter pairs a chapter name with its content.
type NamedChapter struct {
	Name    string
	Content string
}

// Sentinel separates the synthesis (which reviewers see) from the raw agent
// outputs (which are retained for traceability but not fed to future reviews).
const ReviewSentinel = "\n\n<!-- RAW AGENT OUTPUTS BELOW — NOT INCLUDED IN FUTURE REVIEW CONTEXT -->\n\n"

// WriteReview writes a review file to review/ with a globally sequential number.
// Format: NNN-prefix-timestamp.md (e.g., 003-manuscript-critic-2026-04-03-104209.md)
// Returns the relative path and the review number.
func (v *Vault) WriteReview(prefix string, content string) (string, int, error) {
	dir := filepath.Join(v.Root, "review")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, err
	}
	num := v.NextReviewNumber()
	timestamp := time.Now().Format("2006-01-02-150405")
	filename := fmt.Sprintf("%03d-%s-%s.md", num, prefix, timestamp)
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", 0, err
	}
	rel, _ := filepath.Rel(v.Root, path)
	return rel, num, nil
}

// NextReviewNumber scans all files in review/ for the highest leading number
// and returns the next one. The sequence is global across all review types.
func (v *Vault) NextReviewNumber() int {
	dir := filepath.Join(v.Root, "review")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 1
	}
	maxNum := 0
	for _, e := range entries {
		name := e.Name()
		// Parse NNN from NNN-prefix-timestamp.md
		var n int
		if _, err := fmt.Sscanf(name, "%d-", &n); err == nil && n > maxNum {
			maxNum = n
		}
	}
	return maxNum + 1
}

// ReadLatestReviewSynthesis reads the most recent review file matching a prefix,
// but only returns the content above the sentinel (the synthesis + rebuttals).
func (v *Vault) ReadLatestReviewSynthesis(prefix string) (string, error) {
	content, err := v.ReadLatestReview(prefix)
	if err != nil || content == "" {
		return content, err
	}
	if idx := strings.Index(content, ReviewSentinel); idx >= 0 {
		return content[:idx], nil
	}
	return content, nil
}

// ReadReviewByNumber reads a review file by its global sequence number.
func (v *Vault) ReadReviewByNumber(num int) (string, string, error) {
	dir := filepath.Join(v.Root, "review")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", fmt.Errorf("read review directory: %w", err)
	}
	target := fmt.Sprintf("%03d-", num)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), target) {
			path := filepath.Join(dir, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				return "", "", err
			}
			return string(data), e.Name(), nil
		}
	}
	return "", "", fmt.Errorf("review #%03d not found", num)
}

// WriteReviewFile overwrites a review file by name.
func (v *Vault) WriteReviewFile(filename string, content string) error {
	path := filepath.Join(v.Root, "review", filename)
	return os.WriteFile(path, []byte(content), 0o644)
}

// ReadLatestReview reads the most recent review file containing the given prefix.
// Files are NNN-prefix-timestamp.md; we find the highest NNN that contains the prefix.
func (v *Vault) ReadLatestReview(prefix string) (string, error) {
	dir := filepath.Join(v.Root, "review")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	var latest string
	var latestNum int
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		// Check if this file contains the prefix after the NNN-
		if !strings.Contains(name, "-"+prefix+"-") {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(name, "%d-", &n); err == nil && n > latestNum {
			latestNum = n
			latest = name
		}
	}

	if latest == "" {
		return "", nil
	}

	data, err := os.ReadFile(filepath.Join(dir, latest))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// PageCount returns the number of pages for a text (words / 300).
func PageCount(text string) int {
	words := len(strings.Fields(text))
	pages := words / 300
	if words%300 > 0 {
		pages++
	}
	return pages
}

// TotalPageCount counts pages across all chapters in the vault.
func (v *Vault) TotalPageCount() int {
	chapters, err := v.ReadAllChapters()
	if err != nil {
		return 0
	}
	totalWords := 0
	for _, ch := range chapters {
		totalWords += len(strings.Fields(ch.Content))
	}
	pages := totalWords / 300
	if totalWords%300 > 0 {
		pages++
	}
	return pages
}

// WriteSummary writes a chapter summary to summary/<chapter-name>.md.
func (v *Vault) WriteSummary(chapterName string, content string) error {
	dir := filepath.Join(v.Root, "summary")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if !strings.HasSuffix(chapterName, ".md") {
		chapterName = chapterName + ".md"
	}
	return os.WriteFile(filepath.Join(dir, chapterName), []byte(content), 0o644)
}

// ListChapterNames returns the sorted list of chapter filenames (without .md extension).
func (v *Vault) ListChapterNames() ([]string, error) {
	storyDir := filepath.Join(v.Root, "story")
	entries, err := os.ReadDir(storyDir)
	if err != nil {
		return nil, fmt.Errorf("read story directory: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			names = append(names, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	sort.Strings(names)
	return names, nil
}

// ReadIssues reads the issues.md file from the vault root, if it exists.
func (v *Vault) ReadIssues() string {
	data, err := os.ReadFile(filepath.Join(v.Root, "issues.md"))
	if err != nil {
		return ""
	}
	return string(data)
}

// AppendIssue appends an issue entry under a heading in issues.md.
// Creates the file and heading if they don't exist.
func (v *Vault) AppendIssue(heading, entry string) error {
	path := filepath.Join(v.Root, "issues.md")
	existing, _ := os.ReadFile(path)
	content := string(existing)

	headingLine := "## " + heading
	if content == "" {
		content = "# Known Issues\n\nIssues acknowledged but deferred for later resolution. Reviewers: only re-raise these if the issue has escalated in importance.\n\n"
	}

	if !strings.Contains(content, headingLine) {
		content = strings.TrimRight(content, "\n") + "\n\n" + headingLine + "\n\n"
	}

	// Insert the entry after the heading
	idx := strings.Index(content, headingLine)
	afterHeading := idx + len(headingLine)
	// Find the end of this section (next ## or end of file)
	rest := content[afterHeading:]
	nextSection := strings.Index(rest[1:], "\n## ")
	var insertAt int
	if nextSection >= 0 {
		insertAt = afterHeading + 1 + nextSection
	} else {
		insertAt = len(content)
	}

	newContent := content[:insertAt] + "\n" + entry + "\n" + content[insertAt:]
	return os.WriteFile(path, []byte(strings.TrimRight(newContent, "\n")+"\n"), 0o644)
}

// ReadStyleGuide reads the style.md file from the vault root, if it exists.
func (v *Vault) ReadStyleGuide() string {
	data, err := os.ReadFile(filepath.Join(v.Root, "style.md"))
	if err != nil {
		return ""
	}
	return string(data)
}

// ReadReviewerMemory reads a reviewer's memory file.
func (v *Vault) ReadReviewerMemory(role string) (string, error) {
	path := filepath.Join(v.Root, "system", "reviewer-memory", role+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// WriteReviewerMemory writes a reviewer's memory file.
func (v *Vault) WriteReviewerMemory(role string, content string) error {
	dir := filepath.Join(v.Root, "system", "reviewer-memory")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, role+".md"), []byte(content), 0o644)
}
