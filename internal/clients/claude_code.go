package clients

import (
	"os/exec"
	"path/filepath"
)

type claudeCode struct{}

func (claudeCode) ID() string         { return "claude-code" }
func (claudeCode) Name() string       { return "Claude Code (CLI)" }
func (claudeCode) SupportsHTTP() bool { return true }
func (claudeCode) RequiresNPX() bool  { return false }

func (claudeCode) ConfigPath() string {
	return filepath.Join(home(), ".claude.json")
}

func (c claudeCode) Detect() bool {
	if fileExists(c.ConfigPath()) {
		return true
	}
	_, err := exec.LookPath("claude")
	return err == nil
}

func (c claudeCode) IsConnected(port int) bool {
	m, err := readJSON(c.ConfigPath())
	if err != nil {
		return false
	}
	servers, _ := m["mcpServers"].(map[string]any)
	entry, _ := servers[ServerKey].(map[string]any)
	url, _ := entry["url"].(string)
	return url == mcpURL(port)
}

func (c claudeCode) Connect(port int) error {
	m, err := readJSON(c.ConfigPath())
	if err != nil {
		return err
	}
	servers := ensureMap(m, "mcpServers")
	servers[ServerKey] = map[string]any{
		"type": "http",
		"url":  mcpURL(port),
	}
	return writeJSON(c.ConfigPath(), m)
}

func (c claudeCode) Disconnect() error {
	m, err := readJSON(c.ConfigPath())
	if err != nil {
		return err
	}
	if servers, ok := m["mcpServers"].(map[string]any); ok {
		delete(servers, ServerKey)
	}
	return writeJSON(c.ConfigPath(), m)
}
