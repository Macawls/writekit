package clients

import "path/filepath"

type rooCode struct{}

func (rooCode) ID() string         { return "roo-code" }
func (rooCode) Name() string       { return "Roo Code (VS Code)" }
func (rooCode) SupportsHTTP() bool { return true }
func (rooCode) RequiresNPX() bool  { return false }

func (rooCode) ConfigPath() string {
	return clineLikePath("rooveterinaryinc.roo-cline", "mcp_settings.json")
}

func (c rooCode) Detect() bool {
	return dirExists(filepath.Dir(c.ConfigPath()))
}

func (c rooCode) IsConnected(port int) bool { return clineLikeConnected(c.ConfigPath(), port) }
func (c rooCode) Connect(port int) error    { return clineLikeConnect(c.ConfigPath(), port) }
func (c rooCode) Disconnect() error         { return clineLikeDisconnect(c.ConfigPath()) }
