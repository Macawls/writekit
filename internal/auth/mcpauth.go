package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"writekit/internal/platform"
)

type MCPAuth struct {
	DB      *platform.DB
	BaseURL string
}

func (m *MCPAuth) ProtectedResource(w http.ResponseWriter, r *http.Request) {
	metadata := map[string]any{
		"resource":              m.BaseURL + "/mcp",
		"authorization_servers": []string{m.BaseURL},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metadata)
}

func (m *MCPAuth) WellKnown(w http.ResponseWriter, r *http.Request) {
	metadata := map[string]any{
		"issuer":                 m.BaseURL,
		"authorization_endpoint": m.BaseURL + "/oauth/authorize",
		"token_endpoint":         m.BaseURL + "/oauth/token",
		"registration_endpoint":  m.BaseURL + "/oauth/register",
		"response_types_supported":             []string{"code"},
		"grant_types_supported":                []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":     []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metadata)
}

func (m *MCPAuth) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RedirectURIs []string `json:"redirect_uris"`
		ClientName   string   `json:"client_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, "invalid request", http.StatusBadRequest)
		return
	}

	if len(req.RedirectURIs) == 0 {
		httpError(w, "redirect_uris required", http.StatusBadRequest)
		return
	}

	clientID, _ := generateID()
	clientSecret, _ := generateID()

	client := &platform.OAuthClient{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURIs: req.RedirectURIs,
		ClientName:   req.ClientName,
		IsDynamic:    true,
	}

	if err := m.DB.CreateOAuthClient(r.Context(), client); err != nil {
		httpError(w, "failed to register client", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"client_id":     clientID,
		"client_secret": clientSecret,
		"redirect_uris": req.RedirectURIs,
		"client_name":   req.ClientName,
	})
}

func (m *MCPAuth) TokenExchange(w http.ResponseWriter, r *http.Request) {
	grantType := r.FormValue("grant_type")

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
	client, err := m.DB.GetOAuthClient(r.Context(), req.ClientID)
	if err != nil {
		return fmt.Errorf("unknown client")
	}

	validRedirect := false
	for _, uri := range client.RedirectURIs {
		if uri == req.RedirectURI {
			validRedirect = true
			break
		}
	}
	if !validRedirect {
		return fmt.Errorf("invalid redirect_uri")
	}

	return nil
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
