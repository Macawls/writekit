package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"writekit/internal/platform"
)

const (
	AccessTokenDuration  = 1 * time.Hour
	RefreshTokenDuration = 30 * 24 * time.Hour
)

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
	TokenType    string
}

func GenerateTokenPair(ctx context.Context, db *platform.DB, clientID, userID, scope string) (*TokenPair, error) {
	accessToken, err := generateOpaqueToken()
	if err != nil {
		return nil, err
	}
	refreshToken, err := generateOpaqueToken()
	if err != nil {
		return nil, err
	}

	accessExpiry := time.Now().Add(AccessTokenDuration)
	refreshExpiry := time.Now().Add(RefreshTokenDuration)

	err = db.CreateAccessToken(ctx, accessToken, clientID, userID, scope, accessExpiry)
	if err != nil {
		return nil, fmt.Errorf("create access token: %w", err)
	}

	err = db.CreateRefreshToken(ctx, refreshToken, accessToken, clientID, userID, refreshExpiry)
	if err != nil {
		return nil, fmt.Errorf("create refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(AccessTokenDuration.Seconds()),
		TokenType:    "Bearer",
	}, nil
}

func RefreshAccessToken(ctx context.Context, db *platform.DB, refreshToken string) (*TokenPair, error) {
	rt, err := db.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token")
	}

	_ = db.DeleteAccessToken(ctx, rt.AccessToken)
	_ = db.DeleteRefreshToken(ctx, refreshToken)

	return GenerateTokenPair(ctx, db, rt.ClientID, rt.UserID, "")
}

func generateOpaqueToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
