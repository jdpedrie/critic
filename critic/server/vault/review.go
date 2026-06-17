package vault

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ReviewSentinel separates the synthesis (which reviewers see) from the raw
// agent outputs (which are retained for traceability but not fed to future
// reviews).
const ReviewSentinel = "\n\n<!-- RAW AGENT OUTPUTS BELOW — NOT INCLUDED IN FUTURE REVIEW CONTEXT -->\n\n"

// WriteReview writes a review file to review/ with a globally sequential
// number. Format: NNN-prefix-timestamp.md.
func (v *Vault) WriteReview(prefix, content string) (relPath string, num int, err error) {
	dir := filepath.Join(v.Root, "review")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, err
	}
	num = v.NextReviewNumber()
	timestamp := time.Now().Format("2006-01-02-150405")
	filename := fmt.Sprintf("%03d-%s-%s.md", num, prefix, timestamp)
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", 0, err
	}
	rel, _ := filepath.Rel(v.Root, path)
	return rel, num, nil
}

// NextReviewNumber scans review/ for the highest leading NNN and returns
// the next sequential value. Global across all review-type prefixes.
func (v *Vault) NextReviewNumber() int {
	dir := filepath.Join(v.Root, "review")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 1
	}
	maxNum := 0
	for _, e := range entries {
		var n int
		if _, err := fmt.Sscanf(e.Name(), "%d-", &n); err == nil && n > maxNum {
			maxNum = n
		}
	}
	return maxNum + 1
}

// ReadLatestReview returns the content of the most recent review file
// matching the prefix. Empty string if none.
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

// ReadLatestReviewSynthesis returns only the content above the sentinel.
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

// ReadReviewByNumber loads a review by its global sequence number.
func (v *Vault) ReadReviewByNumber(num int) (content, filename string, err error) {
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

// WriteReviewFile overwrites a review file by name (filename only, no dir).
func (v *Vault) WriteReviewFile(filename, content string) error {
	return os.WriteFile(filepath.Join(v.Root, "review", filename), []byte(content), 0o644)
}

// WriteStagedPart writes a named part to review/.staging/ for later assembly.
func (v *Vault) WriteStagedPart(name, content string) error {
	dir := filepath.Join(v.Root, "review", ".staging")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
}

// AssembleReview reads staged parts, assembles them with the sentinel, writes
// the final review file, and cleans up staging.
func (v *Vault) AssembleReview(prefix, synthesisKey string, partKeys []string) (relPath string, num int, err error) {
	dir := filepath.Join(v.Root, "review", ".staging")
	synthesis, err := os.ReadFile(filepath.Join(dir, synthesisKey))
	if err != nil {
		return "", 0, fmt.Errorf("read staged synthesis: %w", err)
	}
	var rawParts []string
	for _, key := range partKeys {
		data, err := os.ReadFile(filepath.Join(dir, key))
		if err != nil {
			continue
		}
		rawParts = append(rawParts, string(data))
	}
	content := string(synthesis) + ReviewSentinel + strings.Join(rawParts, "\n\n---\n\n")
	relPath, num, err = v.WriteReview(prefix, content)
	if err != nil {
		return "", 0, err
	}
	os.RemoveAll(dir)
	return relPath, num, nil
}

// ─── Issues ───────────────────────────────────────────────────────────────

// ReadIssues reads issues.md from the project root, if it exists.
func (v *Vault) ReadIssues() string {
	data, err := os.ReadFile(filepath.Join(v.Root, "issues.md"))
	if err != nil {
		return ""
	}
	return string(data)
}

// AppendIssue inserts an entry under a heading in issues.md. Creates the
// file and heading if needed.
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
	idx := strings.Index(content, headingLine)
	afterHeading := idx + len(headingLine)
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

// ─── Summaries ────────────────────────────────────────────────────────────

// WriteSummary writes a summary to summary/<name>.md.
func (v *Vault) WriteSummary(name, content string) error {
	dir := filepath.Join(v.Root, "summary")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if !strings.HasSuffix(name, ".md") {
		name = name + ".md"
	}
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
}

// ─── Reviewer memory (legacy hook) ────────────────────────────────────────

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

func (v *Vault) WriteReviewerMemory(role, content string) error {
	dir := filepath.Join(v.Root, "system", "reviewer-memory")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, role+".md"), []byte(content), 0o644)
}

// ─── Snapshots (in plugin export format) ──────────────────────────────────

// WriteSnapshot assembles the full manuscript in the storyline plugin's
// export format and writes it to review/.snapshots/<prefix>-<timestamp>.md.
// Returns the new snapshot path and the path of the most recent prior
// snapshot for the same prefix (or "" if none exists).
func (v *Vault) WriteSnapshot(prefix string) (currentRel, priorRel string, err error) {
	dir := filepath.Join(v.Root, "review", ".snapshots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}

	manuscript, err := v.ReadManuscript()
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(manuscript) == "" {
		return "", "", fmt.Errorf("manuscript is empty (no scenes found)")
	}

	priorRel = v.latestSnapshotRel(prefix)

	timestamp := time.Now().Format("2006-01-02-150405")
	filename := fmt.Sprintf("%s-%s.md", prefix, timestamp)
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(manuscript), 0o644); err != nil {
		return "", "", err
	}
	rel, _ := filepath.Rel(v.Root, path)
	return rel, priorRel, nil
}

func (v *Vault) latestSnapshotRel(prefix string) string {
	dir := filepath.Join(v.Root, "review", ".snapshots")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var latest string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		if !strings.HasPrefix(name, prefix+"-") {
			continue
		}
		if name > latest {
			latest = name
		}
	}
	if latest == "" {
		return ""
	}
	rel, _ := filepath.Rel(v.Root, filepath.Join(dir, latest))
	return rel
}

// SnapshotAndDiff writes a fresh snapshot, computes a unified diff against
// the prior snapshot for the same prefix, and writes the diff to a paired
// `.diff` file alongside the snapshot. Returns all four paths and the diff
// text. priorPath/diffPath/diffText are empty when no prior snapshot exists
// or the manuscript is unchanged.
func (v *Vault) SnapshotAndDiff(prefix string) (snapshotPath, priorPath, diffPath, diffText string, err error) {
	snapshotPath, priorPath, err = v.WriteSnapshot(prefix)
	if err != nil {
		return "", "", "", "", err
	}
	if priorPath == "" {
		return snapshotPath, "", "", "", nil
	}
	diffText, err = v.DiffSnapshots(priorPath, snapshotPath)
	if err != nil {
		return snapshotPath, priorPath, "", "", err
	}
	if diffText == "" {
		return snapshotPath, priorPath, "", "", nil
	}
	diffAbs := filepath.Join(v.Root, snapshotPath)
	diffAbs = strings.TrimSuffix(diffAbs, ".md") + ".diff"
	if err := os.WriteFile(diffAbs, []byte(diffText), 0o644); err != nil {
		return snapshotPath, priorPath, "", diffText, err
	}
	diffPath, _ = filepath.Rel(v.Root, diffAbs)
	return snapshotPath, priorPath, diffPath, diffText, nil
}

// DiffSnapshots runs `diff -u prior current`. Paths may be vault-relative or
// absolute. Exit code 1 (files differ) is treated as success.
func (v *Vault) DiffSnapshots(prior, current string) (string, error) {
	priorAbs := prior
	if !filepath.IsAbs(prior) {
		priorAbs = filepath.Join(v.Root, prior)
	}
	currentAbs := current
	if !filepath.IsAbs(current) {
		currentAbs = filepath.Join(v.Root, current)
	}
	return runDiff(priorAbs, currentAbs)
}

func runDiff(prior, current string) (string, error) {
	cmd := exec.Command("diff", "-u", prior, current)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return string(out), nil
			}
			return "", fmt.Errorf("diff: %w (stderr: %s)", err, string(exitErr.Stderr))
		}
		return "", err
	}
	return string(out), nil
}
