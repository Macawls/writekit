package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"writekit/internal/auth"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerTeamTools(mcpServer *mcpsdk.Server) {
	mcpServer.AddTool(&mcpsdk.Tool{
		Name:        "list_members",
		Description: "List all team members for your site.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
		},
	}, s.listMembers)

	mcpServer.AddTool(&mcpsdk.Tool{
		Name:        "invite_member",
		Description: "Invite a user to your site's team by email. They must already have a WriteKit account.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"email":     map[string]any{"type": "string", "description": "Email address of the user to invite"},
				"role":      map[string]any{"type": "string", "enum": []string{"editor", "viewer"}, "description": "Role: editor (can manage content) or viewer (can view private content)"},
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
			"required": []string{"email", "role"},
		},
	}, s.inviteMember)

	mcpServer.AddTool(&mcpsdk.Tool{
		Name:        "remove_member",
		Description: "Remove a team member from your site. Cannot remove the last owner.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"email":     map[string]any{"type": "string", "description": "Email of the member to remove"},
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
			"required": []string{"email"},
		},
	}, s.removeMember)

	mcpServer.AddTool(&mcpsdk.Tool{
		Name:        "update_member_role",
		Description: "Change a team member's role. Cannot demote the last owner.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"email":     map[string]any{"type": "string", "description": "Email of the member"},
				"role":      map[string]any{"type": "string", "enum": []string{"owner", "editor", "viewer"}, "description": "New role"},
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
			"required": []string{"email", "role"},
		},
	}, s.updateMemberRole)
}

func (s *Server) listMembers(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated — please sign in at the WriteKit website first"), nil
	}

	var args struct {
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	_, tenantID, err := s.resolveTenant(user.ID, args.TenantID)
	if err != nil {
		return toolError(err.Error()), nil
	}

	members, err := s.PlatformDB.ListTeamMembers(ctx, tenantID)
	if err != nil {
		return toolError(fmt.Sprintf("failed to list members: %v", err)), nil
	}

	if len(members) == 0 {
		return toolResult("No team members found."), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Team (%d member(s)):\n\n", len(members)))
	for _, m := range members {
		name := m.Name
		if name == "" {
			name = m.Email
		}
		sb.WriteString(fmt.Sprintf("- **%s** <%s> — %s\n", name, m.Email, m.Role))
	}
	return toolResult(sb.String()), nil
}

func (s *Server) inviteMember(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated — please sign in at the WriteKit website first"), nil
	}

	var args struct {
		Email    string `json:"email"`
		Role     string `json:"role"`
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	_, tenantID, err := s.resolveTenantWithRole(ctx, user.ID, args.TenantID, "owner")
	if err != nil {
		return toolError(err.Error()), nil
	}

	invitee, err := s.PlatformDB.GetUserByEmail(ctx, args.Email)
	if err != nil {
		return toolError(fmt.Sprintf("no WriteKit account found for %s — they need to sign up first", args.Email)), nil
	}

	if err := s.PlatformDB.AddTeamMember(ctx, tenantID, invitee.ID, args.Role); err != nil {
		return toolError(fmt.Sprintf("failed to add member: %v", err)), nil
	}

	return toolResult(fmt.Sprintf("**%s** (%s) added as **%s**.", invitee.Name, invitee.Email, args.Role)), nil
}

func (s *Server) removeMember(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated — please sign in at the WriteKit website first"), nil
	}

	var args struct {
		Email    string `json:"email"`
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	_, tenantID, err := s.resolveTenantWithRole(ctx, user.ID, args.TenantID, "owner")
	if err != nil {
		return toolError(err.Error()), nil
	}

	target, err := s.PlatformDB.GetUserByEmail(ctx, args.Email)
	if err != nil {
		return toolError("user not found"), nil
	}

	if err := s.PlatformDB.RemoveTeamMember(ctx, tenantID, target.ID); err != nil {
		return toolError(err.Error()), nil
	}

	return toolResult(fmt.Sprintf("**%s** removed from team.", args.Email)), nil
}

func (s *Server) updateMemberRole(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated — please sign in at the WriteKit website first"), nil
	}

	var args struct {
		Email    string `json:"email"`
		Role     string `json:"role"`
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	_, tenantID, err := s.resolveTenantWithRole(ctx, user.ID, args.TenantID, "owner")
	if err != nil {
		return toolError(err.Error()), nil
	}

	target, err := s.PlatformDB.GetUserByEmail(ctx, args.Email)
	if err != nil {
		return toolError("user not found"), nil
	}

	if err := s.PlatformDB.UpdateTeamMemberRole(ctx, tenantID, target.ID, args.Role); err != nil {
		return toolError(err.Error()), nil
	}

	return toolResult(fmt.Sprintf("**%s** role changed to **%s**.", args.Email, args.Role)), nil
}
