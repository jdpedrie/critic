package vault

import (
	"strings"
	"testing"
)

const priorSnapshot = `# The Test Book

## Act 1: The Start

### Chapter 1: Arrival

> *An epigraph for the chapter.*

#### Docking

They came in slow.

#### The Bar

Bo bought the first round.

### Chapter 2: Departure

#### Casting Off

Lines away.
`

const currentSnapshot = `# The Test Book

## Act 1: The Start

### Chapter 1: Arrival

> *An epigraph for the chapter.*

#### Docking

They came in slow, and the harbour was quiet.

#### The Bar

Bo bought the first round.

### Chapter 2: Departure

#### Casting Off

Lines away.

#### Open Water

Nothing behind them now but the light.

### Chapter 3: Alone

The whole chapter is one untitled scene.
`

func TestChangedScenes(t *testing.T) {
	cs := ChangedScenes(priorSnapshot, currentSnapshot)

	want := []SceneChange{
		{Chapter: "Chapter 1: Arrival", Scene: "Docking", Change: "modified", WordsBefore: 4, WordsAfter: 9},
		{Chapter: "Chapter 2: Departure", Scene: "Open Water", Change: "added", WordsAfter: 7},
		{Chapter: "Chapter 3: Alone", Scene: "", Change: "added", WordsAfter: 7},
	}
	if len(cs.Scenes) != len(want) {
		t.Fatalf("got %d changes, want %d: %+v", len(cs.Scenes), len(want), cs.Scenes)
	}
	for i := range want {
		if cs.Scenes[i] != want[i] {
			t.Errorf("change %d = %+v, want %+v", i, cs.Scenes[i], want[i])
		}
	}

	for _, s := range []string{
		"### Chapter 1: Arrival\n\n#### Docking\n\n*(modified, 4 to 9 words)*\n\nThey came in slow, and the harbour was quiet.",
		"### Chapter 2: Departure\n\n#### Open Water\n\n*(added, 7 words)*\n\nNothing behind them now but the light.",
		"### Chapter 3: Alone\n\n*(added, 7 words)*\n\nThe whole chapter is one untitled scene.",
	} {
		if !strings.Contains(cs.Text, s) {
			t.Errorf("changed text missing %q\n---\n%s", s, cs.Text)
		}
	}
	if strings.Contains(cs.Text, "The Bar") || strings.Contains(cs.Text, "Casting Off") {
		t.Errorf("changed text includes an unchanged scene:\n%s", cs.Text)
	}
}

func TestChangedScenesRemovedAndRetitled(t *testing.T) {
	prior := "### Chapter 1: Arrival\n\n#### Docking\n\nSlow.\n\n#### The Bar\n\nA round.\n"
	current := "### Chapter 1: Arrival\n\n#### Docking\n\nSlow.\n\n#### The Tavern\n\nA round.\n"
	cs := ChangedScenes(prior, current)
	if len(cs.Scenes) != 2 {
		t.Fatalf("got %+v", cs.Scenes)
	}
	if cs.Scenes[0].Change != "added" || cs.Scenes[0].Scene != "The Tavern" {
		t.Errorf("retitled scene should read as added: %+v", cs.Scenes[0])
	}
	if cs.Scenes[1].Change != "removed" || cs.Scenes[1].Scene != "The Bar" || cs.Scenes[1].WordsBefore != 2 {
		t.Errorf("old title should read as removed: %+v", cs.Scenes[1])
	}
}

func TestChangedScenesIdentical(t *testing.T) {
	cs := ChangedScenes(currentSnapshot, currentSnapshot)
	if len(cs.Scenes) != 0 || cs.Text != "" {
		t.Errorf("identical snapshots should produce no changes: %+v", cs)
	}
}

func TestChangedScenesDuplicateTitles(t *testing.T) {
	prior := "### Chapter 1\n\n#### Interlude\n\nOne.\n\n#### Interlude\n\nTwo.\n"
	current := "### Chapter 1\n\n#### Interlude\n\nOne.\n\n#### Interlude\n\nTwo, revised.\n"
	cs := ChangedScenes(prior, current)
	if len(cs.Scenes) != 1 || cs.Scenes[0].Change != "modified" || cs.Scenes[0].WordsBefore != 1 {
		t.Errorf("second duplicate-titled scene should be the modified one: %+v", cs.Scenes)
	}
}

func TestChangedScenesActRelabelIgnored(t *testing.T) {
	prior := "# T\n\n## Act 1\n\n### Chapter 1\n\n#### A\n\nText.\n"
	current := "# T\n\n## Act 1: The Start\n\n### Chapter 1\n\n#### A\n\nText.\n"
	if cs := ChangedScenes(prior, current); len(cs.Scenes) != 0 {
		t.Errorf("act heading changes are not scene changes: %+v", cs.Scenes)
	}
}
