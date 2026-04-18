package clients

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

type zed struct{}

func (zed) ID() string         { return "zed" }
func (zed) Name() string       { return "Zed" }
func (zed) SupportsHTTP() bool { return false }
func (zed) RequiresNPX() bool  { return true }

func (zed) ConfigPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(roamingAppData(), "Zed", "settings.json")
	}
	return filepath.Join(home(), ".config", "zed", "settings.json")
}

func (c zed) Detect() bool {
	if fileExists(c.ConfigPath()) {
		return true
	}
	if runtime.GOOS == "darwin" && dirExists("/Applications/Zed.app") {
		return true
	}
	_, err := exec.LookPath("zed")
	return err == nil
}

func (c zed) IsConnected(port int) bool {
	m, err := readJSON(c.ConfigPath())
	if err != nil {
		return false
	}
	servers, _ := m["context_servers"].(map[string]any)
	entry, _ := servers[ServerKey].(map[string]any)
	cmd, _ := entry["command"].(map[string]any)
	args, _ := cmd["args"].([]any)
	for _, a := range args {
		if s, ok := a.(string); ok && s == mcpURL(port) {
			return true
		}
	}
	return false
}

func (c zed) Connect(port int) error {
	if _, err := exec.LookPath("npx"); err != nil {
		return errNPXMissing
	}
	m, err := readJSON(c.ConfigPath())
	if err != nil {
		return err
	}
	servers := ensureMap(m, "context_servers")
	servers[ServerKey] = map[string]any{
		"command": map[string]any{
			"path": "npx",
			"args": mcpRemoteArgs(port),
		},
	}
	return writeJSON(c.ConfigPath(), m)
}

func (c zed) Disconnect() error {
	m, err := readJSON(c.ConfigPath())
	if err != nil {
		return err
	}
	if servers, ok := m["context_servers"].(map[string]any); ok {
		delete(servers, ServerKey)
	}
	return writeJSON(c.ConfigPath(), m)
}
