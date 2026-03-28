package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	VaultPath string      `yaml:"-"`
	Models    ModelConfig  `yaml:"models"`
	Claude   ClaudeConfig `yaml:"claude"`
	Codex    CodexConfig  `yaml:"codex"`
	Gemini   GeminiConfig `yaml:"gemini"`
	Review   ReviewConfig `yaml:"review"`
	Memory   MemoryConfig `yaml:"memory"`
}

type ModelConfig struct {
	TextAnalytical  string `yaml:"text_analytical"`
	TextImmersive   string `yaml:"text_immersive"`
	FullStructural  string `yaml:"full_structural"`
	FullAdversarial string `yaml:"full_adversarial"`
	Synthesizer     string `yaml:"synthesizer"`
}

type ClaudeConfig struct {
	Model   string `yaml:"model"`
	Enabled bool
}

type CodexConfig struct {
	Model   string `yaml:"model"`
	Enabled bool
}

type GeminiConfig struct {
	Model   string `yaml:"model"`
	Enabled bool
}

type ReviewConfig struct {
	MaxIssues          int    `yaml:"max_issues"`
	MaxNewIssuesRound2 int    `yaml:"max_new_issues_round2"`
	PriorChapters      int    `yaml:"prior_chapters"`
	CanonRetrieval     string `yaml:"canon_retrieval"`
}

type MemoryConfig struct {
	CompressThreshold int `yaml:"compress_threshold"`
	MaxOpenIssues     int `yaml:"max_open_issues"`
}

func LoadConfig(path string) (*Config, error) {
	cfg := &Config{
		Claude: ClaudeConfig{Model: "claude-sonnet-4-6"},
		Codex:  CodexConfig{Model: "gpt-5.4-codex"},
		Gemini: GeminiConfig{Model: "gemini-2.5-flash"},
		Models: ModelConfig{
			TextAnalytical:  "claude",
			TextImmersive:   "codex",
			FullStructural:  "claude",
			FullAdversarial: "codex",
			Synthesizer:     "claude",
		},
		Review: ReviewConfig{
			MaxIssues:          7,
			MaxNewIssuesRound2: 3,
			PriorChapters:      2,
			CanonRetrieval:     "keyword",
		},
		Memory: MemoryConfig{
			CompressThreshold: 2000,
			MaxOpenIssues:     20,
		},
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
	if v := settingOrEnv(ps, "claude_model"); v != "" {
		cfg.Claude.Model = v
	}
	if v := settingOrEnv(ps, "codex_model"); v != "" {
		cfg.Codex.Model = v
	}
	if v := settingOrEnv(ps, "gemini_model"); v != "" {
		cfg.Gemini.Model = v
	}

	// Vault path
	if v := settingOrEnv(ps, "vault_path"); v != "" {
		cfg.VaultPath = v
	}

	// Enable/disable — default to true unless explicitly "false".
	cfg.Claude.Enabled = settingOrEnvBool(ps, "claude_enabled", true)
	cfg.Codex.Enabled = settingOrEnvBool(ps, "codex_enabled", true)
	cfg.Gemini.Enabled = settingOrEnvBool(ps, "gemini_enabled", true)

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

// pluginOpt reads a CLAUDE_PLUGIN_OPTION_<key> env var.
func pluginOpt(key string) string {
	return os.Getenv("CLAUDE_PLUGIN_OPTION_" + key)
}
