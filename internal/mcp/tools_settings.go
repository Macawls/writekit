package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"writekit/internal/auth"
	"writekit/internal/events"
	"writekit/internal/platform"
)

var subdomainRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)

func (s *Server) registerSettingsTools(mcpServer *mcpsdk.Server) {
	mcpServer.AddTool(&mcpsdk.Tool{
		Name: "get_settings",
		Description: `Get your site settings. Available settings:
- **title**: Your site's title (shown in header)
- **description**: A short description of your site`,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
		},
	}, s.getSettings)

	mcpServer.AddTool(&mcpsdk.Tool{
		Name:        "update_settings",
		Description: "Update site settings. Only send the settings you want to change.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title":       map[string]any{"type": "string", "description": "Site title"},
				"description": map[string]any{"type": "string", "description": "Site description"},
				"tenant_id":   map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
		},
	}, s.updateSettings)

	if !s.Config.Local {
		mcpServer.AddTool(&mcpsdk.Tool{
			Name:        "rename_subdomain",
			Description: "Rename your site's subdomain (e.g. my-site → new-name). The old URL will stop working immediately.",
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"new_subdomain"},
				"properties": map[string]any{
					"new_subdomain": map[string]any{"type": "string", "description": "New subdomain (lowercase letters, numbers, hyphens, 3-64 chars)"},
					"tenant_id":     map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
				},
			},
		}, s.renameSubdomain)
	}
}

func (s *Server) getSettings(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated — please sign in at the WriteKit website first"), nil
	}

	var args struct {
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	db, _, err := s.resolveTenant(user.ID, args.TenantID)
	if err != nil {
		return toolError(err.Error()), nil
	}

	settings, err := db.GetSettings(ctx)
	if err != nil {
		return toolError(fmt.Sprintf("failed to get settings: %v", err)), nil
	}

	var sb strings.Builder
	sb.WriteString("Current site settings:\n\n")
	for k, v := range settings {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", k, v))
	}

	return toolResult(sb.String()), nil
}

func (s *Server) updateSettings(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated — please sign in at the WriteKit website first"), nil
	}

	var args map[string]any
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	tenantID, _ := args["tenant_id"].(string)
	db, _, err := s.resolveTenantWithRole(ctx, user.ID, tenantID, "editor")
	if err != nil {
		return toolError(err.Error()), nil
	}

	updates := make(map[string]string)
	for _, key := range []string{"title", "description"} {
		if v, ok := args[key].(string); ok {
			updates[key] = v
		}
	}

	if len(updates) == 0 {
		return toolError("no settings to update"), nil
	}

	if err := db.UpdateSettings(ctx, updates); err != nil {
		return toolError(fmt.Sprintf("failed to update settings: %v", err)), nil
	}

	return toolResult("Settings updated."), nil
}

func (s *Server) renameSubdomain(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated — please sign in at the WriteKit website first"), nil
	}

	var args struct {
		NewSubdomain string `json:"new_subdomain"`
		TenantID     string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	if args.NewSubdomain == "" {
		return toolError("new_subdomain is required"), nil
	}

	if !subdomainRegex.MatchString(args.NewSubdomain) {
		return toolError("invalid subdomain: use only lowercase letters, numbers, and hyphens (3-64 chars)"), nil
	}

	_, oldID, err := s.resolveTenantWithRole(ctx, user.ID, args.TenantID, "owner")
	if err != nil {
		return toolError(err.Error()), nil
	}

	if oldID == args.NewSubdomain {
		return toolResult(fmt.Sprintf("Subdomain is already **%s**.", oldID)), nil
	}

	if ok, err := s.PlatformDB.SlugAvailable(ctx, args.NewSubdomain, oldID); err != nil {
		return toolError(fmt.Sprintf("failed to check availability: %v", err)), nil
	} else if !ok {
		return toolError("this subdomain is already taken"), nil
	}

	if err := s.Pool.Rename(oldID, args.NewSubdomain, func() error {
		return s.PlatformDB.RenameTenant(ctx, oldID, args.NewSubdomain)
	}); err != nil {
		if errors.Is(err, platform.ErrSlugTaken) {
			return toolError("this subdomain is already taken"), nil
		}
		return toolError(fmt.Sprintf("failed to rename: %v", err)), nil
	}

	s.Bus.Emit(events.Event{
		Type:     events.TenantRenamed,
		TenantID: args.NewSubdomain,
		Payload:  events.TenantRenamePayload{OldID: oldID, NewID: args.NewSubdomain},
	})

	return toolResult(fmt.Sprintf("Subdomain renamed from **%s** to **%s**. The old URL will redirect to the new one. Your site is now at https://%s.%s", oldID, args.NewSubdomain, args.NewSubdomain, s.Config.Host)), nil
}
