---
name: settings
description: Configure the critic plugin. Vault path, API keys, model selection, enable/disable providers. Use when the user wants to change critic settings.
disable-model-invocation: true
---

# Critic Settings

Configure the critic plugin. Settings are stored in the plugin data directory and persist across sessions.

Settings file location: `${CLAUDE_PLUGIN_DATA}/settings.json`

## Usage

To view current settings:
> /critic:settings

To update a setting:
> /critic:settings vault_path /Users/me/obsidian/MyVault/MyNovel
> /critic:settings pi_provider google
> /critic:settings codex_enabled false

## Available Settings

| Setting | Description | Example |
|---------|-------------|---------|
| `vault_path` | Absolute path to the storyline project base folder (the folder containing `Scenes/`, `Codex/`, `Research/`, etc.) | `/Users/me/obsidian/MyVault/MyNovel` |
| `codex_enabled` | Enable the Codex reviewer (true/false). Default true. | `true` |
| `codex_model` | Codex model name (omit to let the Codex CLI pick whatever the active subscription supports) | `gpt-5-codex` |
| `openai_api_key` | OpenAI API key (omit to use Codex CLI's login) | `sk-...` |
| `pi_enabled` | Enable the Pi reviewer (true/false). Default true. | `true` |
| `pi_provider` | Default Pi provider (`anthropic`, `openai`, `google`, etc.) | `google` |
| `pi_model` | Default Pi model. Skills can override per-call. | `gemini-2.5-pro` |

Claude is always available. It runs as a Task subagent inside cowork. Its model is whatever cowork is running on. There's nothing to configure.

## Instructions

If $ARGUMENTS is empty, read the settings file at `${CLAUDE_PLUGIN_DATA}/settings.json` using the `read-settings` MCP tool and display the current settings to the user. Mask API key values. Show only the last 4 characters.

If $ARGUMENTS contains a key and value (e.g., "vault_path /path/to/vault"), call the `write-setting` MCP tool with the key and value.

If $ARGUMENTS contains only a key (e.g., "vault_path"), show the current value of that key.
