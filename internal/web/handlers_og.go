package web

import (
	"net/http"

	"writekit/internal/httplog"
	"writekit/internal/og"
)

func (h *Handler) LandingOG(w http.ResponseWriter, r *http.Request) {
	if h.OG == nil {
		http.Error(w, "og renderer unavailable", http.StatusServiceUnavailable)
		return
	}

	img, err := h.OG.RenderLanding(og.LandingData{
		Domain:  h.Config.Host,
		Title:   "Publish by conversation.",
		Eyebrow: "MCP-NATIVE PUBLISHING",
		Tagline: "open source",
	})
	if err != nil {
		httplog.FromContext(r.Context()).Error("landing og render", "err", err)
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(img)
}
