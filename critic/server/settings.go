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

func settingsPath() string {
	dir := os.Getenv("CLAUDE_PLUGIN_DATA")
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, "settings.json")
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
		"vault_path":      true,
		"claude_enabled":  true,
		"claude_model":    true,
		"anthropic_api_key": true,
		"codex_enabled":   true,
		"codex_model":     true,
		"openai_api_key":  true,
		"gemini_enabled":  true,
		"gemini_model":    true,
		"gemini_api_key":  true,
	}

	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, _ := req.RequireString("key")
		value, _ := req.RequireString("value")

		if !validKeys[key] {
			return mcp.NewToolResultError(fmt.Sprintf("unknown setting: %s", key)), nil
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
