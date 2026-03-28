package agent

import (
	"context"
	"fmt"

	"github.com/fanwenlin/codex-go-sdk/codex"
	"github.com/fanwenlin/codex-go-sdk/types"
)

// Codex runs prompts through the OpenAI Codex CLI via codex-go-sdk.
type Codex struct {
	Model  string
	client *codex.Codex
}

func NewCodex(model, apiKey string) *Codex {
	opts := types.CodexOptions{}
	if apiKey != "" {
		opts.ApiKey = apiKey
	}
	client := codex.NewCodex(opts)
	return &Codex{
		Model:  model,
		client: client,
	}
}

func (c *Codex) threadOpts() types.ThreadOptions {
	return types.ThreadOptions{
		Model:            c.Model,
		SandboxMode:      types.SandboxModeReadOnly,
		ApprovalPolicy:   types.ApprovalModeNever,
		SkipGitRepoCheck: true,
	}
}

func (c *Codex) Run(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	text, _, err := c.runThread(ctx, c.client.StartThread(c.threadOpts()), systemPrompt+"\n\n---\n\n"+userPrompt)
	return text, err
}

func (c *Codex) RunSession(ctx context.Context, systemPrompt, userPrompt string) (string, string, error) {
	return c.runThread(ctx, c.client.StartThread(c.threadOpts()), systemPrompt+"\n\n---\n\n"+userPrompt)
}

func (c *Codex) Resume(ctx context.Context, sessionID string, userPrompt string) (string, error) {
	thread := c.client.ResumeThread(sessionID, c.threadOpts())
	text, _, err := c.runThread(ctx, thread, userPrompt)
	return text, err
}

func (c *Codex) runThread(ctx context.Context, thread *codex.Thread, prompt string) (string, string, error) {
	turn, err := thread.Run(prompt, types.TurnOptions{
		Context: ctx,
	})
	if err != nil {
		return "", "", fmt.Errorf("codex run: %w", err)
	}

	var threadID string
	if id := thread.ID(); id != nil {
		threadID = *id
	}

	if turn.FinalResponse == "" {
		return "", threadID, fmt.Errorf("codex returned empty response")
	}
	return turn.FinalResponse, threadID, nil
}
