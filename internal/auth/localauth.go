package auth

import (
	"context"
	"net/http"
	"sync"
	"time"

	"writekit/internal/httplog"
	"writekit/internal/platform"
)

const LocalUserID = "local"

var (
	localUser = &platform.User{
		ID:        LocalUserID,
		Email:     "local@writekit",
		Name:      "You",
		CreatedAt: time.Unix(0, 0),
		UpdatedAt: time.Unix(0, 0),
	}

	workspacesMu    sync.RWMutex
	activeTenantID  string
	localWorkspaces []LocalWorkspace
)

type LocalWorkspace struct {
	ID   string
	Name string
}

func SetLocalWorkspaces(active string, all []LocalWorkspace) {
	workspacesMu.Lock()
	defer workspacesMu.Unlock()
	activeTenantID = active
	localWorkspaces = append([]LocalWorkspace(nil), all...)
}

func ActiveTenantID() string {
	workspacesMu.RLock()
	defer workspacesMu.RUnlock()
	return activeTenantID
}

func AllLocalWorkspaces() []LocalWorkspace {
	workspacesMu.RLock()
	defer workspacesMu.RUnlock()
	return append([]LocalWorkspace(nil), localWorkspaces...)
}

func LocalUser() *platform.User { return localUser }

func LocalTenants() []platform.Tenant {
	workspacesMu.RLock()
	defer workspacesMu.RUnlock()
	for _, w := range localWorkspaces {
		if w.ID == activeTenantID {
			return []platform.Tenant{{
				ID:        w.ID,
				UserID:    LocalUserID,
				Name:      w.Name,
				CreatedAt: time.Unix(0, 0),
			}}
		}
	}
	return nil
}

func LocalAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := httplog.WithFields(r.Context(), "user_id", LocalUserID)
			ctx = context.WithValue(ctx, userContextKey, localUser)
			ctx = context.WithValue(ctx, tenantContextKey, LocalTenants())
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
