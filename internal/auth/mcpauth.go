package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"writekit/internal/platform"
)

// authServerMeta represents RFC 8414 Authorization Server Metadata.
// Defined locally because the SDK's oauthex.AuthServerMeta is behind a build tag.
type authServerMeta struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint,omitempty"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported,omitempty"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported,omitempty"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported,omitempty"`
}

// clientRegistrationRequest represents RFC 7591 Dynamic Client Registration request.
// Defined locally because the SDK's oauthex.ClientRegistrationMetadata is behind a build tag.
type clientRegistrationRequest struct {
	RedirectURIs []string `json:"redirect_uris"`
	ClientName   string   `json:"client_name,omitempty"`
}

// clientRegistrationResponse represents RFC 7591 Dynamic Client Registration response.
type clientRegistrationResponse struct {
	clientRegistrationRequest
	ClientID              string `json:"client_id"`
	ClientSecret          string `json:"client_secret,omitempty"`
	ClientIDIssuedAt      int64  `json:"client_id_issued_at,omitempty"`
	ClientSecretExpiresAt int64  `json:"client_secret_expires_at"`
}

type MCPAuth struct {
	DB      *platform.DB
	BaseURL string
}

// ProtectedResourceHandler returns an http.Handler that serves OAuth 2.0
// protected resource metadata (RFC 9728) with CORS support via the SDK.
func (m *MCPAuth) ProtectedResourceHandler() http.Handler {
	return mcpauth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:             m.BaseURL + "/mcp",
		AuthorizationServers: []string{m.BaseURL},
		ResourceName:         "WriteKit MCP",
	})
}

func (m *MCPAuth) WellKnown(w http.ResponseWriter, r *http.Request) {
	metadata := &authServerMeta{
		Issuer:                            m.BaseURL,
		AuthorizationEndpoint:             m.BaseURL + "/oauth/authorize",
		TokenEndpoint:                     m.BaseURL + "/oauth/token",
		RegistrationEndpoint:              m.BaseURL + "/oauth/register",
		ResponseTypesSupported:            []string{"code"},
		GrantTypesSupported:               []string{"authorization_code", "refresh_token"},
		CodeChallengeMethodsSupported:     []string{"S256"},
		TokenEndpointAuthMethodsSupported: []string{"none"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metadata)
}

func (m *MCPAuth) Register(w http.ResponseWriter, r *http.Request) {
	var meta clientRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&meta); err != nil {
		slog.Warn("oauth register: invalid request body", "err", err)
		httpError(w, "invalid_client_metadata", http.StatusBadRequest)
		return
	}

	if len(meta.RedirectURIs) == 0 {
		slog.Warn("oauth register: missing redirect_uris")
		httpError(w, "invalid_client_metadata", http.StatusBadRequest)
		return
	}

	clientID, _ := generateID()
	clientSecret, _ := generateID()

	client := &platform.OAuthClient{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURIs: meta.RedirectURIs,
		ClientName:   meta.ClientName,
		IsDynamic:    true,
	}

	if err := m.DB.CreateOAuthClient(r.Context(), client); err != nil {
		slog.Error("oauth register: db insert failed", "err", err)
		httpError(w, "server_error", http.StatusInternalServerError)
		return
	}

	slog.Info("oauth client registered", "client_id", clientID, "client_name", meta.ClientName, "redirect_uris", meta.RedirectURIs)

	resp := clientRegistrationResponse{
		clientRegistrationRequest: meta,
		ClientID:                  clientID,
		ClientSecret:              clientSecret,
		ClientIDIssuedAt:          time.Now().Unix(),
		ClientSecretExpiresAt:     0,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (m *MCPAuth) TokenExchange(w http.ResponseWriter, r *http.Request) {
	grantType := r.FormValue("grant_type")
	slog.Info("oauth token exchange", "grant_type", grantType)

	switch grantType {
	case "authorization_code":
		m.handleAuthCodeExchange(w, r)
	case "refresh_token":
		m.handleRefreshExchange(w, r)
	default:
		httpError(w, "unsupported_grant_type", http.StatusBadRequest)
	}
}

func (m *MCPAuth) handleAuthCodeExchange(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	clientID := r.FormValue("client_id")
	codeVerifier := r.FormValue("code_verifier")

	authCode, err := m.DB.GetOAuthCode(r.Context(), code)
	if err != nil {
		httpError(w, "invalid_grant", http.StatusBadRequest)
		return
	}

	_ = m.DB.DeleteOAuthCode(r.Context(), code)

	if authCode.ClientID != clientID {
		httpError(w, "invalid_grant", http.StatusBadRequest)
		return
	}

	if authCode.CodeChallenge != "" {
		if !verifyPKCE(codeVerifier, authCode.CodeChallenge, authCode.CodeMethod) {
			httpError(w, "invalid_grant", http.StatusBadRequest)
			return
		}
	}

	pair, err := GenerateTokenPair(r.Context(), m.DB, clientID, authCode.UserID, authCode.Scope)
	if err != nil {
		httpError(w, "server_error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"token_type":    pair.TokenType,
		"expires_in":    pair.ExpiresIn,
	})
}

func (m *MCPAuth) handleRefreshExchange(w http.ResponseWriter, r *http.Request) {
	refreshToken := r.FormValue("refresh_token")

	pair, err := RefreshAccessToken(r.Context(), m.DB, refreshToken)
	if err != nil {
		httpError(w, "invalid_grant", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"token_type":    pair.TokenType,
		"expires_in":    pair.ExpiresIn,
	})
}

func verifyPKCE(verifier, challenge, method string) bool {
	if method == "S256" {
		h := sha256.Sum256([]byte(verifier))
		computed := base64.RawURLEncoding.EncodeToString(h[:])
		return computed == challenge
	}

	return verifier == challenge
}

func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func httpError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

type AuthRequest struct {
	ClientID      string
	RedirectURI   string
	State         string
	Scope         string
	CodeChallenge string
	CodeMethod    string
}

func ParseAuthRequest(r *http.Request) *AuthRequest {
	return &AuthRequest{
		ClientID:      r.URL.Query().Get("client_id"),
		RedirectURI:   r.URL.Query().Get("redirect_uri"),
		State:         r.URL.Query().Get("state"),
		Scope:         r.URL.Query().Get("scope"),
		CodeChallenge: r.URL.Query().Get("code_challenge"),
		CodeMethod:    r.URL.Query().Get("code_challenge_method"),
	}
}

func (m *MCPAuth) ValidateAuthRequest(r *http.Request, req *AuthRequest) error {
	slog.Info("oauth authorize", "client_id", req.ClientID, "redirect_uri", req.RedirectURI)

	client, err := m.DB.GetOAuthClient(r.Context(), req.ClientID)
	if err != nil {
		slog.Warn("oauth authorize: unknown client", "client_id", req.ClientID, "err", err)
		return fmt.Errorf("unknown client: %s", req.ClientID)
	}

	for _, uri := range client.RedirectURIs {
		if uri == req.RedirectURI {
			return nil
		}
	}

	slog.Warn("oauth authorize: redirect_uri mismatch", "client_id", req.ClientID, "requested", req.RedirectURI, "registered", client.RedirectURIs)
	return fmt.Errorf("invalid redirect_uri")
}

func (m *MCPAuth) IssueAuthCode(r *http.Request, userID string, req *AuthRequest) (string, error) {
	code, err := generateID()
	if err != nil {
		return "", err
	}

	oauthCode := &platform.OAuthCode{
		Code:          code,
		ClientID:      req.ClientID,
		UserID:        userID,
		RedirectURI:   req.RedirectURI,
		CodeChallenge: req.CodeChallenge,
		CodeMethod:    req.CodeMethod,
		Scope:         req.Scope,
		ExpiresAt:     time.Now().Add(10 * time.Minute),
	}

	if err := m.DB.CreateOAuthCode(r.Context(), oauthCode); err != nil {
		return "", err
	}

	return code, nil
}

func BuildRedirectURL(redirectURI, code, state string) string {
	sep := "?"
	if strings.Contains(redirectURI, "?") {
		sep = "&"
	}
	url := redirectURI + sep + "code=" + code
	if state != "" {
		url += "&state=" + state
	}
	return url
}
