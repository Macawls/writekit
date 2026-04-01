package platform

import (
	"context"
	"fmt"
	"time"
)

type TeamMember struct {
	TenantID  string
	UserID    string
	Role      string // "owner", "editor", "viewer"
	Email     string // joined from users
	Name      string // joined from users
	AvatarURL string // joined from users
	CreatedAt time.Time
}

func (db *DB) AddTeamMember(ctx context.Context, tenantID, userID, role string) error {
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO team_members (tenant_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id, user_id) DO NOTHING
	`, tenantID, userID, role)
	if err != nil {
		return fmt.Errorf("add team member: %w", err)
	}
	return nil
}

func (db *DB) RemoveTeamMember(ctx context.Context, tenantID, userID string) error {
	// Guard: cannot remove the last owner
	var ownerCount int
	err := db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM team_members WHERE tenant_id = $1 AND role = 'owner'
	`, tenantID).Scan(&ownerCount)
	if err != nil {
		return fmt.Errorf("count owners: %w", err)
	}

	var memberRole string
	err = db.Pool.QueryRow(ctx, `
		SELECT role FROM team_members WHERE tenant_id = $1 AND user_id = $2
	`, tenantID, userID).Scan(&memberRole)
	if err != nil {
		return fmt.Errorf("get member role: %w", err)
	}

	if memberRole == "owner" && ownerCount <= 1 {
		return fmt.Errorf("cannot remove the last owner")
	}

	_, err = db.Pool.Exec(ctx, `
		DELETE FROM team_members WHERE tenant_id = $1 AND user_id = $2
	`, tenantID, userID)
	if err != nil {
		return fmt.Errorf("remove team member: %w", err)
	}
	return nil
}

func (db *DB) UpdateTeamMemberRole(ctx context.Context, tenantID, userID, role string) error {
	// Guard: cannot demote the last owner
	if role != "owner" {
		var currentRole string
		err := db.Pool.QueryRow(ctx, `
			SELECT role FROM team_members WHERE tenant_id = $1 AND user_id = $2
		`, tenantID, userID).Scan(&currentRole)
		if err != nil {
			return fmt.Errorf("get member role: %w", err)
		}

		if currentRole == "owner" {
			var ownerCount int
			err := db.Pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM team_members WHERE tenant_id = $1 AND role = 'owner'
			`, tenantID).Scan(&ownerCount)
			if err != nil {
				return fmt.Errorf("count owners: %w", err)
			}
			if ownerCount <= 1 {
				return fmt.Errorf("cannot demote the last owner")
			}
		}
	}

	_, err := db.Pool.Exec(ctx, `
		UPDATE team_members SET role = $3 WHERE tenant_id = $1 AND user_id = $2
	`, tenantID, userID, role)
	if err != nil {
		return fmt.Errorf("update team member role: %w", err)
	}
	return nil
}

func (db *DB) GetTeamMember(ctx context.Context, tenantID, userID string) (*TeamMember, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT tm.tenant_id, tm.user_id, tm.role, u.email, u.name, u.avatar_url, tm.created_at
		FROM team_members tm
		JOIN users u ON u.id = tm.user_id
		WHERE tm.tenant_id = $1 AND tm.user_id = $2
	`, tenantID, userID)

	var m TeamMember
	err := row.Scan(&m.TenantID, &m.UserID, &m.Role, &m.Email, &m.Name, &m.AvatarURL, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get team member: %w", err)
	}
	return &m, nil
}

func (db *DB) ListTeamMembers(ctx context.Context, tenantID string) ([]TeamMember, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT tm.tenant_id, tm.user_id, tm.role, u.email, u.name, u.avatar_url, tm.created_at
		FROM team_members tm
		JOIN users u ON u.id = tm.user_id
		WHERE tm.tenant_id = $1
		ORDER BY tm.created_at
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list team members: %w", err)
	}
	defer rows.Close()

	var members []TeamMember
	for rows.Next() {
		var m TeamMember
		if err := rows.Scan(&m.TenantID, &m.UserID, &m.Role, &m.Email, &m.Name, &m.AvatarURL, &m.CreatedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, nil
}

func (db *DB) ListTenantsByMembership(ctx context.Context, userID string) ([]Tenant, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT t.id, t.user_id, t.name, t.created_at
		FROM tenants t
		JOIN team_members tm ON tm.tenant_id = t.id
		WHERE tm.user_id = $1
		ORDER BY t.created_at
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list tenants by membership: %w", err)
	}
	defer rows.Close()

	var tenants []Tenant
	for rows.Next() {
		var t Tenant
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.CreatedAt); err != nil {
			return nil, err
		}
		tenants = append(tenants, t)
	}
	return tenants, nil
}

func (db *DB) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT id, email, name, avatar_url, created_at, updated_at
		FROM users WHERE email = $1
	`, email)

	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil
}
