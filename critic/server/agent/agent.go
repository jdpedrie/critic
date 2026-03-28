package agent

import "context"

// Agent runs a single prompt through an LLM and returns the text response.
type Agent interface {
	Run(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// SessionAgent can run prompts and resume previous sessions.
type SessionAgent interface {
	Agent
	// RunSession runs a prompt and returns the response + a session ID for resumption.
	RunSession(ctx context.Context, systemPrompt, userPrompt string) (response string, sessionID string, err error)
	// Resume sends a follow-up prompt to an existing session.
	Resume(ctx context.Context, sessionID string, userPrompt string) (string, error)
}
