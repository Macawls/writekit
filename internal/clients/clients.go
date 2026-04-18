package clients

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const ServerKey = "writekit"

type Client interface {
	ID() string
	Name() string
	ConfigPath() string
	SupportsHTTP() bool
	RequiresNPX() bool
	Detect() bool
	IsConnected(port int) bool
	Connect(port int) error
	Disconnect() error
}

type Info struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Detected     bool   `json:"detected"`
	Connected    bool   `json:"connected"`
	ConfigPath   string `json:"config_path"`
	SupportsHTTP bool   `json:"supports_http"`
	RequiresNPX  bool   `json:"requires_npx"`
}

func All() []Client {
	return []Client{
		&claudeCode{},
		&cursor{},
		&windsurf{},
		&vscode{},
		&opencode{},
		&claudeDesktop{},
		&zed{},
	}
}

func ByID(id string) Client {
	for _, c := range All() {
		if c.ID() == id {
			return c
		}
	}
	return nil
}

func Snapshot(port int) []Info {
	all := All()
	out := make([]Info, 0, len(all))
	for _, c := range all {
		out = append(out, Info{
			ID:           c.ID(),
			Name:         c.Name(),
			Detected:     c.Detect(),
			Connected:    c.IsConnected(port),
			ConfigPath:   c.ConfigPath(),
			SupportsHTTP: c.SupportsHTTP(),
			RequiresNPX:  c.RequiresNPX(),
		})
	}
	return out
}

func mcpURL(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d/mcp", port)
}

func home() string {
	h, _ := os.UserHomeDir()
	return h
}

func roamingAppData() string {
	if runtime.GOOS == "windows" {
		if v := os.Getenv("APPDATA"); v != "" {
			return v
		}
		return filepath.Join(home(), "AppData", "Roaming")
	}
	return ""
}

func readJSON(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if len(b) == 0 {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

func writeJSON(path string, m map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".writekit.tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func ensureMap(parent map[string]any, key string) map[string]any {
	if existing, ok := parent[key].(map[string]any); ok {
		return existing
	}
	m := map[string]any{}
	parent[key] = m
	return m
}

func mcpRemoteArgs(port int) []any {
	return []any{"-y", "mcp-remote", mcpURL(port)}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}
