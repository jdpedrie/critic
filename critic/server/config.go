package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	VaultPath string      `yaml:"-"`
	Codex     CodexConfig `yaml:"codex"`
	Pi        PiConfig    `yaml:"pi"`

	// Legacy fields kept for backward compatibility with older config.yaml
	// files. They are not consulted by any agent or tool.
	Claude      LegacyConfig `yaml:"claude,omitempty"`
	Gemini      LegacyConfig `yaml:"gemini,omitempty"`
	Adversarial LegacyConfig `yaml:"adversarial,omitempty"`
}

type CodexConfig struct {
	Model   string `yaml:"model"`
	Enabled bool
}

type PiConfig struct {
	Provider string `yaml:"provider"` // anthropic | openai | google
	Model    string `yaml:"model"`
	Enabled  bool
}

// LegacyConfig is a permissive shape that swallows old config sections
// (gemini, adversarial) without failing to parse.
type LegacyConfig struct {
	Model   string `yaml:"model,omitempty"`
	BaseURL string `yaml:"base_url,omitempty"`
	APIKey  string `yaml:"api_key,omitempty"`
}

func LoadConfig(path string) (*Config, error) {
	cfg := &Config{
		Codex: CodexConfig{Model: ""}, // empty = let Codex CLI pick
		Pi:    PiConfig{Provider: "google", Model: ""},
	}

	// Config file is optional — defaults above are sufficient.
	data, err := os.ReadFile(path)
	if err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}

	// Load persistent settings from plugin data directory.
	// These override config.yaml values.
	ps, _ := readSettings()

	// Model overrides: settings file > CLAUDE_PLUGIN_OPTION_ env > config.yaml
	if v := settingOrEnv(ps, "codex_model"); v != "" {
		cfg.Codex.Model = v
	}
	if v := settingOrEnv(ps, "pi_provider"); v != "" {
		cfg.Pi.Provider = v
	}
	if v := settingOrEnv(ps, "pi_model"); v != "" {
		cfg.Pi.Model = v
	}

	// Vault path
	if v := settingOrEnv(ps, "vault_path"); v != "" {
		cfg.VaultPath = v
	}

	// Enable/disable — default to true unless explicitly "false".
	cfg.Codex.Enabled = settingOrEnvBool(ps, "codex_enabled", true)
	cfg.Pi.Enabled = settingOrEnvBool(ps, "pi_enabled", true)

	return cfg, nil
}

// settingOrEnv checks the persistent settings file first, then
// CLAUDE_PLUGIN_OPTION_<key> env var. Returns empty string if neither is set.
func settingOrEnv(ps map[string]string, key string) string {
	if v, ok := ps[key]; ok && v != "" {
		return v
	}
	return os.Getenv("CLAUDE_PLUGIN_OPTION_" + key)
}

// settingOrEnvBool reads a setting as a bool. Returns defaultVal if unset.
func settingOrEnvBool(ps map[string]string, key string, defaultVal bool) bool {
	v := settingOrEnv(ps, key)
	if v == "" {
		return defaultVal
	}
	return strings.EqualFold(v, "true") || v == "1"
}
