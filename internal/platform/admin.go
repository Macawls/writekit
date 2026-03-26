package platform

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

func (db *DB) ListUsers(ctx context.Context, limit, offset int) ([]User, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, email, name, avatar_url, created_at, updated_at
		FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (db *DB) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

func (db *DB) CountTenants(ctx context.Context) (int, error) {
	var count int
	err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM tenants`).Scan(&count)
	return count, err
}

func (db *DB) CountActiveSubscriptions(ctx context.Context) (int, error) {
	var count int
	err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM subscriptions WHERE status = 'active'`).Scan(&count)
	return count, err
}

func (db *DB) SearchUsers(ctx context.Context, query string) ([]User, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, email, name, avatar_url, created_at, updated_at
		FROM users
		WHERE email ILIKE '%' || $1 || '%' OR name ILIKE '%' || $1 || '%'
		ORDER BY created_at DESC LIMIT 50
	`, query)
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (db *DB) ListAllTenants(ctx context.Context) ([]Tenant, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, user_id, name, created_at FROM tenants ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list all tenants: %w", err)
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

type AdminSession struct {
	Token     string
	Email     string
	ExpiresAt time.Time
}

func (db *DB) CreateAdminSession(ctx context.Context, email string) (*AdminSession, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(b)

	sess := &AdminSession{
		Token:     token,
		Email:     email,
		ExpiresAt: time.Now().Add(2 * time.Hour),
	}

	_, err := db.Pool.Exec(ctx, `
		INSERT INTO admin_sessions (token, email, expires_at) VALUES ($1, $2, $3)
	`, sess.Token, sess.Email, sess.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("create admin session: %w", err)
	}
	return sess, nil
}

func (db *DB) GetAdminSession(ctx context.Context, token string) (*AdminSession, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT token, email, expires_at FROM admin_sessions
		WHERE token = $1 AND expires_at > NOW()
	`, token)

	var s AdminSession
	err := row.Scan(&s.Token, &s.Email, &s.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("get admin session: %w", err)
	}
	return &s, nil
}

func (db *DB) DeleteAdminSession(ctx context.Context, token string) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM admin_sessions WHERE token = $1`, token)
	return err
}
