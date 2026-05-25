package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jdp/critic/server/agent"
	"github.com/jdp/critic/server/prompts"
	"github.com/jdp/critic/server/vault"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// appendManuscript reads all chapters in order from `vaultPath`/story and
// appends them to userPrompt under a clear marker. Returns the augmented
// prompt; on read error, returns the original prompt unchanged.
func appendManuscript(userPrompt, vaultPath string) string {
	if vaultPath == "" {
		return userPrompt
	}
	v := vault.New(vaultPath)
	chapters, err := v.ReadAllChapters()
	if err != nil || len(chapters) == 0 {
		return userPrompt
	}
	var b strings.Builder
	b.WriteString(userPrompt)
	b.WriteString("\n\n=== MANUSCRIPT ===\n\n")
	for _, ch := range chapters {
		fmt.Fprintf(&b, "--- %s ---\n%s\n\n", ch.Name, ch.Content)
	}
	return b.String()
}

func optString(req mcp.CallToolRequest, key string) string {
	if v, ok := req.GetArguments()[key].(string); ok {
		return v
	}
	return ""
}

func makeInvokeCodexHandler(c *agent.Codex) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		systemPrompt, _ := req.RequireString("system_prompt")
		userPrompt, _ := req.RequireString("user_prompt")
		sessionID := optString(req, "session_id")
		userPrompt = appendManuscript(userPrompt, optString(req, "include_manuscript_from"))

		var response, newSession string
		var err error

		if sessionID != "" {
			response, err = c.Resume(ctx, sessionID, userPrompt)
			newSession = sessionID
		} else {
			response, newSession, err = c.RunSession(ctx, systemPrompt, userPrompt)
		}

		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("codex: %v", err)), nil
		}

		data, _ := json.Marshal(map[string]string{
			"response":   response,
			"session_id": newSession,
		})
		return mcp.NewToolResultText(string(data)), nil
	}
}

func makeInvokePiHandler(p *agent.Pi) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		systemPrompt, _ := req.RequireString("system_prompt")
		userPrompt, _ := req.RequireString("user_prompt")
		sessionID := optString(req, "session_id")
		provider := optString(req, "provider")
		model := optString(req, "model")
		userPrompt = appendManuscript(userPrompt, optString(req, "include_manuscript_from"))

		var response, newSession string
		var err error

		if sessionID != "" {
			response, err = p.Resume(ctx, sessionID, userPrompt)
			newSession = sessionID
		} else {
			response, newSession, err = p.StartSession(ctx, systemPrompt, userPrompt, provider, model)
		}

		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("pi: %v", err)), nil
		}

		data, _ := json.Marshal(map[string]string{
			"response":   response,
			"session_id": newSession,
		})
		return mcp.NewToolResultText(string(data)), nil
	}
}

func makePiListModelsHandler(p *agent.Pi) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		out, err := p.ListModels(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("pi list-models: %v", err)), nil
		}
		return mcp.NewToolResultText(out), nil
	}
}

func makeGetPromptHandler() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, _ := req.RequireString("name")
		vaultPath := optString(req, "vault")
		varsJSON := optString(req, "vars")

		var rendered string
		if varsJSON != "" {
			var data any
			if err := json.Unmarshal([]byte(varsJSON), &data); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("parse vars JSON: %v", err)), nil
			}
			rendered = prompts.Render(name, vaultPath, data)
		} else {
			rendered = prompts.Load(name, vaultPath)
		}
		return mcp.NewToolResultText(rendered), nil
	}
}
