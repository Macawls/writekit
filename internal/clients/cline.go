package clients

import (
	"path/filepath"
	"runtime"
)

type cline struct{}

func (cline) ID() string         { return "cline" }
func (cline) Name() string       { return "Cline (VS Code)" }
func (cline) SupportsHTTP() bool { return true }
func (cline) RequiresNPX() bool  { return false }

func (cline) ConfigPath() string {
	return clineLikePath("saoudrizwan.claude-dev", "cline_mcp_settings.json")
}

func (c cline) Detect() bool {
	return dirExists(filepath.Dir(c.ConfigPath()))
}

func (c cline) IsConnected(port int) bool { return clineLikeConnected(c.ConfigPath(), port) }
func (c cline) Connect(port int) error    { return clineLikeConnect(c.ConfigPath(), port) }
func (c cline) Disconnect() error         { return clineLikeDisconnect(c.ConfigPath()) }

func clineLikePath(extensionID, filename string) string {
	base := vscodeUserDir()
	return filepath.Join(base, "globalStorage", extensionID, "settings", filename)
}

func vscodeUserDir() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(roamingAppData(), "Code", "User")
	case "darwin":
		return filepath.Join(home(), "Library", "Application Support", "Code", "User")
	}
	return filepath.Join(home(), ".config", "Code", "User")
}

func clineLikeConnected(path string, port int) bool {
	m, err := readJSON(path)
	if err != nil {
		return false
	}
	servers, _ := m["mcpServers"].(map[string]any)
	entry, _ := servers[ServerKey].(map[string]any)
	url, _ := entry["url"].(string)
	return url == mcpURL(port)
}

func clineLikeConnect(path string, port int) error {
	m, err := readJSON(path)
	if err != nil {
		return err
	}
	servers := ensureMap(m, "mcpServers")
	servers[ServerKey] = map[string]any{
		"type": "streamableHttp",
		"url":  mcpURL(port),
	}
	return writeJSON(path, m)
}

func clineLikeDisconnect(path string) error {
	m, err := readJSON(path)
	if err != nil {
		return err
	}
	if servers, ok := m["mcpServers"].(map[string]any); ok {
		delete(servers, ServerKey)
	}
	return writeJSON(path, m)
}
