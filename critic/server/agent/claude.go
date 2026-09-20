package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Claude runs prompts through the Claude Code CLI in headless print mode.
// Sessions are real: `claude -p --output-format json` returns a session_id
// and `-r <id>` resumes it with full server-side context, so cross-review
// continuity does not need history replay (unlike Pi).
type Claude struct {
	Model string // empty = CLI default
}

func NewClaude(model string) *Claude {
	return &Claude{Model: model}
}

// claudeResult is the shape of `--output-format json` output.
type claudeResult struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	IsError   bool   `json:"is_error"`
	Result    string `json:"result"`
	SessionID string `json:"session_id"`
}

// RunSession starts a fresh headless session with the given system prompt
// and returns the response plus the session ID for later resume.
func (c *Claude) RunSession(ctx context.Context, systemPrompt, userPrompt string) (string, string, error) {
	args := []string{
		"-p",
		"--output-format", "json",
		"--system-prompt", systemPrompt,
		"--tools", "",
		"--disable-slash-commands",
	}
	if c.Model != "" {
		args = append(args, "--model", c.Model)
	}
	return c.run(ctx, args, userPrompt)
}

// Resume continues a prior session. The system prompt and conversation
// history persist server-side in the session; only the new user prompt is
// sent.
func (c *Claude) Resume(ctx context.Context, sessionID, userPrompt string) (string, error) {
	args := []string{
		"-p",
		"--output-format", "json",
		"-r", sessionID,
		"--tools", "",
		"--disable-slash-commands",
	}
	response, _, err := c.run(ctx, args, userPrompt)
	return response, err
}

func (c *Claude) run(ctx context.Context, args []string, userPrompt string) (string, string, error) {
	cmd := exec.CommandContext(ctx, "claude", args...)

	// Prompt goes via stdin: manuscripts can exceed comfortable argv sizes.
	cmd.Stdin = strings.NewReader(userPrompt)

	// Neutral working directory so CLAUDE.md project auto-discovery doesn't
	// pull unrelated repo context into the reviewer.
	cmd.Dir = os.TempDir()

	// The server itself runs inside a Claude Code session; scrub the nesting
	// markers so the child CLI starts clean.
	cmd.Env = scrubEnv(os.Environ(), "CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("claude cli: %w\nstderr: %s", err, truncate(stderr.String(), 2000))
	}

	var res claudeResult
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		return "", "", fmt.Errorf("claude cli: parse output: %w\nstdout: %s", err, truncate(stdout.String(), 2000))
	}
	if res.IsError {
		return "", "", fmt.Errorf("claude cli returned error result (%s): %s", res.Subtype, truncate(res.Result, 2000))
	}
	if strings.TrimSpace(res.Result) == "" {
		return "", "", fmt.Errorf("claude cli returned empty result")
	}
	return res.Result, res.SessionID, nil
}

func scrubEnv(env []string, keys ...string) []string {
	out := env[:0:0]
	for _, kv := range env {
		drop := false
		for _, k := range keys {
			if strings.HasPrefix(kv, k+"=") {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, kv)
		}
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…(truncated)"
}
