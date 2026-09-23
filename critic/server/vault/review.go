package vault

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ReviewSentinel separates the synthesis (which reviewers see) from the raw
// agent outputs (which are retained for traceability but not fed to future
// reviews).
const ReviewSentinel = "\n\n<!-- RAW AGENT OUTPUTS BELOW — NOT INCLUDED IN FUTURE REVIEW CONTEXT -->\n\n"

// WriteReview writes a review file to Review/ with a globally sequential
// number. Format: NNN-prefix-timestamp.md.
func (v *Vault) WriteReview(prefix, content string) (relPath string, num int, err error) {
	dir := filepath.Join(v.Root, ReviewDir)
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

// NextReviewNumber scans Review/ for the highest leading NNN and returns
// the next sequential value. Global across all review-type prefixes.
func (v *Vault) NextReviewNumber() int {
	dir := filepath.Join(v.Root, ReviewDir)
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
	dir := filepath.Join(v.Root, ReviewDir)
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
	dir := filepath.Join(v.Root, ReviewDir)
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
	return os.WriteFile(filepath.Join(v.Root, ReviewDir, filename), []byte(content), 0o644)
}

// WriteStagedPart writes a named part to Review/.staging/ for later assembly.
func (v *Vault) WriteStagedPart(name, content string) error {
	dir := filepath.Join(v.Root, ReviewDir, ".staging")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
}

// AssembleReview reads staged parts, assembles them with the sentinel, writes
// the final review file, and cleans up staging.
func (v *Vault) AssembleReview(prefix, synthesisKey string, partKeys []string) (relPath string, num int, err error) {
	dir := filepath.Join(v.Root, ReviewDir, ".staging")
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

// ─── Snapshots ──────────────────────────────────────────────────────

// SnapshotMeta records what a snapshot was taken for. Lineage is the
// grouping key for "previous snapshot" lookups: a review diffs only against
// snapshots in its own lineage, so a craft review never sees a publication
// baseline and vice versa. Kind and Review identify the review that took
// the snapshot.
type SnapshotMeta struct {
	Lineage string `json:"lineage"`
	Kind    string `json:"kind,omitempty"`
	Review  int    `json:"review,omitempty"`
	Created string `json:"created,omitempty"`
}

// Snapshot is one entry in a snapshot listing.
type Snapshot struct {
	Path     string `json:"path"`
	DiffPath string `json:"diff_path,omitempty"`
	SnapshotMeta
}

const snapshotTimestamp = "2006-01-02-150405"

// snapshotClock is the time source for snapshot names; tests replace it so
// several snapshots can land inside one second without colliding.
var snapshotClock = time.Now

var (
	snapshotNameRE = regexp.MustCompile(`^(.+)-\d{4}-\d{2}-\d{2}-\d{6}\.md$`)
	lineageRE      = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

func snapshotsDir(root string) string { return filepath.Join(root, ReviewDir, ".snapshots") }

func sidecarRel(snapshotRel string) string { return strings.TrimSuffix(snapshotRel, ".md") + ".json" }
func diffRel(snapshotRel string) string    { return strings.TrimSuffix(snapshotRel, ".md") + ".diff" }

// snapshotLineage derives a lineage from a snapshot file name. Snapshots
// from before lineages existed are named manuscript-<timestamp>.md and were
// all taken by the publication-frame manuscript review, so they belong to
// the publication lineage.
func snapshotLineage(name string) string {
	m := snapshotNameRE.FindStringSubmatch(name)
	if m == nil {
		return ""
	}
	if m[1] == "manuscript" {
		return "publication"
	}
	return m[1]
}

// readSnapshotMeta loads a snapshot's sidecar, falling back to the lineage
// implied by the file name when there is none.
func (v *Vault) readSnapshotMeta(rel string) SnapshotMeta {
	var meta SnapshotMeta
	if data, err := os.ReadFile(filepath.Join(v.Root, sidecarRel(rel))); err == nil {
		_ = json.Unmarshal(data, &meta)
	}
	if meta.Lineage == "" {
		meta.Lineage = snapshotLineage(filepath.Base(rel))
	}
	return meta
}

// ListSnapshots returns every snapshot, oldest first, optionally limited to
// one lineage.
func (v *Vault) ListSnapshots(lineage string) []Snapshot {
	entries, err := os.ReadDir(snapshotsDir(v.Root))
	if err != nil {
		return nil
	}
	var out []Snapshot
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !snapshotNameRE.MatchString(name) {
			continue
		}
		rel := filepath.Join(ReviewDir, ".snapshots", name)
		meta := v.readSnapshotMeta(rel)
		if lineage != "" && meta.Lineage != lineage {
			continue
		}
		s := Snapshot{Path: rel, SnapshotMeta: meta}
		if _, err := os.Stat(filepath.Join(v.Root, diffRel(rel))); err == nil {
			s.DiffPath = diffRel(rel)
		}
		out = append(out, s)
	}
	// Names end in a timestamp, so ordering on that suffix is chronological
	// whatever the lineage prefix.
	sort.SliceStable(out, func(i, j int) bool {
		return timestampOf(out[i].Path) < timestampOf(out[j].Path)
	})
	return out
}

func timestampOf(rel string) string {
	name := strings.TrimSuffix(filepath.Base(rel), ".md")
	if len(name) < len(snapshotTimestamp) {
		return name
	}
	return name[len(name)-len(snapshotTimestamp):]
}

// priorSnapshot picks what a new snapshot for `review` should diff against:
// the newest snapshot in the lineage. One recorded for the same or a later
// review number is the orphan of a run that died before its review was
// assembled (the number was then reused), and is skipped so the diff still
// covers everything since the last review that landed.
func (v *Vault) priorSnapshot(lineage string, review int) string {
	snaps := v.ListSnapshots(lineage)
	for i := len(snaps) - 1; i >= 0; i-- {
		if review > 0 && snaps[i].Review >= review {
			continue
		}
		return snaps[i].Path
	}
	return ""
}

// WriteSnapshot assembles the full manuscript (see AssembleManuscript),
// writes it to Review/.snapshots/<lineage>-<timestamp>.md with a paired
// .json sidecar holding meta, and returns the new snapshot's path and the
// prior snapshot in the same lineage ("" if none).
func (v *Vault) WriteSnapshot(meta SnapshotMeta) (currentRel, priorRel string, err error) {
	if !lineageRE.MatchString(meta.Lineage) {
		return "", "", fmt.Errorf("snapshot lineage %q: use lowercase letters, digits, and hyphens", meta.Lineage)
	}
	if meta.Lineage == "manuscript" {
		return "", "", fmt.Errorf("snapshot lineage %q names pre-lineage snapshots; use \"publication\"", meta.Lineage)
	}
	dir := snapshotsDir(v.Root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}

	manuscript, err := v.ReadManuscript()
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(manuscript) == "" {
		return "", "", fmt.Errorf("manuscript is empty")
	}

	priorRel = v.priorSnapshot(meta.Lineage, meta.Review)

	now := snapshotClock()
	path := filepath.Join(dir, fmt.Sprintf("%s-%s.md", meta.Lineage, now.Format(snapshotTimestamp)))
	if err := os.WriteFile(path, []byte(manuscript), 0o644); err != nil {
		return "", "", err
	}
	rel, _ := filepath.Rel(v.Root, path)
	meta.Created = now.Format(time.RFC3339)
	sidecar, _ := json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(filepath.Join(v.Root, sidecarRel(rel)), sidecar, 0o644); err != nil {
		return "", "", err
	}
	return rel, priorRel, nil
}

// SnapshotAndDiff writes a fresh snapshot, computes a unified diff against
// the prior snapshot in the same lineage, and writes the diff to a paired
// `.diff` file alongside the snapshot. priorPath/diffPath/diffText are
// empty when the lineage has no prior snapshot or the manuscript is
// unchanged.
func (v *Vault) SnapshotAndDiff(meta SnapshotMeta) (snapshotPath, priorPath, diffPath, diffText string, err error) {
	snapshotPath, priorPath, err = v.WriteSnapshot(meta)
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
	diffPath = diffRel(snapshotPath)
	if err := os.WriteFile(filepath.Join(v.Root, diffPath), []byte(diffText), 0o644); err != nil {
		return snapshotPath, priorPath, "", diffText, err
	}
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

// ─── Changed scenes ─────────────────────────────────────────────────────

// SceneChange records one scene whose text differs between two assembled
// manuscripts.
type SceneChange struct {
	Chapter     string `json:"chapter"` // current chapter heading, e.g. "Chapter 4: Arcadia"
	Scene       string `json:"scene"`   // scene title; empty for an untitled scene or chapter preamble
	Change      string `json:"change"`  // added | modified | removed
	WordsBefore int    `json:"words_before"`
	WordsAfter  int    `json:"words_after"`
}

// ChangeSet is the scene-level view of the difference between two
// snapshots: which scenes changed, and the full text of the ones that are
// new or modified, so a reviewer can be pointed at exactly the material the
// author touched.
type ChangeSet struct {
	Scenes []SceneChange `json:"scenes"` // manuscript order; removed scenes last, in prior order
	Text   string        `json:"text"`   // added and modified scenes in full, with headings
}

// section is one heading-delimited block of an assembled manuscript.
type section struct {
	chapterNum int
	chapter    string // heading text without the leading hashes
	scene      string
	body       string
}

var chapterHeadingRE = regexp.MustCompile(`^### Chapter (\d+)`)

// splitSections walks an assembled manuscript (the snapshot shape: `###
// Chapter N` and `#### <scene>` headings) and returns its scenes in order.
// Text between a chapter heading and its first scene heading is the chapter
// preamble, keyed with an empty scene title; an untitled scene lands there
// too, because the assembler prints no heading for it. Title and act
// headings belong to no scene.
func splitSections(text string) []section {
	var out []section
	var cur *section
	flush := func() {
		if cur == nil {
			return
		}
		cur.body = strings.TrimSpace(cur.body)
		if cur.scene != "" || cur.body != "" {
			out = append(out, *cur)
		}
		cur = nil
	}
	chapterNum, chapter := 0, ""
	for _, line := range strings.Split(text, "\n") {
		switch {
		case chapterHeadingRE.MatchString(line):
			flush()
			fmt.Sscanf(chapterHeadingRE.FindStringSubmatch(line)[1], "%d", &chapterNum)
			chapter = strings.TrimSpace(strings.TrimPrefix(line, "###"))
			cur = &section{chapterNum: chapterNum, chapter: chapter}
		case strings.HasPrefix(line, "#### "):
			flush()
			cur = &section{chapterNum: chapterNum, chapter: chapter, scene: strings.TrimSpace(strings.TrimPrefix(line, "####"))}
		case strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "# "):
			flush()
		case cur == nil:
			continue
		default:
			cur.body += line + "\n"
		}
	}
	flush()
	return out
}

// keyed returns the sections' keys in order plus a lookup. A key is chapter
// number, scene title, and occurrence, so two scenes that share a title
// inside one chapter stay distinct. Retitling a scene or renumbering a
// chapter therefore reads as a removal plus an addition.
func keyed(secs []section) ([]string, map[string]section) {
	keys := make([]string, 0, len(secs))
	m := make(map[string]section, len(secs))
	seen := map[string]int{}
	for _, s := range secs {
		base := fmt.Sprintf("%d|%s", s.chapterNum, s.scene)
		seen[base]++
		k := fmt.Sprintf("%s|%d", base, seen[base])
		keys = append(keys, k)
		m[k] = s
	}
	return keys, m
}

// ChangedScenes compares two assembled manuscripts scene by scene.
func ChangedScenes(prior, current string) ChangeSet {
	beforeKeys, before := keyed(splitSections(prior))
	afterKeys, after := keyed(splitSections(current))

	var cs ChangeSet
	var text strings.Builder
	lastChapter := -1
	for _, k := range afterKeys {
		a := after[k]
		b, existed := before[k]
		var change string
		switch {
		case !existed:
			change = "added"
		case a.body != b.body:
			change = "modified"
		default:
			continue
		}
		wb, wa := WordCount(b.body), WordCount(a.body)
		cs.Scenes = append(cs.Scenes, SceneChange{
			Chapter: a.chapter, Scene: a.scene, Change: change, WordsBefore: wb, WordsAfter: wa,
		})
		if a.chapterNum != lastChapter {
			fmt.Fprintf(&text, "### %s\n\n", a.chapter)
			lastChapter = a.chapterNum
		}
		if a.scene != "" {
			fmt.Fprintf(&text, "#### %s\n\n", a.scene)
		}
		if change == "added" {
			fmt.Fprintf(&text, "*(added, %d words)*\n\n", wa)
		} else {
			fmt.Fprintf(&text, "*(modified, %d to %d words)*\n\n", wb, wa)
		}
		text.WriteString(a.body)
		text.WriteString("\n\n")
	}
	for _, k := range beforeKeys {
		if _, still := after[k]; still {
			continue
		}
		b := before[k]
		cs.Scenes = append(cs.Scenes, SceneChange{
			Chapter: b.chapter, Scene: b.scene, Change: "removed", WordsBefore: WordCount(b.body),
		})
	}
	cs.Text = strings.TrimSpace(text.String())
	return cs
}

// SnapshotChanges reads two snapshot files (vault-relative or absolute) and
// returns their scene-level differences.
func (v *Vault) SnapshotChanges(prior, current string) (ChangeSet, error) {
	read := func(p string) (string, error) {
		if !filepath.IsAbs(p) {
			p = filepath.Join(v.Root, p)
		}
		data, err := os.ReadFile(p)
		return string(data), err
	}
	before, err := read(prior)
	if err != nil {
		return ChangeSet{}, fmt.Errorf("read prior snapshot: %w", err)
	}
	after, err := read(current)
	if err != nil {
		return ChangeSet{}, fmt.Errorf("read current snapshot: %w", err)
	}
	return ChangedScenes(before, after), nil
}
