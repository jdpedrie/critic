package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	claudecode "github.com/jdp/critic/server/claudesdk"
)

// Claude runs prompts through Claude Code CLI via the claudesdk.
type Claude struct {
	Model     string
	APIKey    string // optional override; empty = use system auth
	VaultPath string // set to enable read-chapter tool
}

func NewClaude(model, apiKey string) *Claude {
	return &Claude{Model: model, APIKey: apiKey}
}

func (c *Claude) baseOpts(systemPrompt string) []claudecode.Option {
	opts := []claudecode.Option{
		claudecode.WithSystemPrompt(systemPrompt),
		claudecode.WithPermissionMode(claudecode.PermissionModeBypassPermissions),
	}
	if c.Model != "" {
		opts = append(opts, claudecode.WithModel(c.Model))
	}
	if c.APIKey != "" {
		opts = append(opts, claudecode.WithEnvVar("ANTHROPIC_API_KEY", c.APIKey))
	}

	// If vault path is set, give the agent a read_chapter tool and enough turns to use it.
	if c.VaultPath != "" {
		tool := claudecode.NewTool(
			"read_chapter",
			"Read the full text of a chapter by name (e.g. chapter-01). Use this when you need more detail than the summary provides.",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Chapter name (e.g. chapter-01)",
					},
				},
				"required": []string{"name"},
			},
			c.readChapterHandler(),
		)
		server := claudecode.CreateSDKMcpServer("vault", "1.0.0", tool)
		opts = append(opts,
			claudecode.WithSdkMcpServer("vault", server),
			claudecode.WithAllowedTools("mcp__vault__read_chapter"),
			claudecode.WithMaxTurns(5),
		)
	} else {
		opts = append(opts,
			claudecode.WithTools(), // no tools
			claudecode.WithMaxTurns(1),
		)
	}

	return opts
}

func (c *Claude) readChapterHandler() func(ctx context.Context, args map[string]any) (*claudecode.McpToolResult, error) {
	return func(ctx context.Context, args map[string]any) (*claudecode.McpToolResult, error) {
		name, _ := args["name"].(string)
		if name == "" {
			return &claudecode.McpToolResult{
				Content: []claudecode.McpContent{{Type: "text", Text: "chapter name is required"}},
				IsError: true,
			}, nil
		}
		if filepath.Ext(name) == "" {
			name = name + ".md"
		}
		path := filepath.Join(c.VaultPath, "story", name)
		data, err := os.ReadFile(path)
		if err != nil {
			return &claudecode.McpToolResult{
				Content: []claudecode.McpContent{{Type: "text", Text: fmt.Sprintf("read chapter: %v", err)}},
				IsError: true,
			}, nil
		}
		return &claudecode.McpToolResult{
			Content: []claudecode.McpContent{{Type: "text", Text: string(data)}},
		}, nil
	}
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
		claudecode.WithMaxTurns(5),
		claudecode.WithPermissionMode(claudecode.PermissionModeBypassPermissions),
	}
	if c.Model != "" {
		opts = append(opts, claudecode.WithModel(c.Model))
	}
	// Resume with tool access if vault path is set
	if c.VaultPath != "" {
		tool := claudecode.NewTool(
			"read_chapter",
			"Read the full text of a chapter by name.",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string", "description": "Chapter name"},
				},
				"required": []string{"name"},
			},
			c.readChapterHandler(),
		)
		server := claudecode.CreateSDKMcpServer("vault", "1.0.0", tool)
		opts = append(opts,
			claudecode.WithSdkMcpServer("vault", server),
			claudecode.WithAllowedTools("mcp__vault__read_chapter"),
		)
	} else {
		opts = append(opts, claudecode.WithTools())
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
