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
	var reqBody map[string]any
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		slog.Warn("oauth register: invalid request body", "err", err)
		httpError(w, "invalid_client_metadata", http.StatusBadRequest)
		return
	}

	redirectURIs, _ := reqBody["redirect_uris"].([]any)
	if len(redirectURIs) == 0 {
		slog.Warn("oauth register: missing redirect_uris")
		httpError(w, "invalid_client_metadata", http.StatusBadRequest)
		return
	}

	uris := make([]string, len(redirectURIs))
	for i, u := range redirectURIs {
		s, ok := u.(string)
		if !ok {
			httpError(w, "invalid_client_metadata", http.StatusBadRequest)
			return
		}
		uris[i] = s
	}

	clientName, _ := reqBody["client_name"].(string)

	clientID, _ := generateID()
	clientSecret, _ := generateID()

	client := &platform.OAuthClient{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURIs: uris,
		ClientName:   clientName,
		IsDynamic:    true,
	}

	if err := m.DB.CreateOAuthClient(r.Context(), client); err != nil {
		slog.Error("oauth register: db insert failed", "err", err)
		httpError(w, "server_error", http.StatusInternalServerError)
		return
	}

	slog.Info("oauth client registered", "client_id", clientID, "client_name", clientName, "redirect_uris", uris)

	resp := map[string]any{
		"client_id":                clientID,
		"client_secret":            clientSecret,
		"client_id_issued_at":      time.Now().Unix(),
		"client_secret_expires_at": 0,
	}
	for k, v := range reqBody {
		if _, exists := resp[k]; !exists {
			resp[k] = v
		}
	}
	if _, ok := resp["redirect_uris"]; !ok {
		resp["redirect_uris"] = uris
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
