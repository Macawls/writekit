package platform

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type OAuthClient struct {
	ClientID     string
	ClientSecret string
	RedirectURIs []string
	ClientName   string
	IsDynamic    bool
	CreatedAt    time.Time
}

type OAuthCode struct {
	Code          string
	ClientID      string
	UserID        string
	RedirectURI   string
	CodeChallenge string
	CodeMethod    string
	Scope         string
	ExpiresAt     time.Time
}

type AccessToken struct {
	Token     string
	ClientID  string
	UserID    string
	Scope     string
	ExpiresAt time.Time
}

type RefreshToken struct {
	Token       string
	AccessToken string
	ClientID    string
	UserID      string
	ExpiresAt   time.Time
}

func (db *DB) CreateOAuthClient(ctx context.Context, c *OAuthClient) error {
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO oauth_clients (client_id, client_secret, redirect_uris, client_name, is_dynamic)
		VALUES ($1, $2, $3, $4, $5)
	`, c.ClientID, c.ClientSecret, c.RedirectURIs, c.ClientName, c.IsDynamic)
	return err
}

func (db *DB) GetOAuthClient(ctx context.Context, clientID string) (*OAuthClient, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT client_id, client_secret, redirect_uris, client_name, is_dynamic, created_at
		FROM oauth_clients WHERE client_id = $1
	`, clientID)

	var c OAuthClient
	err := row.Scan(&c.ClientID, &c.ClientSecret, &c.RedirectURIs, &c.ClientName, &c.IsDynamic, &c.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("client not found")
		}
		return nil, err
	}
	return &c, nil
}

func (db *DB) CreateOAuthCode(ctx context.Context, c *OAuthCode) error {
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO oauth_codes (code, client_id, user_id, redirect_uri, code_challenge, code_method, scope, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, c.Code, c.ClientID, c.UserID, c.RedirectURI, c.CodeChallenge, c.CodeMethod, c.Scope, c.ExpiresAt)
	return err
}

func (db *DB) GetOAuthCode(ctx context.Context, code string) (*OAuthCode, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT code, client_id, user_id, redirect_uri, code_challenge, code_method, scope, expires_at
		FROM oauth_codes WHERE code = $1 AND expires_at > NOW()
	`, code)

	var c OAuthCode
	err := row.Scan(&c.Code, &c.ClientID, &c.UserID, &c.RedirectURI, &c.CodeChallenge, &c.CodeMethod, &c.Scope, &c.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("code not found or expired")
	}
	return &c, nil
}

func (db *DB) DeleteOAuthCode(ctx context.Context, code string) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM oauth_codes WHERE code = $1`, code)
	return err
}

func (db *DB) CreateAccessToken(ctx context.Context, token, clientID, userID, scope string, expiresAt time.Time) error {
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO oauth_access_tokens (token, client_id, user_id, scope, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`, token, clientID, userID, scope, expiresAt)
	return err
}

func (db *DB) GetAccessToken(ctx context.Context, token string) (*AccessToken, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT token, client_id, user_id, scope, expires_at
		FROM oauth_access_tokens WHERE token = $1 AND expires_at > NOW()
	`, token)

	var at AccessToken
	err := row.Scan(&at.Token, &at.ClientID, &at.UserID, &at.Scope, &at.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("token not found or expired")
	}
	return &at, nil
}

func (db *DB) DeleteAccessToken(ctx context.Context, token string) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM oauth_access_tokens WHERE token = $1`, token)
	return err
}

func (db *DB) CreateRefreshToken(ctx context.Context, token, accessToken, clientID, userID string, expiresAt time.Time) error {
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO oauth_refresh_tokens (token, access_token, client_id, user_id, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`, token, accessToken, clientID, userID, expiresAt)
	return err
}

func (db *DB) GetRefreshToken(ctx context.Context, token string) (*RefreshToken, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT token, access_token, client_id, user_id, expires_at
		FROM oauth_refresh_tokens WHERE token = $1 AND expires_at > NOW()
	`, token)

	var rt RefreshToken
	err := row.Scan(&rt.Token, &rt.AccessToken, &rt.ClientID, &rt.UserID, &rt.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("refresh token not found or expired")
	}
	return &rt, nil
}

func (db *DB) DeleteRefreshToken(ctx context.Context, token string) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM oauth_refresh_tokens WHERE token = $1`, token)
	return err
}
