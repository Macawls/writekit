package auth

import (
	"context"
	"net/http"
	"strings"

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
			cookie, err := r.Cookie("session")
			if err != nil {
				http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
				return
			}

			sess, err := db.GetSession(r.Context(), cookie.Value)
			if err != nil {
				http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
				return
			}

			user, err := db.GetUser(r.Context(), sess.UserID)
			if err != nil {
				http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
				return
			}

			tenants, _ := db.ListTenantsByUser(r.Context(), user.ID)

			ctx := context.WithValue(r.Context(), userContextKey, user)
			ctx = context.WithValue(ctx, tenantContextKey, tenants)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func BearerAuth(db *platform.DB, baseURL string) func(http.Handler) http.Handler {
	resourceMeta := baseURL + "/.well-known/oauth-protected-resource"
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+resourceMeta+`"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			token := strings.TrimPrefix(auth, "Bearer ")

			at, err := db.GetAccessToken(r.Context(), token)
			if err != nil {
				w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token", resource_metadata="`+resourceMeta+`"`)
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			user, err := db.GetUser(r.Context(), at.UserID)
			if err != nil {
				http.Error(w, "user not found", http.StatusUnauthorized)
				return
			}

			tenants, _ := db.ListTenantsByUser(r.Context(), user.ID)

			ctx := context.WithValue(r.Context(), userContextKey, user)
			ctx = context.WithValue(ctx, tenantContextKey, tenants)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
