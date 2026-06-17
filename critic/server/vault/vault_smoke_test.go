package vault

import (
	"os"
	"strings"
	"testing"
)

// TestStorylineVaultSmoke verifies the storyline vault layer against the
// user's live Noblesse Oblige project. Skipped when the vault isn't present
// on disk.
func TestStorylineVaultSmoke(t *testing.T) {
	root := "/Users/jdp/obsidian/John Pedrie/Fiction/Noblesse Oblige"
	if _, err := os.Stat(root); err != nil {
		t.Skip("storyline vault not present; skipping smoke test")
	}

	v, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if v.ProjectTitle != "Noblesse Oblige" {
		t.Errorf("ProjectTitle = %q, want %q", v.ProjectTitle, "Noblesse Oblige")
	}

	p, err := v.ReadProject()
	if err != nil {
		t.Fatalf("ReadProject: %v", err)
	}
	if p.Title != "Noblesse Oblige" {
		t.Errorf("project Title = %q", p.Title)
	}
	if p.ActLabels[1] != "The Rim" {
		t.Errorf("ActLabels[1] = %q, want %q", p.ActLabels[1], "The Rim")
	}
	if p.ChapterLabels[1] != "Customs at Fontenoy" {
		t.Errorf("ChapterLabels[1] = %q", p.ChapterLabels[1])
	}
	if len(p.DefinedChapters) != 13 {
		t.Errorf("DefinedChapters len = %d, want 13", len(p.DefinedChapters))
	}

	scenes, err := v.ReadScenes()
	if err != nil {
		t.Fatalf("ReadScenes: %v", err)
	}
	if len(scenes) < 10 {
		t.Fatalf("expected at least 10 scenes, got %d", len(scenes))
	}

	// Sort sanity: scenes must be non-decreasing by (act, chapter, sequence).
	for i := 1; i < len(scenes); i++ {
		prev, cur := scenes[i-1], scenes[i]
		if prev.Act > cur.Act ||
			(prev.Act == cur.Act && prev.Chapter > cur.Chapter) ||
			(prev.Act == cur.Act && prev.Chapter == cur.Chapter && prev.Sequence > cur.Sequence) {
			t.Errorf("scenes out of order at %d: %+v then %+v", i, prev, cur)
		}
	}

	// First non-prologue scene should be Customs at Fontenoy (act 1, ch 1, seq 2).
	var customs *Scene
	for i := range scenes {
		if scenes[i].Filename == "01-01 Customs at Fontenoy" {
			customs = &scenes[i]
			break
		}
	}
	if customs == nil {
		t.Fatal("could not find Customs at Fontenoy scene")
	}
	if customs.Act != 1 || customs.Chapter != 1 || customs.Sequence != 2 {
		t.Errorf("Customs act/chapter/sequence = %d/%d/%d, want 1/1/2",
			customs.Act, customs.Chapter, customs.Sequence)
	}
	if customs.POV != "Henry Nelson" {
		t.Errorf("Customs POV = %q, want %q", customs.POV, "Henry Nelson")
	}
	wantChars := []string{"Henry Nelson", "Roger Green", "Kalina Georgieva", "Luma"}
	if len(customs.Characters) != len(wantChars) {
		t.Errorf("Customs Characters = %v, want %v", customs.Characters, wantChars)
	}
	// Body should not start with the YAML frontmatter delimiter.
	if strings.HasPrefix(customs.Body, "---") {
		t.Errorf("Customs Body still has frontmatter: %q", customs.Body[:80])
	}

	// Assemble and check the manuscript shape.
	manuscript := v.AssembleManuscript(p, scenes)
	if !strings.HasPrefix(manuscript, "# Noblesse Oblige\n\n## Act 1: The Rim\n\n") {
		t.Errorf("manuscript prefix: %q", manuscript[:80])
	}
	// Chapter 1 in this vault: scene 1 is the Epigraph, scene 2 is Customs.
	if !strings.Contains(manuscript, "### Chapter 1: Customs at Fontenoy\n\n#### Epigraph\n\n") {
		t.Errorf("manuscript missing expected chapter/scene structure")
	}
	if !strings.Contains(manuscript, "#### Customs at Fontenoy\n\n") {
		t.Errorf("manuscript missing Customs at Fontenoy scene heading")
	}
	// Wikilinks in the body should be stripped to display text.
	if strings.Contains(manuscript, "[[") {
		idx := strings.Index(manuscript, "[[")
		t.Errorf("manuscript still contains wikilink: %q", manuscript[idx:idx+60])
	}

	// Codex entries — pull the Henry Nelson character.
	codex, err := v.ReadCodexEntries([]string{"Henry Nelson", "Fontenoy Harbor"})
	if err != nil {
		t.Fatalf("ReadCodexEntries: %v", err)
	}
	if len(codex) != 2 {
		t.Errorf("Codex filtered entries: got %d (%v), want 2", len(codex), keysOf(codex))
	}

	// Research files
	research, err := v.ReadResearchFiles()
	if err != nil {
		t.Fatalf("ReadResearchFiles: %v", err)
	}
	if len(research) < 5 {
		t.Errorf("Research files = %d (%v), expected ≥5", len(research), keysOf(research))
	}

	// Auto-stage should mention Act 1 and at least one chapter label.
	stage := v.DerivedStage(p, scenes)
	if !strings.Contains(stage, "Act 1 — The Rim") {
		t.Errorf("DerivedStage missing act header; got: %s", stage)
	}
	if !strings.Contains(stage, "Customs at Fontenoy") {
		t.Errorf("DerivedStage missing chapter label")
	}

	// Entity name union — should include the Customs cast.
	names := SceneEntityNames(scenes)
	if !contains(names, "Henry Nelson") || !contains(names, "Luma") {
		t.Errorf("SceneEntityNames missing principals: %v", names)
	}

	// RenderChapter: chapter 1 should produce both Epigraph and Customs scenes,
	// in sequence order, under the chapter heading.
	var ch1 []Scene
	for _, s := range scenes {
		if s.Chapter == 1 {
			ch1 = append(ch1, s)
		}
	}
	if len(ch1) < 2 {
		t.Fatalf("expected ≥2 scenes in chapter 1, got %d", len(ch1))
	}
	chapterText := RenderChapter(p, 1, ch1)
	if !strings.HasPrefix(chapterText, "### Chapter 1: Customs at Fontenoy\n\n") {
		t.Errorf("RenderChapter prefix wrong: %q", chapterText[:60])
	}
	if !strings.Contains(chapterText, "#### Epigraph\n\n") {
		t.Errorf("RenderChapter missing Epigraph scene heading")
	}
	if !strings.Contains(chapterText, "#### Customs at Fontenoy\n\n") {
		t.Errorf("RenderChapter missing Customs scene heading")
	}
	// Epigraph (seq 1) must precede Customs (seq 2) in the rendered text.
	if strings.Index(chapterText, "#### Epigraph") > strings.Index(chapterText, "#### Customs at Fontenoy") {
		t.Errorf("RenderChapter scenes out of order")
	}

	// RenderScene: pick Customs and render alone.
	sceneText := RenderScene(*customs)
	if !strings.HasPrefix(sceneText, "#### Customs at Fontenoy\n\n") {
		t.Errorf("RenderScene prefix wrong: %q", sceneText[:60])
	}
	if strings.Contains(sceneText, "[[") {
		t.Errorf("RenderScene still contains wikilinks")
	}
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
