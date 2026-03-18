package web

import "net/http"

func (h *Handler) Docs(w http.ResponseWriter, r *http.Request) {
	h.Engine.Render(w, "docs.html", nil)
}

func (h *Handler) LLMsTxt(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/llms.txt")
}
