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
		Domain:   h.Config.Host,
		Title:    "Publish by conversation",
		Subtitle: "Pages, collections, and docs managed entirely through your AI assistant. Connect via MCP and publish to your own subdomain.",
		Command:  "claude mcp add --transport http writekit https://" + h.Config.Host + "/mcp",
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
