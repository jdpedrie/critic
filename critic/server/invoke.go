package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jdp/critic/server/agent"
	"github.com/jdp/critic/server/prompts"
	"github.com/jdp/critic/server/vault"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// appendManuscript assembles the manuscript at `vaultPath` and appends it to
// userPrompt under a clear marker. A failure is an error, not a no-op: a
// reviewer handed the prompt without the book still answers, confidently,
// about nothing.
func appendManuscript(userPrompt, vaultPath string) (string, error) {
	if vaultPath == "" {
		return userPrompt, nil
	}
	v, err := vault.New(vaultPath)
	if err != nil {
		return "", fmt.Errorf("include_manuscript_from: %w", err)
	}
	manuscript, err := v.ReadManuscript()
	if err != nil {
		return "", fmt.Errorf("include_manuscript_from: %w", err)
	}
	var b strings.Builder
	b.WriteString(userPrompt)
	b.WriteString("\n\n=== MANUSCRIPT ===\n\n")
	b.WriteString(manuscript)
	return b.String(), nil
}

func optString(req mcp.CallToolRequest, key string) string {
	if v, ok := req.GetArguments()[key].(string); ok {
		return v
	}
	return ""
}

func optFloat(req mcp.CallToolRequest, key string) (float64, bool) {
	switch v := req.GetArguments()[key].(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f, true
		}
	}
	return 0, false
}

// waitBudget is how long this call should block before handing back a job
// handle. Callers may shorten it (0 returns a handle immediately) or lengthen
// it up to maxWaitSeconds.
func waitBudget(req mcp.CallToolRequest) time.Duration {
	secs := float64(defaultWaitSeconds)
	if v, ok := optFloat(req, "wait_seconds"); ok {
		secs = v
	}
	if secs < 0 {
		secs = 0
	}
	if secs > maxWaitSeconds {
		secs = maxWaitSeconds
	}
	return time.Duration(secs * float64(time.Second))
}

// jobResult renders a job for the caller. A finished job keeps the original
// {response, session_id} shape, so callers that predate jobs still work; an
// unfinished one comes back as a handle to collect later.
func jobResult(st JobStatus) *mcp.CallToolResult {
	elapsed := int(st.Elapsed.Round(time.Second).Seconds())

	if st.State == JobFailed {
		return mcp.NewToolResultError(fmt.Sprintf("%s: %s", st.Kind, st.Err))
	}

	payload := map[string]any{
		"status":          string(st.State),
		"job_id":          st.ID,
		"elapsed_seconds": elapsed,
	}

	if st.State == JobDone {
		payload["response"] = st.Response
		payload["session_id"] = st.SessionID
		if st.Output != "" {
			payload["output_file"] = st.Output
		}
	} else {
		payload["note"] = fmt.Sprintf(
			"%s is still running after %ds. This is normal for a full-manuscript review. "+
				"The work continues on the server; it did not fail and it was not cancelled. "+
				"Collect it with invoke-status using this job_id.", st.Kind, elapsed)
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal job result: %v", err))
	}
	return mcp.NewToolResultText(string(data))
}

// runInvocation is the shared body of every invoke-* handler. The work starts
// as a job so it outlives this tool call, then the call waits out its budget,
// heartbeating, and answers with whatever is true at that point.
func runInvocation(
	ctx context.Context,
	req mcp.CallToolRequest,
	js *Jobs,
	kind string,
	fn func(context.Context) (string, string, error),
) (*mcp.CallToolResult, error) {
	job := js.Start(kind, fn)
	js.Wait(ctx, job, waitBudget(req), heartbeatFor(ctx, req, kind))
	return jobResult(job.Status()), nil
}

func makeInvokeClaudeHandler(c *agent.Claude, js *Jobs) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		systemPrompt, _ := req.RequireString("system_prompt")
		userPrompt, _ := req.RequireString("user_prompt")
		sessionID := optString(req, "session_id")
		userPrompt, err := appendManuscript(userPrompt, optString(req, "include_manuscript_from"))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return runInvocation(ctx, req, js, "claude", func(ctx context.Context) (string, string, error) {
			if sessionID != "" {
				response, err := c.Resume(ctx, sessionID, userPrompt)
				return response, sessionID, err
			}
			return c.RunSession(ctx, systemPrompt, userPrompt)
		})
	}
}

func makeInvokeCodexHandler(c *agent.Codex, js *Jobs) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		systemPrompt, _ := req.RequireString("system_prompt")
		userPrompt, _ := req.RequireString("user_prompt")
		sessionID := optString(req, "session_id")
		userPrompt, err := appendManuscript(userPrompt, optString(req, "include_manuscript_from"))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return runInvocation(ctx, req, js, "codex", func(ctx context.Context) (string, string, error) {
			if sessionID != "" {
				response, err := c.Resume(ctx, sessionID, userPrompt)
				return response, sessionID, err
			}
			return c.RunSession(ctx, systemPrompt, userPrompt)
		})
	}
}

func makeInvokePiHandler(p *agent.Pi, js *Jobs) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		systemPrompt, _ := req.RequireString("system_prompt")
		userPrompt, _ := req.RequireString("user_prompt")
		sessionID := optString(req, "session_id")
		provider := optString(req, "provider")
		model := optString(req, "model")
		userPrompt, err := appendManuscript(userPrompt, optString(req, "include_manuscript_from"))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return runInvocation(ctx, req, js, "pi", func(ctx context.Context) (string, string, error) {
			if sessionID != "" {
				response, err := p.Resume(ctx, sessionID, userPrompt)
				return response, sessionID, err
			}
			return p.StartSession(ctx, systemPrompt, userPrompt, provider, model)
		})
	}
}

// makeInvokeStatusHandler collects a job started by an invoke-* tool. With no
// job_id it lists what the server knows about, which is the way back in after
// a session loses track of an invocation.
func makeInvokeStatusHandler(js *Jobs) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := optString(req, "job_id")
		if id == "" {
			return listJobsResult(js)
		}

		job, ok := js.Get(id)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf(
				"no job %q. It may have been pruned (jobs are kept %s after they finish), "+
					"or the server restarted. Call invoke-status with no job_id to see what's known.",
				id, jobRetention)), nil
		}

		js.Wait(ctx, job, waitBudget(req), heartbeatFor(ctx, req, job.Status().Kind))
		return jobResult(job.Status()), nil
	}
}

func listJobsResult(js *Jobs) (*mcp.CallToolResult, error) {
	all := js.List()
	rows := make([]map[string]any, 0, len(all))
	for _, st := range all {
		row := map[string]any{
			"job_id":          st.ID,
			"kind":            st.Kind,
			"status":          string(st.State),
			"elapsed_seconds": int(st.Elapsed.Round(time.Second).Seconds()),
		}
		if st.Output != "" {
			row["output_file"] = st.Output
		}
		if st.Err != "" {
			row["error"] = st.Err
		}
		rows = append(rows, row)
	}

	data, err := json.Marshal(map[string]any{
		"jobs":     rows,
		"job_dir":  js.Dir(),
		"retained": jobRetention.String(),
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal job list: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
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
