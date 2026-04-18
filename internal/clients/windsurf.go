package clients

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

type windsurf struct{}

func (windsurf) ID() string         { return "windsurf" }
func (windsurf) Name() string       { return "Windsurf" }
func (windsurf) SupportsHTTP() bool { return true }
func (windsurf) RequiresNPX() bool  { return false }

func (windsurf) ConfigPath() string {
	return filepath.Join(home(), ".codeium", "windsurf", "mcp_config.json")
}

func (c windsurf) Detect() bool {
	if dirExists(filepath.Join(home(), ".codeium", "windsurf")) {
		return true
	}
	if runtime.GOOS == "darwin" && dirExists("/Applications/Windsurf.app") {
		return true
	}
	_, err := exec.LookPath("windsurf")
	return err == nil
}

func (c windsurf) IsConnected(port int) bool {
	m, err := readJSON(c.ConfigPath())
	if err != nil {
		return false
	}
	servers, _ := m["mcpServers"].(map[string]any)
	entry, _ := servers[ServerKey].(map[string]any)
	url, _ := entry["serverUrl"].(string)
	return url == mcpURL(port)
}

func (c windsurf) Connect(port int) error {
	m, err := readJSON(c.ConfigPath())
	if err != nil {
		return err
	}
	servers := ensureMap(m, "mcpServers")
	servers[ServerKey] = map[string]any{
		"serverUrl": mcpURL(port),
	}
	return writeJSON(c.ConfigPath(), m)
}

func (c windsurf) Disconnect() error {
	m, err := readJSON(c.ConfigPath())
	if err != nil {
		return err
	}
	if servers, ok := m["mcpServers"].(map[string]any); ok {
		delete(servers, ServerKey)
	}
	return writeJSON(c.ConfigPath(), m)
}
