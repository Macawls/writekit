package auth

import (
	"context"
	"net/http"
	"time"

	"writekit/internal/httplog"
	"writekit/internal/platform"
)

const (
	LocalUserID   = "local"
	LocalTenantID = "local"
)

var (
	localUser = &platform.User{
		ID:        LocalUserID,
		Email:     "local@writekit",
		Name:      "You",
		CreatedAt: time.Unix(0, 0),
		UpdatedAt: time.Unix(0, 0),
	}
	localTenants = []platform.Tenant{{
		ID:        LocalTenantID,
		UserID:    LocalUserID,
		Name:      "My Site",
		CreatedAt: time.Unix(0, 0),
	}}
)

func LocalUser() *platform.User          { return localUser }
func LocalTenants() []platform.Tenant    { return localTenants }

func LocalAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := httplog.WithFields(r.Context(), "user_id", LocalUserID)
			ctx = context.WithValue(ctx, userContextKey, localUser)
			ctx = context.WithValue(ctx, tenantContextKey, localTenants)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
