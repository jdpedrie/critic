package types

// ReviewOutput is the structured output from a single reviewer.
type ReviewOutput struct {
	Issues          []Issue          `json:"issues"`
	ConfusionPoints []ConfusionPoint `json:"confusion_points"`
	Strengths       []string         `json:"strengths"`
	Confidence      float64          `json:"confidence"`
}

type Issue struct {
	ID         string `json:"id"`
	Type       string `json:"type"`       // clarity, continuity, pacing, motivation, structure, voice, etc.
	Severity   string `json:"severity"`   // critical, moderate, minor
	Claim      string `json:"claim"`      // what the issue is
	Evidence   string `json:"evidence"`   // quote or paragraph reference
	Suggestion string `json:"suggestion"` // how to fix it
}

type ConfusionPoint struct {
	ID       string `json:"id"`
	Location string `json:"location"` // paragraph or section reference
	Question string `json:"question"` // what confused the reviewer
}

// CrossReviewOutput is the structured output from a cross-review round.
type CrossReviewOutput struct {
	Agreements    []Agreement    `json:"agreements"`
	Disagreements []Disagreement `json:"disagreements"`
	SelfRevisions []SelfRevision `json:"self_revisions"`
	NewIssues     []Issue        `json:"new_issues"`
}

type Agreement struct {
	TargetID      string `json:"target_id"`
	Justification string `json:"justification"`
}

type Disagreement struct {
	TargetID        string `json:"target_id"`
	CounterArgument string `json:"counter_argument"`
	Evidence        string `json:"evidence"`
}

type SelfRevision struct {
	OriginalID string `json:"original_id"`
	Revision   string `json:"revision"`
	Reason     string `json:"reason"`
}

// SynthesisOutput is the final synthesis including the human-readable report.
type SynthesisOutput struct {
	Markdown        string           `json:"markdown"`
	ConfirmedIssues []Issue          `json:"confirmed_issues"`
	ContestedIssues []ContestedIssue `json:"contested_issues"`
	Strengths       []string         `json:"strengths"`
	OpenQuestions   []string         `json:"open_questions"`
}

type ContestedIssue struct {
	Claim      string `json:"claim"`
	PositionA  string `json:"position_a"`
	PositionB  string `json:"position_b"`
	Assessment string `json:"assessment"`
}

// ExtractedFact is a single fact extracted from a chapter for canon checking.
type ExtractedFact struct {
	Type           string `json:"type"`            // character, relationship, location, timeline, rule
	Entity         string `json:"entity"`          // name of the entity
	Claim          string `json:"claim"`           // what is asserted
	SourcePassage  string `json:"source_passage"`  // quote from the chapter
	CanonStatus    string `json:"canon_status"`    // confirmed, new, contradicts
	CanonReference string `json:"canon_reference"` // path to canon file
	ConflictDetail string `json:"conflict_detail"` // only if contradicts
}

// ExtractionOutput is the result of the canon extraction tool.
type ExtractionOutput struct {
	Facts []ExtractedFact `json:"facts"`
}

// DownstreamOutput is the result of assessing downstream effects of an edit.
type DownstreamOutput struct {
	DownstreamIssues []DownstreamIssue `json:"downstream_issues"`
	SafeChapters     []string          `json:"safe_chapters"`
	Summary          string            `json:"summary"`
}

type DownstreamIssue struct {
	ID                 string `json:"id"`
	AffectedChapter    string `json:"affected_chapter"`
	Severity           string `json:"severity"`
	Category           string `json:"category"`
	Claim              string `json:"claim"`
	EditedCause        string `json:"edited_cause"`
	DownstreamEvidence string `json:"downstream_evidence"`
	Suggestion         string `json:"suggestion"`
}

// ManuscriptOutput is the result of a full manuscript review.
type ManuscriptOutput struct {
	ArcIssues       []ArcIssue       `json:"arc_issues"`
	PacingIssues    []PacingIssue    `json:"pacing_issues"`
	CharacterIssues []CharacterIssue `json:"character_issues"`
	DanglingThreads []DanglingThread `json:"dangling_threads"`
	TonalIssues     []TonalIssue     `json:"tonal_issues"`
	Strengths       []string         `json:"strengths"`
	Summary         string           `json:"summary"`
}

type ArcIssue struct {
	ID         string   `json:"id"`
	Arc        string   `json:"arc"`
	Status     string   `json:"status"`
	Detail     string   `json:"detail"`
	Chapters   []string `json:"chapters"`
	Suggestion string   `json:"suggestion"`
}

type PacingIssue struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Chapters   []string `json:"chapters"`
	Detail     string   `json:"detail"`
	Suggestion string   `json:"suggestion"`
}

type CharacterIssue struct {
	ID        string   `json:"id"`
	Character string   `json:"character"`
	Type      string   `json:"type"`
	Detail    string   `json:"detail"`
	Chapters  []string `json:"chapters"`
	Evidence  string   `json:"evidence"`
}

type DanglingThread struct {
	ID           string `json:"id"`
	Thread       string `json:"thread"`
	SetupChapter string `json:"setup_chapter"`
	Detail       string `json:"detail"`
	Suggestion   string `json:"suggestion"`
}

type TonalIssue struct {
	ID       string   `json:"id"`
	Chapters []string `json:"chapters"`
	Detail   string   `json:"detail"`
}
