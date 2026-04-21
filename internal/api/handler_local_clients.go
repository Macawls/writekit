package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"writekit/internal/clients"
	"writekit/internal/httplog"
)

type localInfoResponse struct {
	Port    int    `json:"port"`
	DataDir string `json:"data_dir"`
	MCPURL  string `json:"mcp_url"`
}

func (h *Handler) LocalInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, localInfoResponse{
		Port:    h.Config.Port,
		DataDir: h.Config.DataDir,
		MCPURL:  mcpURL(h.Config.Port),
	})
}

func (h *Handler) LocalClients(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, clients.Snapshot(h.Config.Port))
}

func (h *Handler) LocalClientConnect(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	client := clients.ByID(id)
	if client == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown client"})
		return
	}
	log := httplog.FromContext(r.Context())
	if err := client.Connect(h.Config.Port); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, clients.ErrNPXMissing) {
			status = http.StatusBadRequest
		}
		log.Warn("client connect failed", "client", id, "err", err)
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	log.Info("client connected", "client", id)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "connected",
		"needs_restart": true,
	})
}

func (h *Handler) LocalClientDisconnect(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	client := clients.ByID(id)
	if client == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown client"})
		return
	}
	log := httplog.FromContext(r.Context())
	if err := client.Disconnect(); err != nil {
		log.Warn("client disconnect failed", "client", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	log.Info("client disconnected", "client", id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "disconnected"})
}

func mcpURL(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d/mcp", port)
}
