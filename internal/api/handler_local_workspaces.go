package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/oklog/ulid/v2"
	"writekit/internal/auth"
	"writekit/internal/desksettings"
	"writekit/internal/httplog"
)

type workspaceResp struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

func (h *Handler) LocalListWorkspaces(w http.ResponseWriter, r *http.Request) {
	active := auth.ActiveTenantID()
	all := auth.AllLocalWorkspaces()
	out := make([]workspaceResp, len(all))
	for i, ws := range all {
		out[i] = workspaceResp{ID: ws.ID, Name: ws.Name, Active: ws.ID == active}
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) LocalCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = "New Site"
	}

	store, err := desksettings.Open()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	current, err := store.Load()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	id := ulid.Make().String()
	current.Workspaces = append(current.Workspaces, desksettings.Workspace{ID: id, Name: name})
	current.ActiveWorkspaceID = id

	if err := store.Save(current); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if _, err := h.Pool.Get(id); err != nil {
		log.Error("create workspace: open db", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create workspace db"})
		return
	}

	applyWorkspaces(current)
	log.Info("workspace created", "id", id, "name", name)
	writeJSON(w, http.StatusCreated, workspaceResp{ID: id, Name: name, Active: true})
}

func (h *Handler) LocalSwitchWorkspace(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	id := chi.URLParam(r, "id")

	store, err := desksettings.Open()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	current, err := store.Load()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	found := false
	for _, ws := range current.Workspaces {
		if ws.ID == id {
			found = true
			break
		}
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "workspace not found"})
		return
	}

	current.ActiveWorkspaceID = id
	if err := store.Save(current); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	applyWorkspaces(current)
	log.Info("workspace switched", "id", id)
	writeJSON(w, http.StatusOK, map[string]string{"active_id": id})
}

func applyWorkspaces(s desksettings.Settings) {
	all := make([]auth.LocalWorkspace, len(s.Workspaces))
	for i, w := range s.Workspaces {
		all[i] = auth.LocalWorkspace{ID: w.ID, Name: w.Name}
	}
	auth.SetLocalWorkspaces(s.ActiveWorkspaceID, all)
}
