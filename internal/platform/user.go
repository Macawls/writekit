package platform

import (
	"context"
	"fmt"
	"time"
)

type User struct {
	ID        string
	Email     string
	Name      string
	AvatarURL string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (db *DB) GetUser(ctx context.Context, id string) (*User, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT id, email, name, avatar_url, created_at, updated_at
		FROM users WHERE id = $1
	`, id)

	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}

func (db *DB) UpdateUser(ctx context.Context, id, name string) error {
	if _, err := db.Pool.Exec(ctx, `
		UPDATE users SET name = $2, updated_at = NOW() WHERE id = $1
	`, id, name); err != nil {
		return fmt.Errorf("update user %s: %w", id, err)
	}
	return nil
}

func (db *DB) DeleteUser(ctx context.Context, id string) ([]string, error) {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `SELECT id FROM tenants WHERE user_id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	var tenantIDs []string
	for rows.Next() {
		var tid string
		if err := rows.Scan(&tid); err != nil {
			rows.Close()
			return nil, err
		}
		tenantIDs = append(tenantIDs, tid)
	}
	rows.Close()

	for _, table := range []string{
		"oauth_refresh_tokens", "oauth_access_tokens", "oauth_codes",
		"sessions", "subscriptions", "linked_accounts", "team_members", "tenants",
	} {
		if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE user_id = $1", table), id); err != nil {
			return nil, fmt.Errorf("delete from %s: %w", table, err)
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM users WHERE id = $1`, id); err != nil {
		return nil, fmt.Errorf("delete user: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return tenantIDs, nil
}
