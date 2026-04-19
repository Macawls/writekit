package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"
	"github.com/stripe/stripe-go/v82"
	billingportal "github.com/stripe/stripe-go/v82/billingportal/session"
	checkout "github.com/stripe/stripe-go/v82/checkout/session"
	"writekit/internal/config"
	"writekit/internal/embedding"
	"writekit/internal/events"
	"writekit/internal/httplog"
	"writekit/internal/platform"
	"writekit/internal/tenant"
)

type Handler struct {
	DB       *platform.DB
	Pool     *tenant.Pool
	Config   *config.Config
	Embedder *embedding.Client
	Bus      *events.Bus
}

type contextKey string

const userContextKey contextKey = "user"

func userFromContext(ctx context.Context) *platform.User {
	u, _ := ctx.Value(userContextKey).(*platform.User)
	return u
}

func (h *Handler) Routes(r chi.Router) {
	r.Use(h.authMiddleware)
	r.Get("/api/me", h.Me)
	r.Post("/api/site", h.CreateSite)
	r.Put("/api/site/slug", h.UpdateSlug)
	r.Get("/api/site", h.GetSite)
	r.Put("/api/me", h.UpdateProfile)
	r.Post("/api/billing/checkout", h.BillingCheckout)
	r.Post("/api/billing/portal", h.BillingPortal)
	r.Get("/api/team", h.ListTeamMembers)
	r.Post("/api/team", h.InviteTeamMember)
	r.Put("/api/team/{userId}", h.UpdateTeamMemberRole)
	r.Delete("/api/team/{userId}", h.RemoveTeamMember)
	r.Get("/api/graph", h.Graph)
	r.Get("/api/db/tables", h.DBTables)
	r.Get("/api/db/tables/{name}", h.DBTableRows)
	r.Post("/api/db/query", h.DBQuery)
	r.Get("/api/db/export", h.DBExport)
}

func (h *Handler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := httplog.FromContext(r.Context())
		cookie, err := r.Cookie("session")
		if err != nil {
			log.Debug("api auth: missing session cookie", "err", err)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		sess, err := h.DB.GetSession(r.Context(), cookie.Value)
		if err != nil {
			log.Debug("api auth: invalid session", "err", err)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		user, err := h.DB.GetUser(r.Context(), sess.UserID)
		if err != nil {
			log.Warn("api auth: session ok but user lookup failed", "user_id", sess.UserID, "err", err)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		ctx := httplog.WithFields(r.Context(), "user_id", user.ID)
		ctx = context.WithValue(ctx, userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	user := userFromContext(r.Context())
	site, err := h.DB.GetTenantByUser(r.Context(), user.ID)
	if err != nil {
		log.Debug("me: no tenant for user", "user_id", user.ID, "err", err)
	}
	sub, err := h.DB.GetSubscription(r.Context(), user.ID)
	if err != nil {
		log.Debug("me: no subscription for user", "user_id", user.ID, "err", err)
	}

	var role string
	if site != nil {
		if member, err := h.DB.GetTeamMember(r.Context(), site.ID, user.ID); err == nil {
			role = member.Role
		} else {
			log.Warn("me: get team member failed", "tenant", site.ID, "user_id", user.ID, "err", err)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id":         user.ID,
			"email":      user.Email,
			"name":       user.Name,
			"avatar_url": user.AvatarURL,
		},
		"site":         site,
		"subscription": sub,
		"role":         role,
	})
}

func (h *Handler) GetSite(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	site, err := h.DB.GetTenantByUser(r.Context(), user.ID)
	if err != nil {
		httplog.FromContext(r.Context()).Debug("get site: no tenant", "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no site found"})
		return
	}
	writeJSON(w, http.StatusOK, site)
}

var slugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)

func (h *Handler) CreateSite(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	user := userFromContext(r.Context())

	var body struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		log.Warn("create site: decode body", "err", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if !slugRegex.MatchString(body.Slug) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid slug: use only lowercase letters, numbers, and hyphens (3-64 chars)"})
		return
	}

	if body.Slug == "app" || body.Slug == "www" || body.Slug == "api" || body.Slug == "admin" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "this subdomain is reserved"})
		return
	}

	if body.Name == "" {
		body.Name = body.Slug
	}

	existing, _ := h.DB.GetTenantByUser(r.Context(), user.ID)
	if existing != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "you already have a site"})
		return
	}

	err := h.DB.CreateTenant(r.Context(), &platform.Tenant{
		ID:     body.Slug,
		UserID: user.ID,
		Name:   body.Name,
	})
	if err != nil {
		log.Error("create tenant failed", "slug", body.Slug, "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusConflict, map[string]string{"error": "slug is already taken"})
		return
	}

	if _, err := h.Pool.Get(body.Slug); err != nil {
		log.Error("init tenant db failed", "slug", body.Slug, "err", err)
	}

	site, err := h.DB.GetTenantByUser(r.Context(), user.ID)
	if err != nil {
		log.Error("create site: reload after create", "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load site"})
		return
	}
	log.Info("site created", "slug", body.Slug, "user_id", user.ID)
	writeJSON(w, http.StatusCreated, site)
}

func (h *Handler) UpdateSlug(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	user := userFromContext(r.Context())

	var body struct {
		Slug string `json:"slug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		log.Warn("update slug: decode body", "err", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if !slugRegex.MatchString(body.Slug) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid slug: use only lowercase letters, numbers, and hyphens (3-64 chars)"})
		return
	}

	if body.Slug == "app" || body.Slug == "www" || body.Slug == "api" || body.Slug == "admin" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "this subdomain is reserved"})
		return
	}

	existing, err := h.DB.GetTenantByUser(r.Context(), user.ID)
	if err != nil {
		log.Warn("update slug: no existing tenant", "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no site found"})
		return
	}

	if existing.ID == body.Slug {
		writeJSON(w, http.StatusOK, existing)
		return
	}

	if err := h.Pool.Rename(existing.ID, body.Slug); err != nil {
		log.Error("rename tenant db failed", "old", existing.ID, "new", body.Slug, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to rename site"})
		return
	}

	if err := h.DB.RenameTenant(r.Context(), existing.ID, body.Slug); err != nil {
		log.Error("rename tenant row failed, rolling back file rename", "old", existing.ID, "new", body.Slug, "err", err)
		if rbErr := h.Pool.Rename(body.Slug, existing.ID); rbErr != nil {
			log.Error("rollback tenant rename failed", "new", body.Slug, "old", existing.ID, "err", rbErr)
		}
		writeJSON(w, http.StatusConflict, map[string]string{"error": "slug is already taken"})
		return
	}

	site, err := h.DB.GetTenantByUser(r.Context(), user.ID)
	if err != nil {
		log.Error("update slug: reload after rename", "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load site"})
		return
	}
	log.Info("tenant renamed", "old", existing.ID, "new", body.Slug, "user_id", user.ID)
	writeJSON(w, http.StatusOK, site)
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	user := userFromContext(r.Context())

	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		log.Warn("update profile: decode body", "err", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if body.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	if err := h.DB.UpdateUser(r.Context(), user.ID, body.Name); err != nil {
		log.Error("update user", "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update profile"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"name": body.Name})
}

func (h *Handler) BillingCheckout(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())

	appURL := "https://app." + h.Config.Host

	params := &stripe.CheckoutSessionParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: stripe.String(h.Config.StripePriceID), Quantity: stripe.Int64(1)},
		},
		SuccessURL:        stripe.String(appURL + "?billing=success"),
		CancelURL:         stripe.String(appURL + "?billing=cancel"),
		CustomerEmail:     stripe.String(user.Email),
		ClientReferenceID: stripe.String(user.ID),
	}

	sess, err := checkout.New(params)
	if err != nil {
		httplog.FromContext(r.Context()).Error("stripe: create checkout session", "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create checkout"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"url": sess.URL})
}

func (h *Handler) BillingPortal(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	user := userFromContext(r.Context())
	sub, err := h.DB.GetSubscription(r.Context(), user.ID)
	if err != nil {
		log.Warn("billing portal: no subscription", "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no subscription found"})
		return
	}

	appURL := "https://app." + h.Config.Host

	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(sub.StripeCustomerID),
		ReturnURL: stripe.String(appURL),
	}

	sess, err := billingportal.New(params)
	if err != nil {
		log.Error("stripe: create portal session", "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create portal"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"url": sess.URL})
}

func (h *Handler) requireOwner(r *http.Request) (*platform.Tenant, error) {
	user := userFromContext(r.Context())
	site, err := h.DB.GetTenantByUser(r.Context(), user.ID)
	if err != nil {
		return nil, fmt.Errorf("no site found")
	}
	member, err := h.DB.GetTeamMember(r.Context(), site.ID, user.ID)
	if err != nil || member.Role != "owner" {
		return nil, fmt.Errorf("owner access required")
	}
	return site, nil
}

func (h *Handler) ListTeamMembers(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	user := userFromContext(r.Context())
	site, err := h.DB.GetTenantByUser(r.Context(), user.ID)
	if err != nil {
		log.Warn("list team members: no tenant", "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no site found"})
		return
	}

	members, err := h.DB.ListTeamMembers(r.Context(), site.ID)
	if err != nil {
		log.Error("list team members", "tenant", site.ID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list members"})
		return
	}

	type memberResp struct {
		UserID    string `json:"user_id"`
		Email     string `json:"email"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
		Role      string `json:"role"`
		CreatedAt string `json:"created_at"`
	}
	result := make([]memberResp, len(members))
	for i, m := range members {
		result[i] = memberResp{
			UserID:    m.UserID,
			Email:     m.Email,
			Name:      m.Name,
			AvatarURL: m.AvatarURL,
			Role:      m.Role,
			CreatedAt: m.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) InviteTeamMember(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	site, err := h.requireOwner(r)
	if err != nil {
		log.Warn("invite: owner check failed", "err", err)
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}

	var body struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		log.Warn("invite: decode body", "err", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.Role != "editor" && body.Role != "viewer" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "role must be editor or viewer"})
		return
	}

	invitee, err := h.DB.GetUserByEmail(r.Context(), body.Email)
	if err != nil {
		log.Info("invite: invitee not found", "email", body.Email, "err", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no WriteKit account found for this email"})
		return
	}

	if err := h.DB.AddTeamMember(r.Context(), site.ID, invitee.ID, body.Role); err != nil {
		log.Error("invite: add team member", "tenant", site.ID, "invitee", invitee.ID, "role", body.Role, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to add member"})
		return
	}

	log.Info("team member added", "tenant", site.ID, "user_id", invitee.ID, "role", body.Role)
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *Handler) UpdateTeamMemberRole(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	site, err := h.requireOwner(r)
	if err != nil {
		log.Warn("update role: owner check failed", "err", err)
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}

	userID := chi.URLParam(r, "userId")
	var body struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		log.Warn("update role: decode body", "err", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.Role != "owner" && body.Role != "editor" && body.Role != "viewer" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid role"})
		return
	}

	if err := h.DB.UpdateTeamMemberRole(r.Context(), site.ID, userID, body.Role); err != nil {
		log.Warn("update team member role", "tenant", site.ID, "target", userID, "role", body.Role, "err", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	log.Info("team member role updated", "tenant", site.ID, "target", userID, "role", body.Role)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) RemoveTeamMember(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	site, err := h.requireOwner(r)
	if err != nil {
		log.Warn("remove member: owner check failed", "err", err)
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}

	userID := chi.URLParam(r, "userId")
	if err := h.DB.RemoveTeamMember(r.Context(), site.ID, userID); err != nil {
		log.Warn("remove team member", "tenant", site.ID, "target", userID, "err", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	log.Info("team member removed", "tenant", site.ID, "target", userID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write json response", "err", err)
	}
}
