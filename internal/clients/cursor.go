package clients

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

type cursor struct{}

func (cursor) ID() string         { return "cursor" }
func (cursor) Name() string       { return "Cursor" }
func (cursor) SupportsHTTP() bool { return true }
func (cursor) RequiresNPX() bool  { return false }

func (cursor) ConfigPath() string {
	return filepath.Join(home(), ".cursor", "mcp.json")
}

func (c cursor) Detect() bool {
	if dirExists(filepath.Join(home(), ".cursor")) {
		return true
	}
	if runtime.GOOS == "darwin" && dirExists("/Applications/Cursor.app") {
		return true
	}
	_, err := exec.LookPath("cursor")
	return err == nil
}

func (c cursor) IsConnected(port int) bool {
	m, err := readJSON(c.ConfigPath())
	if err != nil {
		return false
	}
	servers, _ := m["mcpServers"].(map[string]any)
	entry, _ := servers[ServerKey].(map[string]any)
	url, _ := entry["url"].(string)
	return url == mcpURL(port)
}

func (c cursor) Connect(port int) error {
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

func (c cursor) Disconnect() error {
	m, err := readJSON(c.ConfigPath())
	if err != nil {
		return err
	}
	if servers, ok := m["mcpServers"].(map[string]any); ok {
		delete(servers, ServerKey)
	}
	return writeJSON(c.ConfigPath(), m)
}
