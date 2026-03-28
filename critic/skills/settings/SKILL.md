---
name: settings
description: Configure the critic plugin — vault path, API keys, model selection, enable/disable providers. Use when the user wants to change critic settings.
disable-model-invocation: true
---

# Critic Settings

Configure the critic plugin. Settings are stored in the plugin data directory and persist across sessions.

Settings file location: `${CLAUDE_PLUGIN_DATA}/settings.json`

## Usage

To view current settings:
> /critic:settings

To update a setting:
> /critic:settings vault_path /Users/me/obsidian/vault
> /critic:settings gemini_api_key AIza...
> /critic:settings claude_enabled false

## Available Settings

| Setting | Description | Example |
|---------|-------------|---------|
| `vault_path` | Absolute path to your Obsidian vault | `/Users/me/obsidian/MyVault` |
| `claude_enabled` | Enable Claude reviewer (true/false) | `true` |
| `claude_model` | Claude model name | `claude-sonnet-4-6` |
| `anthropic_api_key` | Anthropic API key (omit to use system auth) | `sk-ant-...` |
| `codex_enabled` | Enable Codex reviewer (true/false) | `true` |
| `codex_model` | Codex model name | `gpt-5.4-codex` |
| `openai_api_key` | OpenAI API key (omit to use system auth) | `sk-...` |
| `gemini_enabled` | Enable Gemini reviewer (true/false) | `true` |
| `gemini_model` | Gemini model name | `gemini-2.5-flash` |
| `gemini_api_key` | Gemini API key (omit to use GEMINI_API_KEY env) | `AIza...` |

## Instructions

If $ARGUMENTS is empty, read the settings file at `${CLAUDE_PLUGIN_DATA}/settings.json` using the `read-settings` MCP tool and display the current settings to the user. Mask API key values — show only the last 4 characters.

If $ARGUMENTS contains a key and value (e.g., "vault_path /path/to/vault"), call the `write-setting` MCP tool with the key and value.

If $ARGUMENTS contains only a key (e.g., "vault_path"), show the current value of that key.
