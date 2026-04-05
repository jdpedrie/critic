package reviewer

import (
	"fmt"

	"github.com/jdp/critic/server/prompts"
)

// VaultPath is set at init time and passed through to prompt loading.
// This allows vault-level prompt overrides.
var VaultPath string

func lp(name string) string {
	return prompts.Load(name, VaultPath)
}

func systemPrompt(role string, maxIssues int) string {
	base := lp("agent-framing.md") + "\n" + prompts.Render("review-base.md", VaultPath, struct {
		Role      string
		MaxIssues int
	}{role, maxIssues})

	roleFile := map[string]string{
		"analytical":  "review-analytical.md",
		"immersive":   "review-immersive.md",
		"structural":  "review-structural.md",
		"adversarial": "review-adversarial-role.md",
	}

	if f, ok := roleFile[role]; ok {
		base += "\n" + lp(f)
	}

	return base + "\n" + lp("verdict.md")
}

func crossReviewSystemPrompt(maxNewIssues int) string {
	return lp("agent-framing.md") + "\n" + prompts.Render("cross-review.md", VaultPath, struct {
		MaxNewIssues int
	}{maxNewIssues})
}

func synthesisSystemPrompt(reviewNum int) string {
	return lp("agent-framing.md") + "\n" + prompts.Render("synthesis.md", VaultPath, struct {
		ReviewNum string
	}{fmt.Sprintf("%03d", reviewNum)}) + "\n" + lp("verdict.md")
}

func canonExtractionSystemPrompt() string {
	return lp("agent-framing.md") + "\n" + lp("canon-extraction.md")
}

func downstreamSystemPrompt() string {
	return lp("agent-framing.md") + "\n" + lp("downstream.md")
}

func manuscriptSystemPrompt() string {
	return lp("agent-framing.md") + "\n" + lp("manuscript.md") + "\n" + lp("verdict.md")
}

func rejectionPassPrompt() string {
	return lp("rejection-pass.md")
}

func AdversarialManuscriptPrompt(vaultPath string) string {
	return prompts.Load("adversarial.md", vaultPath)
}
