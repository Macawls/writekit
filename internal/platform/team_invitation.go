package platform

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const invitationDuration = 14 * 24 * time.Hour

var (
	ErrInvitationNotFound    = errors.New("invitation not found")
	ErrInvitationExpired     = errors.New("invitation expired")
	ErrInvitationConsumed    = errors.New("invitation already accepted or revoked")
	ErrInvitationExists      = errors.New("a pending invitation already exists for this email")
	ErrAlreadyTeamMember     = errors.New("this user is already a team member")
	ErrInvitationRateLimited = errors.New("too many invitations sent recently — try again later")
)

type TeamInvitation struct {
	ID              string
	TenantID        string
	Email           string
	Role            string
	Token           string
	InvitedByUserID string
	ExpiresAt       time.Time
	AcceptedAt      *time.Time
	RevokedAt       *time.Time
	CreatedAt       time.Time

	InviterName  string
	InviterEmail string
	TenantName   string
}

func (i *TeamInvitation) IsExpired() bool {
	return time.Now().After(i.ExpiresAt)
}

func normalizeEmail(e string) string {
	return strings.ToLower(strings.TrimSpace(e))
}

func newInvitationToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func (db *DB) CreateInvitation(ctx context.Context, tenantID, email, role, invitedByUserID string) (*TeamInvitation, error) {
	email = normalizeEmail(email)

	if !invitationLimiter.allow(tenantID) {
		return nil, ErrInvitationRateLimited
	}

	var memberCount int
	err := db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM team_members tm
		JOIN users u ON u.id = tm.user_id
		WHERE tm.tenant_id = $1 AND u.email = $2
	`, tenantID, email).Scan(&memberCount)
	if err != nil {
		return nil, fmt.Errorf("check membership: %w", err)
	}
	if memberCount > 0 {
		return nil, ErrAlreadyTeamMember
	}

	var pendingCount int
	err = db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM team_invitations
		WHERE tenant_id = $1 AND email = $2 AND accepted_at IS NULL AND revoked_at IS NULL
	`, tenantID, email).Scan(&pendingCount)
	if err != nil {
		return nil, fmt.Errorf("check pending: %w", err)
	}
	if pendingCount > 0 {
		return nil, ErrInvitationExists
	}

	token, err := newInvitationToken()
	if err != nil {
		return nil, err
	}

	row := db.Pool.QueryRow(ctx, `
		INSERT INTO team_invitations (tenant_id, email, role, token, invited_by_user_id, expires_at)
		VALUES ($1, $2, $3, $4, $5, NOW() + $6::interval)
		RETURNING id, tenant_id, email, role, token, invited_by_user_id, expires_at, accepted_at, revoked_at, created_at
	`, tenantID, email, role, token, invitedByUserID, fmt.Sprintf("%d seconds", int(invitationDuration.Seconds())))

	var inv TeamInvitation
	if err := row.Scan(&inv.ID, &inv.TenantID, &inv.Email, &inv.Role, &inv.Token, &inv.InvitedByUserID, &inv.ExpiresAt, &inv.AcceptedAt, &inv.RevokedAt, &inv.CreatedAt); err != nil {
		return nil, fmt.Errorf("create invitation: %w", err)
	}
	return &inv, nil
}

func (db *DB) GetInvitationByToken(ctx context.Context, token string) (*TeamInvitation, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT i.id, i.tenant_id, i.email, i.role, i.token, i.invited_by_user_id,
		       i.expires_at, i.accepted_at, i.revoked_at, i.created_at,
		       u.name, u.email, t.name
		FROM team_invitations i
		JOIN users u ON u.id = i.invited_by_user_id
		JOIN tenants t ON t.id = i.tenant_id
		WHERE i.token = $1
	`, token)

	var inv TeamInvitation
	err := row.Scan(&inv.ID, &inv.TenantID, &inv.Email, &inv.Role, &inv.Token, &inv.InvitedByUserID,
		&inv.ExpiresAt, &inv.AcceptedAt, &inv.RevokedAt, &inv.CreatedAt,
		&inv.InviterName, &inv.InviterEmail, &inv.TenantName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvitationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get invitation: %w", err)
	}
	return &inv, nil
}

func (db *DB) ListPendingInvitations(ctx context.Context, tenantID string) ([]TeamInvitation, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT i.id, i.tenant_id, i.email, i.role, i.token, i.invited_by_user_id,
		       i.expires_at, i.accepted_at, i.revoked_at, i.created_at,
		       u.name, u.email
		FROM team_invitations i
		JOIN users u ON u.id = i.invited_by_user_id
		WHERE i.tenant_id = $1 AND i.accepted_at IS NULL AND i.revoked_at IS NULL
		ORDER BY i.created_at DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	defer rows.Close()

	var out []TeamInvitation
	for rows.Next() {
		var inv TeamInvitation
		if err := rows.Scan(&inv.ID, &inv.TenantID, &inv.Email, &inv.Role, &inv.Token, &inv.InvitedByUserID,
			&inv.ExpiresAt, &inv.AcceptedAt, &inv.RevokedAt, &inv.CreatedAt,
			&inv.InviterName, &inv.InviterEmail); err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, nil
}

func (db *DB) AcceptInvitation(ctx context.Context, token, userID string) (*TeamInvitation, error) {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		SELECT id, tenant_id, email, role, token, invited_by_user_id, expires_at, accepted_at, revoked_at, created_at
		FROM team_invitations
		WHERE token = $1
		FOR UPDATE
	`, token)

	var inv TeamInvitation
	err = row.Scan(&inv.ID, &inv.TenantID, &inv.Email, &inv.Role, &inv.Token, &inv.InvitedByUserID,
		&inv.ExpiresAt, &inv.AcceptedAt, &inv.RevokedAt, &inv.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvitationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock invitation: %w", err)
	}
	if inv.AcceptedAt != nil || inv.RevokedAt != nil {
		return nil, ErrInvitationConsumed
	}
	if time.Now().After(inv.ExpiresAt) {
		return nil, ErrInvitationExpired
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO team_members (tenant_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id, user_id) DO NOTHING
	`, inv.TenantID, userID, inv.Role); err != nil {
		return nil, fmt.Errorf("add team member: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE team_invitations SET accepted_at = NOW() WHERE id = $1
	`, inv.ID); err != nil {
		return nil, fmt.Errorf("mark accepted: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	now := time.Now()
	inv.AcceptedAt = &now
	return &inv, nil
}

func (db *DB) RevokeInvitation(ctx context.Context, tenantID, invitationID string) error {
	ct, err := db.Pool.Exec(ctx, `
		UPDATE team_invitations
		SET revoked_at = NOW()
		WHERE id = $1 AND tenant_id = $2 AND accepted_at IS NULL AND revoked_at IS NULL
	`, invitationID, tenantID)
	if err != nil {
		return fmt.Errorf("revoke invitation: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrInvitationNotFound
	}
	return nil
}

func (db *DB) ResendInvitation(ctx context.Context, tenantID, invitationID string) (*TeamInvitation, error) {
	token, err := newInvitationToken()
	if err != nil {
		return nil, err
	}

	row := db.Pool.QueryRow(ctx, `
		UPDATE team_invitations
		SET token = $3, expires_at = NOW() + $4::interval
		WHERE id = $1 AND tenant_id = $2 AND accepted_at IS NULL AND revoked_at IS NULL
		RETURNING id, tenant_id, email, role, token, invited_by_user_id, expires_at, accepted_at, revoked_at, created_at
	`, invitationID, tenantID, token, fmt.Sprintf("%d seconds", int(invitationDuration.Seconds())))

	var inv TeamInvitation
	err = row.Scan(&inv.ID, &inv.TenantID, &inv.Email, &inv.Role, &inv.Token, &inv.InvitedByUserID,
		&inv.ExpiresAt, &inv.AcceptedAt, &inv.RevokedAt, &inv.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvitationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("resend invitation: %w", err)
	}
	return &inv, nil
}

func (db *DB) CleanExpiredInvitations(ctx context.Context) error {
	if _, err := db.Pool.Exec(ctx, `
		DELETE FROM team_invitations
		WHERE expires_at < NOW() AND accepted_at IS NULL
	`); err != nil {
		return fmt.Errorf("clean expired invitations: %w", err)
	}
	return nil
}
