package auth

import (
	"context"
	"net/http"
	"strings"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"writekit/internal/httplog"
	"writekit/internal/platform"
)

type contextKey string

const (
	userContextKey   contextKey = "user"
	tenantContextKey contextKey = "tenants"
)

func UserFromContext(ctx context.Context) *platform.User {
	u, _ := ctx.Value(userContextKey).(*platform.User)
	return u
}

func TenantsFromContext(ctx context.Context) []platform.Tenant {
	t, _ := ctx.Value(tenantContextKey).([]platform.Tenant)
	return t
}

func WebAuth(db *platform.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := httplog.FromContext(r.Context())
			cookie, err := r.Cookie("session")
			if err != nil {
				log.Debug("web auth: missing session cookie", "err", err)
				http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
				return
			}

			sess, err := db.GetSession(r.Context(), cookie.Value)
			if err != nil {
				log.Debug("web auth: invalid session", "err", err)
				http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
				return
			}

			user, err := db.GetUser(r.Context(), sess.UserID)
			if err != nil {
				log.Warn("web auth: session ok but user lookup failed", "user_id", sess.UserID, "err", err)
				http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
				return
			}

			tenants, err := db.ListTenantsByMembership(r.Context(), user.ID)
			if err != nil {
				log.Warn("web auth: list memberships failed", "user_id", user.ID, "err", err)
			}

			ctx := httplog.WithFields(r.Context(), "user_id", user.ID)
			ctx = context.WithValue(ctx, userContextKey, user)
			ctx = context.WithValue(ctx, tenantContextKey, tenants)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func OptionalWebAuth(db *platform.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session")
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			sess, err := db.GetSession(r.Context(), cookie.Value)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			user, err := db.GetUser(r.Context(), sess.UserID)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func BearerAuth(db *platform.DB, baseURL string) func(http.Handler) http.Handler {
	resourceMeta := baseURL + "/.well-known/oauth-protected-resource"
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := httplog.FromContext(r.Context())
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				log.Debug("bearer auth: missing or malformed Authorization header")
				w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+resourceMeta+`"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			token := strings.TrimPrefix(auth, "Bearer ")

			at, err := db.GetAccessToken(r.Context(), token)
			if err != nil {
				log.Warn("bearer auth: invalid access token", "err", err)
				w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token", resource_metadata="`+resourceMeta+`"`)
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			user, err := db.GetUser(r.Context(), at.UserID)
			if err != nil {
				log.Error("bearer auth: token references missing user", "user_id", at.UserID, "err", err)
				http.Error(w, "user not found", http.StatusUnauthorized)
				return
			}

			tenants, err := db.ListTenantsByMembership(r.Context(), user.ID)
			if err != nil {
				log.Warn("bearer auth: list memberships failed", "user_id", user.ID, "err", err)
			}

			ctx := httplog.WithFields(r.Context(), "user_id", user.ID)
			ctx = context.WithValue(ctx, userContextKey, user)
			ctx = context.WithValue(ctx, tenantContextKey, tenants)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// NewTokenVerifier returns a TokenVerifier that looks up opaque tokens in the DB
// and populates the request context with the authenticated user.
func NewTokenVerifier(db *platform.DB) mcpauth.TokenVerifier {
	return func(ctx context.Context, token string, req *http.Request) (*mcpauth.TokenInfo, error) {
		at, err := db.GetAccessToken(ctx, token)
		if err != nil {
			return nil, mcpauth.ErrInvalidToken
		}
		user, err := db.GetUser(ctx, at.UserID)
		if err != nil {
			return nil, mcpauth.ErrInvalidToken
		}
		tenants, _ := db.ListTenantsByMembership(ctx, user.ID)
		return &mcpauth.TokenInfo{
			UserID:     user.ID,
			Expiration: at.ExpiresAt,
			Extra: map[string]any{
				"user":    user,
				"tenants": tenants,
			},
		}, nil
	}
}

func MCPBearerAuth(db *platform.DB, mcpBaseURL string) func(http.Handler) http.Handler {
	verifier := NewTokenVerifier(db)
	sdkMiddleware := mcpauth.RequireBearerToken(verifier, &mcpauth.RequireBearerTokenOptions{
		ResourceMetadataURL: mcpBaseURL + "/.well-known/oauth-protected-resource",
	})
	return func(next http.Handler) http.Handler {
		wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			info := mcpauth.TokenInfoFromContext(r.Context())
			if info != nil && info.Extra != nil {
				ctx := r.Context()
				if user, ok := info.Extra["user"].(*platform.User); ok {
					ctx = context.WithValue(ctx, userContextKey, user)
				}
				if tenants, ok := info.Extra["tenants"].([]platform.Tenant); ok {
					ctx = context.WithValue(ctx, tenantContextKey, tenants)
				}
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(w, r)
		})
		return sdkMiddleware(wrapped)
	}
}
