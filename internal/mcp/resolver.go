package mcp

import (
	"context"
	"log/slog"

	"writekit/internal/auth"
	"writekit/internal/platform"
	"writekit/internal/tenant"
)

type TenantResolver interface {
	Resolve(ctx context.Context, userID, tenantID string) (*tenant.DB, string, error)
	Role(ctx context.Context, userID, tenantID string) (string, error)
}

type HostedResolver struct {
	DB   *platform.DB
	Pool *tenant.Pool
}

func (r *HostedResolver) Resolve(ctx context.Context, userID, tenantID string) (*tenant.DB, string, error) {
	tenants, err := r.DB.ListTenantsByMembership(ctx, userID)
	if err != nil {
		slog.Error("hosted resolver: list memberships", "user_id", userID, "err", err)
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

	db, err := r.Pool.Get(selectedID)
	if err != nil {
		slog.Error("hosted resolver: open tenant db", "tenant", selectedID, "err", err)
		return nil, "", err
	}
	return db, selectedID, nil
}

func (r *HostedResolver) Role(ctx context.Context, userID, tenantID string) (string, error) {
	member, err := r.DB.GetTeamMember(ctx, tenantID, userID)
	if err != nil {
		return "", errTenantNotFound
	}
	return member.Role, nil
}

type LocalResolver struct {
	Pool *tenant.Pool
}

func (r *LocalResolver) Resolve(ctx context.Context, userID, tenantID string) (*tenant.DB, string, error) {
	if tenantID != "" && tenantID != auth.LocalTenantID {
		return nil, "", errTenantNotFound
	}
	db, err := r.Pool.Get(auth.LocalTenantID)
	if err != nil {
		return nil, "", err
	}
	return db, auth.LocalTenantID, nil
}

func (r *LocalResolver) Role(ctx context.Context, userID, tenantID string) (string, error) {
	return "owner", nil
}
