package clients

import (
	"os/exec"
	"path/filepath"
)

type opencode struct{}

func (opencode) ID() string         { return "opencode" }
func (opencode) Name() string       { return "OpenCode" }
func (opencode) SupportsHTTP() bool { return true }
func (opencode) RequiresNPX() bool  { return false }

func (opencode) ConfigPath() string {
	return filepath.Join(home(), ".config", "opencode", "opencode.json")
}

func (c opencode) Detect() bool {
	if fileExists(c.ConfigPath()) {
		return true
	}
	_, err := exec.LookPath("opencode")
	return err == nil
}

func (c opencode) IsConnected(port int) bool {
	m, err := readJSON(c.ConfigPath())
	if err != nil {
		return false
	}
	mcp, _ := m["mcp"].(map[string]any)
	entry, _ := mcp[ServerKey].(map[string]any)
	url, _ := entry["url"].(string)
	return url == mcpURL(port)
}

func (c opencode) Connect(port int) error {
	m, err := readJSON(c.ConfigPath())
	if err != nil {
		return err
	}
	mcp := ensureMap(m, "mcp")
	mcp[ServerKey] = map[string]any{
		"type":    "remote",
		"url":     mcpURL(port),
		"enabled": true,
	}
	return writeJSON(c.ConfigPath(), m)
}

func (c opencode) Disconnect() error {
	m, err := readJSON(c.ConfigPath())
	if err != nil {
		return err
	}
	if mcp, ok := m["mcp"].(map[string]any); ok {
		delete(mcp, ServerKey)
	}
	return writeJSON(c.ConfigPath(), m)
}
