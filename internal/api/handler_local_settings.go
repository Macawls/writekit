package api

import (
	"encoding/json"
	"net/http"

	"writekit/internal/autostart"
	"writekit/internal/desksettings"
	"writekit/internal/httplog"
)

type desktopSettingsResponse struct {
	Autostart      bool `json:"autostart"`
	CloseToTray    bool `json:"close_to_tray"`
	StartMinimized bool `json:"start_minimized"`
}

func (h *Handler) LocalGetSettings(w http.ResponseWriter, r *http.Request) {
	store, err := desksettings.Open()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	settings, err := store.Load()
	if err != nil {
		httplog.FromContext(r.Context()).Warn("load settings", "err", err)
	}
	// Reconcile autostart with OS state so the UI reflects reality.
	settings.Autostart = autostart.IsEnabled()
	writeJSON(w, http.StatusOK, desktopSettingsResponse{
		Autostart:      settings.Autostart,
		CloseToTray:    settings.CloseToTray,
		StartMinimized: settings.StartMinimized,
	})
}

func (h *Handler) LocalPutSettings(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	var body desktopSettingsResponse
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	store, err := desksettings.Open()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	current, _ := store.Load()

	next := desksettings.Settings{
		Autostart:      body.Autostart,
		CloseToTray:    body.CloseToTray,
		StartMinimized: body.StartMinimized,
	}

	if next.Autostart != current.Autostart || next.Autostart != autostart.IsEnabled() {
		if err := autostart.Set(next.Autostart); err != nil {
			log.Warn("autostart toggle failed", "want", next.Autostart, "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update autostart: " + err.Error()})
			return
		}
	}

	if err := store.Save(next); err != nil {
		log.Warn("save settings", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	next.Autostart = autostart.IsEnabled()
	writeJSON(w, http.StatusOK, desktopSettingsResponse{
		Autostart:      next.Autostart,
		CloseToTray:    next.CloseToTray,
		StartMinimized: next.StartMinimized,
	})
}
