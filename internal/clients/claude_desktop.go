package clients

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

type claudeDesktop struct{}

func (claudeDesktop) ID() string         { return "claude-desktop" }
func (claudeDesktop) Name() string       { return "Claude Desktop" }
func (claudeDesktop) SupportsHTTP() bool { return false }
func (claudeDesktop) RequiresNPX() bool  { return true }

func (claudeDesktop) ConfigPath() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(roamingAppData(), "Claude", "claude_desktop_config.json")
	case "darwin":
		return filepath.Join(home(), "Library", "Application Support", "Claude", "claude_desktop_config.json")
	}
	return filepath.Join(home(), ".config", "Claude", "claude_desktop_config.json")
}

func (c claudeDesktop) Detect() bool {
	if dirExists(filepath.Dir(c.ConfigPath())) {
		return true
	}
	switch runtime.GOOS {
	case "darwin":
		return dirExists("/Applications/Claude.app")
	case "windows":
		return dirExists(filepath.Join(home(), "AppData", "Local", "AnthropicClaude"))
	}
	return false
}

func (c claudeDesktop) IsConnected(port int) bool {
	m, err := readJSON(c.ConfigPath())
	if err != nil {
		return false
	}
	servers, _ := m["mcpServers"].(map[string]any)
	entry, _ := servers[ServerKey].(map[string]any)
	args, _ := entry["args"].([]any)
	for _, a := range args {
		if s, ok := a.(string); ok && s == mcpURL(port) {
			return true
		}
	}
	return false
}

func (c claudeDesktop) Connect(port int) error {
	if _, err := exec.LookPath("npx"); err != nil {
		return errNPXMissing
	}
	m, err := readJSON(c.ConfigPath())
	if err != nil {
		return err
	}
	servers := ensureMap(m, "mcpServers")
	servers[ServerKey] = map[string]any{
		"command": "npx",
		"args":    mcpRemoteArgs(port),
	}
	return writeJSON(c.ConfigPath(), m)
}

func (c claudeDesktop) Disconnect() error {
	m, err := readJSON(c.ConfigPath())
	if err != nil {
		return err
	}
	if servers, ok := m["mcpServers"].(map[string]any); ok {
		delete(servers, ServerKey)
	}
	return writeJSON(c.ConfigPath(), m)
}
