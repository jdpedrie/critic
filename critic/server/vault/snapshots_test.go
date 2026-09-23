package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeClock hands out one fixed timestamp per snapshot so several can be
// written inside a single test without colliding on the second-resolution
// file name.
func fakeClock(t *testing.T, stamps ...string) func() time.Time {
	t.Helper()
	i := 0
	return func() time.Time {
		if i >= len(stamps) {
			t.Fatalf("fakeClock: more than %d snapshots written", len(stamps))
		}
		ts, err := time.Parse(snapshotTimestamp, stamps[i])
		if err != nil {
			t.Fatal(err)
		}
		i++
		return ts
	}
}

func TestSnapshotLineages(t *testing.T) {
	v := openFixture(t, fixture())
	snapshotClock = fakeClock(t,
		"2026-02-01-000000", "2026-02-02-000000", "2026-02-03-000000",
		"2026-02-04-000000", "2026-02-05-000000", "2026-02-06-000000",
	)
	defer func() { snapshotClock = time.Now }()

	snapDir := filepath.Join(v.Root, "Review", ".snapshots")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A snapshot from before lineages existed: no sidecar, manuscript- prefix.
	legacyRel := filepath.Join("Review", ".snapshots", "manuscript-2026-01-01-000000.md")
	if err := os.WriteFile(filepath.Join(v.Root, legacyRel), []byte("# old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	pubA, prior, err := v.WriteSnapshot(SnapshotMeta{Lineage: "publication", Kind: "manuscript-critic", Review: 5})
	if err != nil {
		t.Fatal(err)
	}
	if prior != legacyRel {
		t.Errorf("publication should inherit the legacy manuscript-* snapshot as prior; got %q", prior)
	}
	if !strings.HasPrefix(filepath.Base(pubA), "publication-") {
		t.Errorf("snapshot name should carry the lineage: %s", pubA)
	}

	craftB, prior, err := v.WriteSnapshot(SnapshotMeta{Lineage: "craft", Kind: "manuscript-craft", Review: 6})
	if err != nil {
		t.Fatal(err)
	}
	if prior != "" {
		t.Errorf("craft must never diff against a publication snapshot; got prior %q", prior)
	}

	craftC, prior, err := v.WriteSnapshot(SnapshotMeta{Lineage: "craft", Kind: "delta-craft", Review: 7})
	if err != nil {
		t.Fatal(err)
	}
	if prior != craftB {
		t.Errorf("craft prior = %q, want %q", prior, craftB)
	}

	_, prior, err = v.WriteSnapshot(SnapshotMeta{Lineage: "publication", Kind: "manuscript-critic", Review: 8})
	if err != nil {
		t.Fatal(err)
	}
	if prior != pubA {
		t.Errorf("publication prior = %q, want %q (craft snapshots must be invisible)", prior, pubA)
	}

	// A run that died after snapshotting for review 9; the retry reuses 9.
	if _, _, err := v.WriteSnapshot(SnapshotMeta{Lineage: "craft", Kind: "delta-craft", Review: 9}); err != nil {
		t.Fatal(err)
	}
	_, prior, err = v.WriteSnapshot(SnapshotMeta{Lineage: "craft", Kind: "delta-craft", Review: 9})
	if err != nil {
		t.Fatal(err)
	}
	if prior != craftC {
		t.Errorf("orphan snapshot for the same review number should be skipped; prior = %q, want %q", prior, craftC)
	}

	craft := v.ListSnapshots("craft")
	if len(craft) != 4 {
		t.Fatalf("ListSnapshots(craft) = %d entries, want 4: %+v", len(craft), craft)
	}
	if craft[0].Path != craftB || craft[0].Kind != "manuscript-craft" || craft[0].Review != 6 || craft[0].Created == "" {
		t.Errorf("sidecar meta not round-tripped: %+v", craft[0])
	}
	all := v.ListSnapshots("")
	if len(all) != 7 || all[0].Path != legacyRel || all[0].Lineage != "publication" {
		t.Errorf("ListSnapshots(all) should start with the legacy snapshot as publication: %+v", all)
	}

	for _, bad := range []string{"manuscript", "Craft", "craft review", ""} {
		if _, _, err := v.WriteSnapshot(SnapshotMeta{Lineage: bad}); err == nil {
			t.Errorf("lineage %q should be rejected", bad)
		}
	}
}

func TestSnapshotAndDiffWithinLineage(t *testing.T) {
	v := openFixture(t, fixture())
	snapshotClock = fakeClock(t, "2026-03-01-000000", "2026-03-02-000000", "2026-03-03-000000")
	defer func() { snapshotClock = time.Now }()

	if _, _, _, diff, err := v.SnapshotAndDiff(SnapshotMeta{Lineage: "craft", Review: 1}); err != nil || diff != "" {
		t.Fatalf("first snapshot in a lineage: diff %q err %v", diff, err)
	}

	chapter := filepath.Join(v.Root, "Story", "01 Arrival.md")
	data, err := os.ReadFile(chapter)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(chapter, append(data, []byte("\n\nA new closing line for the bar.\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	// The publication lineage has no baseline, so it sees no diff even
	// though the craft lineage does.
	if _, prior, _, diff, err := v.SnapshotAndDiff(SnapshotMeta{Lineage: "publication", Review: 2}); err != nil || prior != "" || diff != "" {
		t.Fatalf("publication must not diff against craft: prior %q diff %q err %v", prior, diff, err)
	}

	snap, prior, diffPath, diff, err := v.SnapshotAndDiff(SnapshotMeta{Lineage: "craft", Kind: "delta-craft", Review: 3})
	if err != nil {
		t.Fatal(err)
	}
	if prior == "" || diff == "" || !strings.Contains(diff, "A new closing line") {
		t.Errorf("craft diff should show the edit: prior %q diff %q", prior, diff)
	}
	if diffPath != strings.TrimSuffix(snap, ".md")+".diff" {
		t.Errorf("diff path %q not paired with snapshot %q", diffPath, snap)
	}
	changes, err := v.SnapshotChanges(prior, snap)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes.Scenes) != 1 || changes.Scenes[0].Scene != "The Bar" || changes.Scenes[0].Change != "modified" {
		t.Errorf("changed scenes = %+v", changes.Scenes)
	}
	listed := v.ListSnapshots("craft")
	if got := listed[len(listed)-1]; got.DiffPath != diffPath || got.Kind != "delta-craft" {
		t.Errorf("listing should carry the diff path and kind: %+v", got)
	}
}
