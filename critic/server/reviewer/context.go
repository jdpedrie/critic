package reviewer

import (
	"fmt"
	"strings"
)

// WithStyleGuide prepends the style guide to a context string, if present.
func WithStyleGuide(styleGuide, context string) string {
	if styleGuide == "" {
		return context
	}
	return "=== STYLE GUIDE ===\n\n" + styleGuide + "\n\n" + context
}

// WithKnownIssues prepends the known issues file to a context string, if present.
func WithKnownIssues(issues, context string) string {
	if issues == "" {
		return context
	}
	return "=== KNOWN ISSUES (DEFERRED) ===\nThe following issues have been acknowledged by the author and deferred. Only re-raise if the issue has escalated in importance due to new text.\n\n" + issues + "\n\n" + context
}

// WithPageInfo prepends page count information to a context string.
func WithPageInfo(inputPages, totalPages int, context string) string {
	return fmt.Sprintf("=== MANUSCRIPT INFO ===\nPages in this input: ~%d (at 300 words/page)\nTotal manuscript pages: ~%d\n\n%s", inputPages, totalPages, context)
}

// BuildTextOnlyContext builds the user prompt for text-only reviewers (analytical, immersive).
func BuildTextOnlyContext(chapter string, priorSummaries []struct{ Name, Content string }) string {
	var b strings.Builder

	if len(priorSummaries) > 0 {
		b.WriteString("=== PRIOR CHAPTER SUMMARIES ===\n")
		b.WriteString("(You may request the full text of any chapter if you need more detail.)\n\n")
		for _, s := range priorSummaries {
			fmt.Fprintf(&b, "--- %s ---\n%s\n\n", s.Name, s.Content)
		}
	}

	fmt.Fprintf(&b, "=== CHAPTER TO REVIEW ===\n\n%s", chapter)

	return b.String()
}

// BuildFullContext builds the user prompt for full-context reviewers (structural, adversarial).
func BuildFullContext(chapter string, canon map[string]string, plot map[string]string) string {
	var b strings.Builder

	if len(canon) > 0 {
		b.WriteString("=== CANON (World-Building Notes) ===\n\n")
		for path, content := range canon {
			fmt.Fprintf(&b, "--- %s ---\n%s\n\n", path, content)
		}
	}

	if len(plot) > 0 {
		b.WriteString("=== PLOT (Outline & Arcs) ===\n\n")
		for path, content := range plot {
			fmt.Fprintf(&b, "--- %s ---\n%s\n\n", path, content)
		}
	}

	fmt.Fprintf(&b, "=== CHAPTER TO REVIEW ===\n\n%s", chapter)

	return b.String()
}

// BuildCrossReviewContext builds the user prompt for cross-review.
func BuildCrossReviewContext(ownReview, counterpartReview string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "=== YOUR ORIGINAL REVIEW ===\n\n%s\n\n", ownReview)
	fmt.Fprintf(&b, "=== COUNTERPART'S REVIEW ===\n\n%s", counterpartReview)

	return b.String()
}

// BuildSynthesisContext builds the user prompt for synthesis.
func BuildSynthesisContext(reviews map[string]string, rebuttals map[string]string) string {
	var b strings.Builder

	b.WriteString("=== REVIEWS ===\n\n")
	for role, review := range reviews {
		fmt.Fprintf(&b, "--- %s ---\n%s\n\n", role, review)
	}

	if len(rebuttals) > 0 {
		b.WriteString("=== CROSS-REVIEW REBUTTALS ===\n\n")
		for pair, rebuttal := range rebuttals {
			fmt.Fprintf(&b, "--- %s ---\n%s\n\n", pair, rebuttal)
		}
	}

	return b.String()
}

// BuildDownstreamContext builds the user prompt for downstream assessment.
func BuildDownstreamContext(editedChapter string, downstreamChapters []struct{ Name, Content string }, canon map[string]string, plot map[string]string) string {
	var b strings.Builder

	if len(canon) > 0 {
		b.WriteString("=== CANON ===\n\n")
		for path, content := range canon {
			fmt.Fprintf(&b, "--- %s ---\n%s\n\n", path, content)
		}
	}

	if len(plot) > 0 {
		b.WriteString("=== PLOT ===\n\n")
		for path, content := range plot {
			fmt.Fprintf(&b, "--- %s ---\n%s\n\n", path, content)
		}
	}

	fmt.Fprintf(&b, "=== EDITED CHAPTER ===\n\n%s\n\n", editedChapter)

	b.WriteString("=== DOWNSTREAM CHAPTERS ===\n\n")
	for _, ch := range downstreamChapters {
		fmt.Fprintf(&b, "--- %s ---\n%s\n\n", ch.Name, ch.Content)
	}

	return b.String()
}

// BuildManuscriptContext builds the user prompt for full manuscript review.
// Text-only — no canon or plot. The manuscript reviewer reads as a reader, not an editor.
func BuildManuscriptContext(chapters []struct{ Name, Content string }) string {
	var b strings.Builder

	b.WriteString("=== FULL MANUSCRIPT ===\n\n")
	for _, ch := range chapters {
		fmt.Fprintf(&b, "--- %s ---\n%s\n\n", ch.Name, ch.Content)
	}

	return b.String()
}

// BuildCanonExtractionContext builds the user prompt for canon extraction.
func BuildCanonExtractionContext(chapter string, canon map[string]string) string {
	var b strings.Builder

	if len(canon) > 0 {
		b.WriteString("=== EXISTING CANON ===\n\n")
		for path, content := range canon {
			fmt.Fprintf(&b, "--- %s ---\n%s\n\n", path, content)
		}
	}

	fmt.Fprintf(&b, "=== CHAPTER TO EXTRACT FROM ===\n\n%s", chapter)

	return b.String()
}
