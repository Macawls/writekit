package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"writekit/internal/autostart"
	"writekit/internal/desksettings"
	"writekit/internal/httplog"
)

type desktopSettingsResponse struct {
	Autostart          bool   `json:"autostart"`
	CloseToTray        bool   `json:"close_to_tray"`
	StartMinimized     bool   `json:"start_minimized"`
	DataDir            string `json:"data_dir"`
	EffectiveDataDir   string `json:"effective_data_dir"`
	OnboardingComplete bool   `json:"onboarding_complete"`
	NeedsRestart       bool   `json:"needs_restart,omitempty"`
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
	settings.Autostart = autostart.IsEnabled()
	writeJSON(w, http.StatusOK, desktopSettingsResponse{
		Autostart:          settings.Autostart,
		CloseToTray:        settings.CloseToTray,
		StartMinimized:     settings.StartMinimized,
		DataDir:            settings.DataDir,
		EffectiveDataDir:   h.Config.DataDir,
		OnboardingComplete: settings.OnboardingComplete,
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

	newDataDir := filepath.Clean(body.DataDir)
	if body.DataDir == "" {
		newDataDir = ""
	}
	if newDataDir != "" {
		if err := validateDataDir(newDataDir); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
	}
	dataDirChanged := newDataDir != current.DataDir

	next := desksettings.Settings{
		Autostart:          body.Autostart,
		CloseToTray:        body.CloseToTray,
		StartMinimized:     body.StartMinimized,
		DataDir:            newDataDir,
		OnboardingComplete: current.OnboardingComplete || body.OnboardingComplete,
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
		Autostart:          next.Autostart,
		CloseToTray:        next.CloseToTray,
		StartMinimized:     next.StartMinimized,
		DataDir:            next.DataDir,
		EffectiveDataDir:   h.Config.DataDir,
		OnboardingComplete: next.OnboardingComplete,
		NeedsRestart:       dataDirChanged,
	})
}

func (h *Handler) LocalShow(w http.ResponseWriter, r *http.Request) {
	if desksettings.ShowWindow != nil {
		desksettings.ShowWindow()
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) LocalPickFolder(w http.ResponseWriter, r *http.Request) {
	if desksettings.PickFolder == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "folder picker unavailable"})
		return
	}
	path, err := desksettings.PickFolder("Choose WriteKit data folder")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if path == "" {
		writeJSON(w, http.StatusOK, map[string]string{"path": ""})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"path":              path,
		"has_existing_data": dirHasTenantFiles(path),
	})
}

func validateDataDir(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}
	probe := filepath.Join(path, ".writekit-probe")
	f, err := os.Create(probe)
	if err != nil {
		return err
	}
	f.Close()
	return os.Remove(probe)
}

func dirHasTenantFiles(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) == ".db" {
			return true
		}
	}
	return false
}
