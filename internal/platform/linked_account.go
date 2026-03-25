package platform

import (
	"context"
	"fmt"
	"time"
)

type LinkedAccount struct {
	ID            string
	UserID        string
	Provider      string
	ProviderID    string
	Email         string
	EmailVerified bool
	CreatedAt     time.Time
}


func (db *DB) FindUserByProvider(ctx context.Context, provider, providerID string) (*User, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT u.id, u.email, u.name, u.avatar_url, u.created_at, u.updated_at
		FROM users u
		JOIN linked_accounts la ON la.user_id = u.id
		WHERE la.provider = $1 AND la.provider_id = $2
	`, provider, providerID)

	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("find user by provider: %w", err)
	}
	return &u, nil
}


func (db *DB) FindUserByVerifiedEmail(ctx context.Context, email string) (*User, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT u.id, u.email, u.name, u.avatar_url, u.created_at, u.updated_at
		FROM users u
		JOIN linked_accounts la ON la.user_id = u.id
		WHERE la.email = $1 AND la.email_verified = TRUE
		LIMIT 1
	`, email)

	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("find user by verified email: %w", err)
	}
	return &u, nil
}


func (db *DB) CreateUser(ctx context.Context, email, name, avatarURL string) (*User, error) {
	row := db.Pool.QueryRow(ctx, `
		INSERT INTO users (email, name, avatar_url)
		VALUES ($1, $2, $3)
		RETURNING id, email, name, avatar_url, created_at, updated_at
	`, email, name, avatarURL)

	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &u, nil
}


func (db *DB) LinkAccount(ctx context.Context, userID, provider, providerID, email string, emailVerified bool) (*LinkedAccount, error) {
	row := db.Pool.QueryRow(ctx, `
		INSERT INTO linked_accounts (user_id, provider, provider_id, email, email_verified)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, provider, provider_id, email, email_verified, created_at
	`, userID, provider, providerID, email, emailVerified)

	var la LinkedAccount
	err := row.Scan(&la.ID, &la.UserID, &la.Provider, &la.ProviderID, &la.Email, &la.EmailVerified, &la.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("link account: %w", err)
	}
	return &la, nil
}


func (db *DB) ListLinkedAccounts(ctx context.Context, userID string) ([]LinkedAccount, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, user_id, provider, provider_id, email, email_verified, created_at
		FROM linked_accounts WHERE user_id = $1
		ORDER BY created_at
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list linked accounts: %w", err)
	}
	defer rows.Close()

	var accounts []LinkedAccount
	for rows.Next() {
		var la LinkedAccount
		if err := rows.Scan(&la.ID, &la.UserID, &la.Provider, &la.ProviderID, &la.Email, &la.EmailVerified, &la.CreatedAt); err != nil {
			return nil, err
		}
		accounts = append(accounts, la)
	}
	return accounts, nil
}


func (db *DB) UnlinkAccount(ctx context.Context, userID, accountID string) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var count int
	err = tx.QueryRow(ctx, `SELECT COUNT(*) FROM linked_accounts WHERE user_id = $1`, userID).Scan(&count)
	if err != nil {
		return fmt.Errorf("count accounts: %w", err)
	}
	if count <= 1 {
		return fmt.Errorf("cannot remove last linked account")
	}

	tag, err := tx.Exec(ctx, `DELETE FROM linked_accounts WHERE id = $1 AND user_id = $2`, accountID, userID)
	if err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("account not found")
	}

	return tx.Commit(ctx)
}


func (db *DB) HasLinkedProvider(ctx context.Context, userID, provider string) (bool, error) {
	var exists bool
	err := db.Pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM linked_accounts WHERE user_id = $1 AND provider = $2)
	`, userID, provider).Scan(&exists)
	return exists, err
}
