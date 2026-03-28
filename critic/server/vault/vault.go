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

// NamedChapter pairs a chapter name with its content.
type NamedChapter struct {
	Name    string
	Content string
}

// WriteReview writes a review file to review/ with a timestamped filename.
// Returns the path relative to the vault root.
func (v *Vault) WriteReview(prefix string, content string) (string, error) {
	dir := filepath.Join(v.Root, "review")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	timestamp := time.Now().Format("2006-01-02-150405")
	filename := prefix + "-" + timestamp + ".md"
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	rel, _ := filepath.Rel(v.Root, path)
	return rel, nil
}

// ReadLatestReview reads the most recent review file matching the given prefix.
// Returns empty string if no prior review exists.
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
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() && strings.HasPrefix(name, prefix+"-") && strings.HasSuffix(name, ".md") {
			if name > latest {
				latest = name
			}
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
