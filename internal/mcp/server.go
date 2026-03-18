package mcp

import (
	"context"
	"net/http"

	"writekit/internal/config"
	"writekit/internal/events"
	"writekit/internal/platform"
	"writekit/internal/tenant"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Server struct {
	PlatformDB *platform.DB
	Pool       *tenant.Pool
	Config     *config.Config
	Bus        *events.Bus
	mcpServer  *mcp.Server
}

func New(platformDB *platform.DB, pool *tenant.Pool, cfg *config.Config, bus *events.Bus) *Server {
	s := &Server{
		PlatformDB: platformDB,
		Pool:       pool,
		Config:     cfg,
		Bus:        bus,
	}

	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "WriteKit",
		Version: "2.0.0",
	}, nil)

	s.registerTools(mcpServer)
	s.registerCommentTools(mcpServer)
	s.registerSettingsTools(mcpServer)
	s.registerResources(mcpServer)

	s.mcpServer = mcpServer
	return s
}

func (s *Server) Handler() http.Handler {
	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return s.mcpServer
	}, nil)
}

func (s *Server) resolveTenant(userID string, tenantID string) (*tenant.DB, string, error) {
	tenants, err := s.PlatformDB.ListTenantsByUser(context.Background(), userID)
	if err != nil {
		return nil, "", err
	}

	if len(tenants) == 0 {
		return nil, "", errNoTenants
	}

	var selectedID string
	if tenantID != "" {
		for _, t := range tenants {
			if t.ID == tenantID {
				selectedID = t.ID
				break
			}
		}
		if selectedID == "" {
			return nil, "", errTenantNotFound
		}
	} else if len(tenants) == 1 {
		selectedID = tenants[0].ID
	} else {
		return nil, "", errMultipleTenants
	}

	db, err := s.Pool.Get(selectedID)
	if err != nil {
		return nil, "", err
	}
	return db, selectedID, nil
}

type mcpError struct {
	msg string
}

func (e *mcpError) Error() string { return e.msg }

var (
	errNoTenants       = &mcpError{"you don't have any blogs yet — create one at the WriteKit dashboard"}
	errTenantNotFound  = &mcpError{"blog not found or you don't have access to it"}
	errMultipleTenants = &mcpError{"you have multiple blogs — please specify tenant_id"}
)

func (s *Server) buildPostURL(tenantID, slug string) string {
	return "https://" + tenantID + "." + s.Config.Host + "/posts/" + slug
}

func (s *Server) buildPreviewURL(tenantID, token string) string {
	return "https://" + tenantID + "." + s.Config.Host + "/preview/" + token
}

func toolError(msg string) *mcp.CallToolResult {
	r := &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
		IsError: true,
	}
	return r
}

func toolResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}
