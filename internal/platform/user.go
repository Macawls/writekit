package platform

import (
	"context"
	"fmt"
	"time"
)

type User struct {
	ID            string
	Email         string
	Name          string
	AvatarURL     string
	OAuthProvider string
	OAuthID       string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (db *DB) UpsertUser(ctx context.Context, u *User) (*User, bool, error) {
	row := db.Pool.QueryRow(ctx, `
		INSERT INTO users (email, name, avatar_url, oauth_provider, oauth_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (oauth_provider, oauth_id) DO UPDATE SET
			email = EXCLUDED.email,
			name = EXCLUDED.name,
			avatar_url = EXCLUDED.avatar_url,
			updated_at = NOW()
		RETURNING id, email, name, avatar_url, oauth_provider, oauth_id, created_at, updated_at, (xmax = 0) AS is_new
	`, u.Email, u.Name, u.AvatarURL, u.OAuthProvider, u.OAuthID)

	var out User
	var isNew bool
	err := row.Scan(&out.ID, &out.Email, &out.Name, &out.AvatarURL,
		&out.OAuthProvider, &out.OAuthID, &out.CreatedAt, &out.UpdatedAt, &isNew)
	if err != nil {
		return nil, false, fmt.Errorf("upsert user: %w", err)
	}
	return &out, isNew, nil
}

func (db *DB) GetUser(ctx context.Context, id string) (*User, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT id, email, name, avatar_url, oauth_provider, oauth_id, created_at, updated_at
		FROM users WHERE id = $1
	`, id)

	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL,
		&u.OAuthProvider, &u.OAuthID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}

func (db *DB) UpdateUser(ctx context.Context, id, name string) error {
	_, err := db.Pool.Exec(ctx, `
		UPDATE users SET name = $2, updated_at = NOW() WHERE id = $1
	`, id, name)
	return err
}
