package reviewer

import (
	"context"
	"fmt"

	"github.com/jdp/critic/server/agent"
)

// Role identifies a reviewer role.
type Role string

const (
	RoleAnalytical  Role = "analytical"
	RoleImmersive   Role = "immersive"
	RoleStructural  Role = "structural"
	RoleAdversarial Role = "adversarial"
)

// Reviewer runs a single review pass with a specific role and agent.
type Reviewer struct {
	Role      Role
	Agent     agent.Agent
	MaxIssues int
}

func New(role Role, a agent.Agent, maxIssues int) *Reviewer {
	return &Reviewer{
		Role:      role,
		Agent:     a,
		MaxIssues: maxIssues,
	}
}

// Review runs the reviewer against the given context and returns prose.
func (r *Reviewer) Review(ctx context.Context, userPrompt string) (string, error) {
	prompt := systemPrompt(string(r.Role), r.MaxIssues)
	raw, err := r.Agent.Run(ctx, prompt, userPrompt)
	if err != nil {
		return "", fmt.Errorf("review (%s): %w", r.Role, err)
	}
	return raw, nil
}

// CrossReview runs a cross-review pass. Returns prose.
func CrossReview(ctx context.Context, a agent.Agent, ownReview, counterpartReview string, maxNewIssues int) (string, error) {
	prompt := crossReviewSystemPrompt(maxNewIssues)
	userPrompt := BuildCrossReviewContext(ownReview, counterpartReview)

	raw, err := a.Run(ctx, prompt, userPrompt)
	if err != nil {
		return "", fmt.Errorf("cross-review: %w", err)
	}
	return raw, nil
}

// Synthesize runs the synthesis pass. Returns prose.
func Synthesize(ctx context.Context, a agent.Agent, reviews map[string]string, rebuttals map[string]string, reviewNum int) (string, error) {
	prompt := synthesisSystemPrompt(reviewNum)
	userPrompt := BuildSynthesisContext(reviews, rebuttals)

	raw, err := a.Run(ctx, prompt, userPrompt)
	if err != nil {
		return "", fmt.Errorf("synthesis: %w", err)
	}
	return raw, nil
}

// ExtractCanon runs the canon extraction pass. Returns prose.
func ExtractCanon(ctx context.Context, a agent.Agent, chapter string, canon map[string]string, styleGuide string, inputPages, totalPages int) (string, error) {
	prompt := canonExtractionSystemPrompt()
	userPrompt := WithPageInfo(inputPages, totalPages, WithStyleGuide(styleGuide, BuildCanonExtractionContext(chapter, canon)))

	raw, err := a.Run(ctx, prompt, userPrompt)
	if err != nil {
		return "", fmt.Errorf("canon extraction: %w", err)
	}
	return raw, nil
}

// AssessDownstream assesses downstream effects of an edited chapter. Returns prose.
func AssessDownstream(ctx context.Context, a agent.Agent, editedChapter string, downstream []struct{ Name, Content string }, canon, plot map[string]string, styleGuide string, inputPages, totalPages int) (string, error) {
	prompt := downstreamSystemPrompt()
	userPrompt := WithPageInfo(inputPages, totalPages, WithStyleGuide(styleGuide, BuildDownstreamContext(editedChapter, downstream, canon, plot)))

	raw, err := a.Run(ctx, prompt, userPrompt)
	if err != nil {
		return "", fmt.Errorf("downstream assessment: %w", err)
	}
	return raw, nil
}

// ReviewManuscript reviews the full manuscript at the book level.
// Returns prose + session ID for cross-review continuity.
func ReviewManuscript(ctx context.Context, a agent.Agent, chapters []struct{ Name, Content string }, priorSummary, styleGuide, knownIssues string, totalPages int) (response string, sessionID string, err error) {
	prompt := manuscriptSystemPrompt()
	// For manuscript review, input pages = total pages (reading the whole thing)
	userPrompt := WithPageInfo(totalPages, totalPages, WithKnownIssues(knownIssues, WithStyleGuide(styleGuide, BuildManuscriptContext(chapters))))

	if priorSummary != "" {
		userPrompt = fmt.Sprintf("=== PRIOR REVIEW SUMMARY ===\nThe following issues were raised in the previous review. Assess whether each has been addressed in the current manuscript, then proceed with your full review.\n\n%s\n\n%s", priorSummary, userPrompt)
	}

	if sa, ok := a.(agent.SessionAgent); ok {
		response, sessionID, err = sa.RunSession(ctx, prompt, userPrompt)
		if err != nil {
			return "", "", fmt.Errorf("manuscript review: %w", err)
		}
		return response, sessionID, nil
	}

	response, err = a.Run(ctx, prompt, userPrompt)
	if err != nil {
		return "", "", fmt.Errorf("manuscript review: %w", err)
	}
	return response, "", nil
}

// ReviewManuscriptWithRejection does the initial review then a rejection pass
// in the same session. Returns both the review and the rejection pass.
func ReviewManuscriptWithRejection(ctx context.Context, a agent.Agent, chapters []struct{ Name, Content string }, priorSummary, styleGuide, knownIssues string, totalPages int) (review, rejection, sessionID string, err error) {
	review, sessionID, err = ReviewManuscript(ctx, a, chapters, priorSummary, styleGuide, knownIssues, totalPages)
	if err != nil {
		return "", "", "", err
	}

	// Rejection pass — resume the same session
	if sa, ok := a.(agent.SessionAgent); ok && sessionID != "" {
		rejection, err = sa.Resume(ctx, sessionID, rejectionPassPrompt())
		if err != nil {
			// Non-fatal — return the review even if rejection pass fails
			rejection = fmt.Sprintf("*Rejection pass failed: %v*", err)
		}
	}

	return review, rejection, sessionID, nil
}

// CrossReviewResume runs a cross-review by resuming an existing session.
// The reviewer already has its original review in context and now receives
// the counterpart's review for rebuttal.
func CrossReviewResume(ctx context.Context, a agent.Agent, sessionID string, counterpartReview string, maxNewIssues int) (string, error) {
	prompt := fmt.Sprintf(`You previously wrote a manuscript review. Now review your counterpart's assessment below.

State your model identity at the start of your output (e.g., "Reviewer: Claude", "Reviewer: Gemini", "Reviewer: GPT").

Where you agree, say so briefly. Where you disagree, explain why with evidence from the text.
Note anything your counterpart caught that you missed, and anything they missed that you stand by.

Be direct. Write readable prose.

Counterpart's review:

%s`, counterpartReview)

	if sa, ok := a.(agent.SessionAgent); ok && sessionID != "" {
		return sa.Resume(ctx, sessionID, prompt)
	}
	// Fallback: no session, run cold
	return a.Run(ctx, crossReviewSystemPrompt(maxNewIssues), prompt)
}
