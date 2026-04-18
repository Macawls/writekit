package web

import (
	"fmt"
	"net/http"
)

func (h *Handler) Docs(w http.ResponseWriter, r *http.Request) {
	h.Engine.Render(w, "docs.html", nil)
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	h.Engine.Render(w, "download.html", nil)
}

func (h *Handler) LLMsTxt(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/llms.txt")
}

func (h *Handler) LLMsFullTxt(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/llms-full.txt")
}

func (h *Handler) RobotsTxt(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/robots.txt")
}

func (h *Handler) Sitemap(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://writekit.dev/</loc><priority>1.0</priority></url>
  <url><loc>https://writekit.dev/download</loc><priority>0.9</priority></url>
  <url><loc>https://writekit.dev/docs</loc><priority>0.8</priority></url>
  <url><loc>https://writekit.dev/llms.txt</loc><priority>0.5</priority></url>
  <url><loc>https://writekit.dev/llms-full.txt</loc><priority>0.5</priority></url>
</urlset>`)
}
