package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

//go:embed *.md
var defaults embed.FS

// Load reads a prompt file with no template processing. Resolution order:
//  1. <vaultPath>/prompts/<name> (author override)
//  2. $CLAUDE_PLUGIN_ROOT/prompts/<name> (plugin directory)
//  3. Embedded default (compiled into the binary)
func Load(name, vaultPath string) string {
	return resolve(name, vaultPath)
}

// Render reads a prompt file and executes it as a Go text/template with
// the given data. Use standard template syntax: {{.FieldName}}, {{if}}, etc.
func Render(name, vaultPath string, data any) string {
	raw := resolve(name, vaultPath)

	tmpl, err := template.New(name).Parse(raw)
	if err != nil {
		panic(fmt.Sprintf("prompts: parse template %s: %v", name, err))
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		panic(fmt.Sprintf("prompts: execute template %s: %v", name, err))
	}
	return buf.String()
}

func resolve(name, vaultPath string) string {
	// Vault override
	if vaultPath != "" {
		data, err := os.ReadFile(filepath.Join(vaultPath, "prompts", name))
		if err == nil && len(data) > 0 {
			return string(data)
		}
	}

	// Plugin directory
	pluginRoot := os.Getenv("CLAUDE_PLUGIN_ROOT")
	if pluginRoot != "" {
		data, err := os.ReadFile(filepath.Join(pluginRoot, "prompts", name))
		if err == nil && len(data) > 0 {
			return string(data)
		}
	}

	// Embedded default
	data, err := defaults.ReadFile(name)
	if err != nil {
		panic(fmt.Sprintf("missing embedded prompt: %s", name))
	}
	return string(data)
}
