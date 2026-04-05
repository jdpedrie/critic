package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Gemini runs prompts through the Gemini CLI in headless mode.
type Gemini struct {
	Model string
}

func NewGemini(model string) *Gemini {
	return &Gemini{Model: model}
}

type geminiOutput struct {
	Response  string `json:"response"`
	SessionID string `json:"session_id"`
	Error     string `json:"error"`
}

func (g *Gemini) run(ctx context.Context, prompt string, extraArgs ...string) (geminiOutput, error) {
	args := []string{
		"-p", prompt,
		"--output-format", "json",
		"--sandbox",
	}
	if g.Model != "" {
		args = append(args, "-m", g.Model)
	}
	args = append(args, extraArgs...)

	cmd := exec.CommandContext(ctx, "gemini", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return geminiOutput{}, fmt.Errorf("gemini cli: %w\nstderr: %s", err, string(exitErr.Stderr))
		}
		return geminiOutput{}, fmt.Errorf("gemini cli: %w", err)
	}

	var result geminiOutput
	if err := json.Unmarshal(out, &result); err != nil {
		// If JSON parse fails, treat raw output as the response
		text := strings.TrimSpace(string(out))
		if text != "" {
			return geminiOutput{Response: text}, nil
		}
		return geminiOutput{}, fmt.Errorf("gemini: parse output: %w\nraw: %s", err, string(out))
	}

	if result.Error != "" {
		return geminiOutput{}, fmt.Errorf("gemini: %s", result.Error)
	}

	result.Response = strings.TrimSpace(result.Response)
	if result.Response == "" {
		return geminiOutput{}, fmt.Errorf("gemini returned empty response")
	}
	return result, nil
}

func (g *Gemini) Run(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	result, err := g.run(ctx, systemPrompt+"\n\n---\n\n"+userPrompt)
	return result.Response, err
}

func (g *Gemini) RunSession(ctx context.Context, systemPrompt, userPrompt string) (string, string, error) {
	result, err := g.run(ctx, systemPrompt+"\n\n---\n\n"+userPrompt)
	if err != nil {
		return "", "", err
	}
	return result.Response, result.SessionID, nil
}

func (g *Gemini) Resume(ctx context.Context, sessionID string, userPrompt string) (string, error) {
	result, err := g.run(ctx, userPrompt, "--resume", sessionID)
	return result.Response, err
}
