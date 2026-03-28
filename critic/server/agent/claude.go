package agent

import (
	"context"
	"fmt"

	claudecode "github.com/jdp/critic/server/claudesdk"
)

// Claude runs prompts through Claude Code CLI via the claudesdk.
type Claude struct {
	Model  string
	APIKey string // optional override; empty = use system auth
}

func NewClaude(model, apiKey string) *Claude {
	return &Claude{Model: model, APIKey: apiKey}
}

func (c *Claude) baseOpts(systemPrompt string) []claudecode.Option {
	opts := []claudecode.Option{
		claudecode.WithSystemPrompt(systemPrompt),
		claudecode.WithMaxTurns(1),
		claudecode.WithPermissionMode(claudecode.PermissionModeBypassPermissions),
		claudecode.WithTools(), // no tools — pure text responder
	}
	if c.Model != "" {
		opts = append(opts, claudecode.WithModel(c.Model))
	}
	if c.APIKey != "" {
		opts = append(opts, claudecode.WithEnvVar("ANTHROPIC_API_KEY", c.APIKey))
	}
	return opts
}

func (c *Claude) Run(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	text, _, err := c.run(ctx, c.baseOpts(systemPrompt), userPrompt)
	return text, err
}

func (c *Claude) RunSession(ctx context.Context, systemPrompt, userPrompt string) (string, string, error) {
	return c.run(ctx, c.baseOpts(systemPrompt), userPrompt)
}

func (c *Claude) Resume(ctx context.Context, sessionID string, userPrompt string) (string, error) {
	opts := []claudecode.Option{
		claudecode.WithResume(sessionID),
		claudecode.WithMaxTurns(1),
		claudecode.WithPermissionMode(claudecode.PermissionModeBypassPermissions),
		claudecode.WithTools(),
	}
	if c.Model != "" {
		opts = append(opts, claudecode.WithModel(c.Model))
	}
	text, _, err := c.run(ctx, opts, userPrompt)
	return text, err
}

func (c *Claude) run(ctx context.Context, opts []claudecode.Option, userPrompt string) (string, string, error) {
	iter, err := claudecode.Query(ctx, userPrompt, opts...)
	if err != nil {
		return "", "", fmt.Errorf("claude query: %w", err)
	}

	var result string
	var sessionID string
	for {
		msg, err := iter.Next(ctx)
		if err != nil {
			break
		}
		if am, ok := msg.(*claudecode.AssistantMessage); ok {
			for _, block := range am.Content {
				if tb, ok := block.(*claudecode.TextBlock); ok {
					result += tb.Text
				}
			}
		}
		if rm, ok := msg.(*claudecode.ResultMessage); ok {
			sessionID = rm.SessionID
		}
	}

	if result == "" {
		return "", "", fmt.Errorf("claude returned empty response")
	}
	return result, sessionID, nil
}
