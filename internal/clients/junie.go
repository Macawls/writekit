package clients

import (
	"path/filepath"
	"slices"
)

type junie struct{}

func (junie) ID() string         { return "junie" }
func (junie) Name() string       { return "JetBrains AI Assistant / Junie" }
func (junie) SupportsHTTP() bool { return true }
func (junie) RequiresNPX() bool  { return false }

func (junie) ConfigPath() string {
	return filepath.Join(home(), ".junie", "mcp", "mcp.json")
}

func (c junie) Detect() bool {
	if dirExists(filepath.Join(home(), ".junie")) {
		return true
	}
	return hasJetBrainsIDE()
}

func (c junie) IsConnected(port int) bool {
	m, err := readJSON(c.ConfigPath())
	if err != nil {
		return false
	}
	servers, _ := m["mcpServers"].(map[string]any)
	entry, _ := servers[ServerKey].(map[string]any)
	url, _ := entry["url"].(string)
	return url == mcpURL(port)
}

func (c junie) Connect(port int) error {
	m, err := readJSON(c.ConfigPath())
	if err != nil {
		return err
	}
	servers := ensureMap(m, "mcpServers")
	servers[ServerKey] = map[string]any{
		"url": mcpURL(port),
	}
	return writeJSON(c.ConfigPath(), m)
}

func (c junie) Disconnect() error {
	m, err := readJSON(c.ConfigPath())
	if err != nil {
		return err
	}
	if servers, ok := m["mcpServers"].(map[string]any); ok {
		delete(servers, ServerKey)
	}
	return writeJSON(c.ConfigPath(), m)
}

func hasJetBrainsIDE() bool {
	return slices.ContainsFunc(jetBrainsConfigDirs(), dirExists)
}

func jetBrainsConfigDirs() []string {
	return []string{
		filepath.Join(home(), "Library", "Application Support", "JetBrains"),
		filepath.Join(home(), ".config", "JetBrains"),
		filepath.Join(roamingAppData(), "JetBrains"),
	}
}
