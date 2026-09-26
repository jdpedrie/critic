package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// settingsPath resolves where settings live. CRITIC_DATA wins, then the
// harness-provided CLAUDE_PLUGIN_DATA, then a fixed path under the user's
// home. There is no relative fallback on purpose: defaulting to the working
// directory wrote settings to wherever the server happened to be started
// from, which silently lost them and failed outright when that directory was
// read-only.
func settingsPath() string {
	for _, env := range []string{"CRITIC_DATA", "CLAUDE_PLUGIN_DATA"} {
		if dir := os.Getenv(env); dir != "" {
			return filepath.Join(dir, "settings.json")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "settings.json")
	}
	return filepath.Join(home, ".claude", "plugins", "data", "critic-critic", "settings.json")
}

func readSettings() (map[string]string, error) {
	data, err := os.ReadFile(settingsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, err
	}
	var settings map[string]string
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}
	return settings, nil
}

func writeSettings(settings map[string]string) error {
	dir := filepath.Dir(settingsPath())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath(), data, 0o644)
}

func makeReadSettingsHandler() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		settings, err := readSettings()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("read settings: %v", err)), nil
		}
		if len(settings) == 0 {
			return mcp.NewToolResultText("No settings configured yet. Use /critic:settings <key> <value> to set values."), nil
		}
		data, _ := json.MarshalIndent(settings, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}

func makeWriteSettingHandler() server.ToolHandlerFunc {
	validKeys := map[string]bool{
		"vault_path":         true,
		"frame":              true,
		"codex_enabled":      true,
		"codex_model":        true,
		"openai_api_key":     true,
		"pi_enabled":         true,
		"pi_provider":        true,
		"pi_model":           true,
		"adversary_provider": true,
		"adversary_model":    true,
		"claude_enabled":     true,
		"claude_model":       true,
	}

	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, _ := req.RequireString("key")
		value, _ := req.RequireString("value")

		if !validKeys[key] {
			return mcp.NewToolResultError(fmt.Sprintf("unknown setting: %s", key)), nil
		}
		if key == "frame" && value != "craft" && value != "publication" {
			return mcp.NewToolResultError("frame must be \"craft\" or \"publication\""), nil
		}

		settings, err := readSettings()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("read settings: %v", err)), nil
		}

		settings[key] = value
		if err := writeSettings(settings); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("write settings: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Set %s. Restart the plugin for changes to take effect.", key)), nil
	}
}
