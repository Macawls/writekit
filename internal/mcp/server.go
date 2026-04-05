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

const writeKitInstructions = `You are helping the user manage their site on WriteKit. Pages can be standalone or organized into collections. Always write rich, well-structured Markdown content.

Content guidelines:
- Use headings (##, ###) to organize sections
- Use **bold** and *italic* for emphasis
- Use code blocks with language tags (` + "```go, ```python" + `, etc.) for any code — they render with syntax highlighting and a copy button
- Use blockquotes (>) for callouts and quotes
- Use bullet and numbered lists for structured information
- Use tables for comparative data
- Use links with descriptive text: [text](url)
- Use images where relevant: ![alt](url)

Advanced features:
- Callout blocks: Start a blockquote with [!NOTE], [!TIP], [!WARNING], or [!DANGER] for styled alert boxes
- Media embeds: Use <embed src="url" /> for YouTube, Spotify, SoundCloud, Twitter, or GitHub Gists
- D2 diagrams: Use ` + "```d2" + ` code blocks for architecture diagrams
- Footnotes: Use [^1] syntax for references

Workflow: Create pages as drafts first, share the preview URL, then publish when ready. Use collections to group related pages (docs, tutorials, series, etc.).

Visibility: Pages and collections can be public (default, visible to everyone), unlisted (accessible via URL but hidden from index/sitemap), or private (only visible to authenticated team members). Always ask the user whether content should be public, unlisted, or private before creating or publishing — never assume a visibility level.

Teams: Sites have team members with roles — owner (full control), editor (manage content), viewer (view private content). Use invite_member, remove_member, update_member_role, and list_members tools to manage the team.

Important: Never ask the user for tenant_id. It is auto-resolved from their account. Only include tenant_id if the user explicitly tells you which site to target, or if a tool returns an error about multiple sites.`

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
	}, &mcp.ServerOptions{
		Instructions: writeKitInstructions,
	})

	s.registerPageTools(mcpServer)
	s.registerCollectionTools(mcpServer)
	s.registerSettingsTools(mcpServer)
	s.registerTeamTools(mcpServer)
	s.registerResources(mcpServer)
	s.registerPrompts(mcpServer)

	s.mcpServer = mcpServer
	return s
}

func (s *Server) Handler() http.Handler {
	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return s.mcpServer
	}, nil)
}

func (s *Server) resolveTenant(userID string, tenantID string) (*tenant.DB, string, error) {
	tenants, err := s.PlatformDB.ListTenantsByMembership(context.Background(), userID)
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

var roleLevel = map[string]int{"viewer": 0, "editor": 1, "owner": 2}

func hasMinRole(actual, required string) bool {
	return roleLevel[actual] >= roleLevel[required]
}

func (s *Server) resolveTenantWithRole(ctx context.Context, userID, tenantID, minRole string) (*tenant.DB, string, error) {
	db, tid, err := s.resolveTenant(userID, tenantID)
	if err != nil {
		return nil, "", err
	}
	member, err := s.PlatformDB.GetTeamMember(ctx, tid, userID)
	if err != nil {
		return nil, "", errTenantNotFound
	}
	if !hasMinRole(member.Role, minRole) {
		return nil, "", &mcpError{msg: "you don't have permission for this action (requires " + minRole + " role)"}
	}
	return db, tid, nil
}

type mcpError struct {
	msg string
}

func (e *mcpError) Error() string { return e.msg }

var (
	errNoTenants       = &mcpError{"you don't have any sites yet — create one at the WriteKit dashboard"}
	errTenantNotFound  = &mcpError{"site not found or you don't have access to it"}
	errMultipleTenants = &mcpError{"you have multiple sites — please specify tenant_id"}
)

func (s *Server) buildPageURL(tenantID string, collectionID *string, pageSlug string) string {
	base := "https://" + tenantID + "." + s.Config.Host
	if collectionID != nil && *collectionID != "" {
		col, err := s.getCollectionSlug(tenantID, *collectionID)
		if err == nil {
			return base + "/" + col + "/" + pageSlug
		}
	}
	return base + "/" + pageSlug
}

func (s *Server) getCollectionSlug(tenantID, collectionID string) (string, error) {
	db, err := s.Pool.Get(tenantID)
	if err != nil {
		return "", err
	}
	col, err := db.GetCollection(context.Background(), collectionID)
	if err != nil {
		return "", err
	}
	return col.Slug, nil
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
