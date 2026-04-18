package clients

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

type vscode struct{}

func (vscode) ID() string         { return "vscode" }
func (vscode) Name() string       { return "VS Code" }
func (vscode) SupportsHTTP() bool { return true }
func (vscode) RequiresNPX() bool  { return false }

func (vscode) ConfigPath() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(roamingAppData(), "Code", "User", "mcp.json")
	case "darwin":
		return filepath.Join(home(), "Library", "Application Support", "Code", "User", "mcp.json")
	}
	return filepath.Join(home(), ".config", "Code", "User", "mcp.json")
}

func (c vscode) Detect() bool {
	if dirExists(filepath.Dir(c.ConfigPath())) {
		return true
	}
	_, err := exec.LookPath("code")
	return err == nil
}

func (c vscode) IsConnected(port int) bool {
	m, err := readJSON(c.ConfigPath())
	if err != nil {
		return false
	}
	servers, _ := m["servers"].(map[string]any)
	entry, _ := servers[ServerKey].(map[string]any)
	url, _ := entry["url"].(string)
	return url == mcpURL(port)
}

func (c vscode) Connect(port int) error {
	m, err := readJSON(c.ConfigPath())
	if err != nil {
		return err
	}
	servers := ensureMap(m, "servers")
	servers[ServerKey] = map[string]any{
		"type": "http",
		"url":  mcpURL(port),
	}
	return writeJSON(c.ConfigPath(), m)
}

func (c vscode) Disconnect() error {
	m, err := readJSON(c.ConfigPath())
	if err != nil {
		return err
	}
	if servers, ok := m["servers"].(map[string]any); ok {
		delete(servers, ServerKey)
	}
	return writeJSON(c.ConfigPath(), m)
}
