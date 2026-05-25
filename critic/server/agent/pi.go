package agent

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
)

// Pi runs prompts through the `pi` CLI harness (https://pi.dev).
// Provider and model are per-call (with struct defaults). Sessions are
// maintained in-memory by accumulating message history; provider/model are
// pinned to the session at creation.
type Pi struct {
	DefaultProvider string
	DefaultModel    string

	mu       sync.Mutex
	sessions map[string]*piSession
	nextID   atomic.Int64
}

type piSession struct {
	turns    []piTurn
	provider string
	model    string
}

type piTurn struct {
	Role    string // "system", "user", "assistant"
	Content string
}

func NewPi(defaultProvider, defaultModel string) *Pi {
	return &Pi{
		DefaultProvider: defaultProvider,
		DefaultModel:    defaultModel,
		sessions:        make(map[string]*piSession),
	}
}

// Invoke runs a one-shot pi call.
func (p *Pi) Invoke(ctx context.Context, systemPrompt, userPrompt, provider, model string) (string, error) {
	prov, mod := p.resolve(provider, model)
	turns := []piTurn{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	return p.call(ctx, turns, prov, mod)
}

// StartSession runs the first turn and returns the response + a session ID.
func (p *Pi) StartSession(ctx context.Context, systemPrompt, userPrompt, provider, model string) (response string, sessionID string, err error) {
	prov, mod := p.resolve(provider, model)
	turns := []piTurn{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	response, err = p.call(ctx, turns, prov, mod)
	if err != nil {
		return "", "", err
	}

	turns = append(turns, piTurn{Role: "assistant", Content: response})
	sessionID = fmt.Sprintf("pi-%d", p.nextID.Add(1))

	p.mu.Lock()
	p.sessions[sessionID] = &piSession{turns: turns, provider: prov, model: mod}
	p.mu.Unlock()

	return response, sessionID, nil
}

// Resume continues a session with a new user message. Provider/model are
// pinned to the session at creation and cannot be changed mid-conversation.
func (p *Pi) Resume(ctx context.Context, sessionID, userPrompt string) (string, error) {
	p.mu.Lock()
	sess, ok := p.sessions[sessionID]
	if !ok {
		p.mu.Unlock()
		return "", fmt.Errorf("pi session %s not found", sessionID)
	}
	turns := append([]piTurn(nil), sess.turns...)
	turns = append(turns, piTurn{Role: "user", Content: userPrompt})
	prov, mod := sess.provider, sess.model
	p.mu.Unlock()

	response, err := p.call(ctx, turns, prov, mod)
	if err != nil {
		return "", err
	}

	p.mu.Lock()
	if existing, ok := p.sessions[sessionID]; ok {
		existing.turns = append(turns, piTurn{Role: "assistant", Content: response})
	}
	p.mu.Unlock()

	return response, nil
}

// ListModels runs `pi --list-models` and returns the raw output.
// Output format is whatever the pi CLI produces — callers should pass it
// straight back to the user.
func (p *Pi) ListModels(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "pi", "--list-models")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("pi --list-models: %w\nstderr: %s", err, stderr.String())
	}
	return strings.TrimSpace(string(out)), nil
}

func (p *Pi) resolve(provider, model string) (string, string) {
	if provider == "" {
		provider = p.DefaultProvider
	}
	if model == "" {
		model = p.DefaultModel
	}
	return provider, model
}

func (p *Pi) call(ctx context.Context, turns []piTurn, provider, model string) (string, error) {
	prompt := renderConversation(turns)

	args := []string{
		"-p", prompt,
		"--no-session",
		"--no-tools",
	}
	if provider != "" {
		args = append(args, "--provider", provider)
	}
	if model != "" {
		args = append(args, "--model", model)
	}

	cmd := exec.CommandContext(ctx, "pi", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("pi cli: %w\nstderr: %s", err, stderr.String())
	}

	text := strings.TrimSpace(string(out))
	if text == "" {
		return "", fmt.Errorf("pi returned empty response")
	}
	return text, nil
}

// renderConversation flattens a turn sequence into a single prompt string.
// Pi's -p mode is one-shot, so we have to inline the conversation.
func renderConversation(turns []piTurn) string {
	var b strings.Builder
	for i, t := range turns {
		switch t.Role {
		case "system":
			fmt.Fprintf(&b, "## System Instructions\n\n%s\n\n", t.Content)
		case "user":
			if i == 0 || (i > 0 && turns[i-1].Role == "system") {
				fmt.Fprintf(&b, "%s\n", t.Content)
			} else {
				fmt.Fprintf(&b, "\n\n## Follow-up\n\n%s\n", t.Content)
			}
		case "assistant":
			fmt.Fprintf(&b, "\n\n## Your Previous Response\n\n%s\n", t.Content)
		}
	}
	return b.String()
}
