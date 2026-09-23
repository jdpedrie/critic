package vault

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeTree writes files (vault-relative path → content) under a temp root.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func fixture() map[string]string {
	return map[string]string{
		"The Test Book.md": `---
type: book
title: The Test Book
acts:
  1: The Start
---

A book for tests.
`,
		"Story/00 Prologue.md": `---
chapter: 0
title: Prologue
act: 1
status: final
scenes:
  - title: Prologue
---

## Prologue

In the beginning, [[Ann|the captain]] was alone.
`,
		"Story/01 Arrival.md": `---
chapter: 1
title: Arrival
act: 1
status: draft
pov: "[[Ann]]"
characters:
  - "[[Ann]]"
  - "[[Bo]]"
locations:
  - "[[Port]]"
scenes:
  - title: Docking
    pov: "[[Ann]]"
    location: "[[Port]]"
    characters:
      - "[[Ann]]"
  - title: The Bar
    pov: "[[Bo]]"
    characters:
      - "[[Bo]]"
      - "[[Cy]]"
---

> *An epigraph for the chapter.*

## Docking

They came in slow.

### A heading inside the prose

---

Still docking.

## The Bar

Bo ordered a drink.
`,
		"Story/02.md":                                       "No headings at all. One untitled scene.\n",
		"Story/Outline.md":                                  "Not a chapter: no number anywhere.\n",
		"Background/Characters/Ann.md":                      "Ann entry\n",
		"Background/Locations/Port.md":                      "Port entry\n",
		"Background/world.md":                               "World bible\n",
		"Background/style.md":                               "Background style\n",
		"Review/001-manuscript-critic-2026-01-01-000000.md": "old review\n",
	}
}

func openFixture(t *testing.T, files map[string]string) *Vault {
	t.Helper()
	v, err := New(writeTree(t, files))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return v
}

func TestNewRequiresStory(t *testing.T) {
	root := writeTree(t, map[string]string{"Background/world.md": "x"})
	_, err := New(root)
	if err == nil || !strings.Contains(err.Error(), "Story/") {
		t.Fatalf("New without Story/: err = %v, want mention of Story/", err)
	}
}

func TestReadBook(t *testing.T) {
	v := openFixture(t, fixture())
	b, err := v.ReadBook()
	if err != nil {
		t.Fatal(err)
	}
	if b.Title != "The Test Book" || b.ActLabels[1] != "The Start" || b.Description != "A book for tests." {
		t.Errorf("book = %+v", b)
	}

	bare := openFixture(t, map[string]string{"Story/01.md": "x"})
	b, err = bare.ReadBook()
	if err != nil {
		t.Fatal(err)
	}
	if b.Title != filepath.Base(bare.Root) {
		t.Errorf("fallback title = %q, want root folder name", b.Title)
	}

	files := fixture()
	files["Another.md"] = "---\ntype: book\ntitle: Other\n---\n"
	if _, err := openFixture(t, files).ReadBook(); err == nil {
		t.Error("two book notes: want error")
	}
}

func TestReadChapters(t *testing.T) {
	v := openFixture(t, fixture())
	chapters, err := v.ReadChapters()
	if err != nil {
		t.Fatal(err)
	}

	var nums []int
	for _, ch := range chapters {
		nums = append(nums, ch.Number)
	}
	if !reflect.DeepEqual(nums, []int{0, 1, 2}) {
		t.Fatalf("chapter numbers = %v, want [0 1 2] (Outline.md is not a chapter)", nums)
	}

	ch1 := chapters[1]
	if ch1.Title != "Arrival" || ch1.Status != "draft" || ch1.Preamble != "> *An epigraph for the chapter.*" {
		t.Errorf("chapter 1 = title %q status %q preamble %q", ch1.Title, ch1.Status, ch1.Preamble)
	}
	if len(ch1.Scenes) != 2 {
		t.Fatalf("chapter 1 scenes = %d, want 2", len(ch1.Scenes))
	}
	docking, bar := ch1.Scenes[0], ch1.Scenes[1]
	if docking.ID() != "01-01" || bar.ID() != "01-02" {
		t.Errorf("scene IDs = %s, %s", docking.ID(), bar.ID())
	}
	if !strings.Contains(docking.Body, "### A heading inside the prose") || !strings.Contains(docking.Body, "Still docking.") {
		t.Errorf("### and --- inside a scene must stay in its body; got %q", docking.Body)
	}
	if docking.POV != "Ann" || docking.Location != "Port" || bar.POV != "Bo" {
		t.Errorf("scene meta: docking pov %q loc %q, bar pov %q", docking.POV, docking.Location, bar.POV)
	}
	if !reflect.DeepEqual(bar.Characters, []string{"Bo", "Cy"}) {
		t.Errorf("bar characters = %v", bar.Characters)
	}

	ch2 := chapters[2]
	if ch2.Title != "" || len(ch2.Scenes) != 1 || ch2.Scenes[0].Title != "" ||
		ch2.Scenes[0].Body != "No headings at all. One untitled scene." {
		t.Errorf("headingless chapter = %+v", ch2)
	}
}

func TestSceneMetaFollowsTitleNotPosition(t *testing.T) {
	files := fixture()
	files["Story/01 Arrival.md"] = strings.Replace(files["Story/01 Arrival.md"],
		"## Docking\n\nThey came in slow.\n\n### A heading inside the prose\n\n---\n\nStill docking.\n\n## The Bar\n\nBo ordered a drink.\n",
		"## The Bar\n\nBo ordered a drink.\n\n## Docking\n\nThey came in slow.\n", 1)
	chapters, err := openFixture(t, files).ReadChapters()
	if err != nil {
		t.Fatal(err)
	}
	first := chapters[1].Scenes[0]
	if first.Title != "The Bar" || first.POV != "Bo" {
		t.Errorf("reordered scene 1 = %q pov %q, want The Bar / Bo", first.Title, first.POV)
	}
}

func TestDuplicateChapterNumber(t *testing.T) {
	files := fixture()
	files["Story/01 Also Arrival.md"] = "---\nchapter: 1\n---\n\ntext\n"
	_, err := openFixture(t, files).ReadChapters()
	if err == nil || !strings.Contains(err.Error(), "chapter 1") {
		t.Fatalf("duplicate chapter: err = %v", err)
	}
}

func TestAssembleManuscript(t *testing.T) {
	got, err := openFixture(t, fixture()).ReadManuscript()
	if err != nil {
		t.Fatal(err)
	}
	want := `# The Test Book

## Act 1: The Start

### Chapter 0: Prologue

#### Prologue

In the beginning, the captain was alone.

### Chapter 1: Arrival

> *An epigraph for the chapter.*

#### Docking

They came in slow.

### A heading inside the prose

---

Still docking.

#### The Bar

Bo ordered a drink.

### Chapter 2

No headings at all. One untitled scene.

`
	if got != want {
		t.Errorf("manuscript mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFindScene(t *testing.T) {
	chapters, err := openFixture(t, fixture()).ReadChapters()
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"01-02", "1-2", "1.2", "01-02 The Bar", "01-02 The Bar.md", "the bar"} {
		s, err := FindScene(chapters, ref)
		if err != nil || s.Title != "The Bar" {
			t.Errorf("FindScene(%q) = %v, %v; want The Bar", ref, s, err)
		}
	}
	for _, ref := range []string{"09-01", "01-09", "Nowhere", ""} {
		if _, err := FindScene(chapters, ref); err == nil {
			t.Errorf("FindScene(%q): want error", ref)
		}
	}

	files := fixture()
	files["Story/03 Again.md"] = "## The Bar\n\nBack again.\n"
	chapters, err = openFixture(t, files).ReadChapters()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := FindScene(chapters, "The Bar"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("duplicate title: err = %v, want ambiguous", err)
	}
}

func TestEntityNames(t *testing.T) {
	chapters, err := openFixture(t, fixture()).ReadChapters()
	if err != nil {
		t.Fatal(err)
	}
	if got := ChapterEntityNames(chapters[1]); !reflect.DeepEqual(got, []string{"Ann", "Bo", "Cy", "Port"}) {
		t.Errorf("ChapterEntityNames = %v", got)
	}
	if got := SceneEntityNames(chapters[1].Scenes[:1]); !reflect.DeepEqual(got, []string{"Ann", "Port"}) {
		t.Errorf("SceneEntityNames(docking) = %v", got)
	}
}

func TestBackground(t *testing.T) {
	v := openFixture(t, fixture())

	names, err := v.ListCodexEntries()
	if err != nil || !reflect.DeepEqual(names, []string{"Ann", "Port"}) {
		t.Errorf("ListCodexEntries = %v, %v", names, err)
	}
	if got, err := v.ReadCodexEntry("Port"); err != nil || got != "Port entry\n" {
		t.Errorf("ReadCodexEntry(Port) = %q, %v", got, err)
	}
	if _, err := v.ReadCodexEntry("Nobody"); !os.IsNotExist(err) {
		t.Errorf("ReadCodexEntry(Nobody) err = %v, want not-exist", err)
	}

	research, err := v.ReadResearchFiles()
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for k := range research {
		keys = append(keys, k)
	}
	if len(keys) != 2 || research[filepath.Join("Background", "world.md")] == "" || research[filepath.Join("Background", "style.md")] == "" {
		t.Errorf("research keys = %v; want world.md and style.md only, no entries", keys)
	}

	if got := v.ReadStyleGuide(); got != "Background style\n" {
		t.Errorf("style fallback = %q", got)
	}
	os.WriteFile(filepath.Join(v.Root, "style.md"), []byte("Root style\n"), 0o644)
	if got := v.ReadStyleGuide(); got != "Root style\n" {
		t.Errorf("root style.md should win; got %q", got)
	}
}

func TestReviewAndSnapshotPaths(t *testing.T) {
	v := openFixture(t, fixture())
	if n := v.NextReviewNumber(); n != 2 {
		t.Errorf("NextReviewNumber = %d, want 2", n)
	}
	if err := v.WriteStagedPart("synthesis", "syn"); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteStagedPart("raw", "raw"); err != nil {
		t.Fatal(err)
	}
	rel, num, err := v.AssembleReview("manuscript-critic", "synthesis", []string{"raw"})
	if err != nil {
		t.Fatal(err)
	}
	if num != 2 || !strings.HasPrefix(rel, filepath.Join("Review", "002-manuscript-critic-")) {
		t.Errorf("AssembleReview = %s #%d", rel, num)
	}
	if _, err := os.Stat(filepath.Join(v.Root, "Review", ".staging")); !os.IsNotExist(err) {
		t.Error("staging dir should be removed after assembly")
	}

	snap, prior, err := v.WriteSnapshot(SnapshotMeta{Lineage: "publication", Kind: "manuscript-critic", Review: 2})
	if err != nil {
		t.Fatal(err)
	}
	if prior != "" || !strings.HasPrefix(snap, filepath.Join("Review", ".snapshots", "publication-")) {
		t.Errorf("WriteSnapshot = %q prior %q", snap, prior)
	}
}
