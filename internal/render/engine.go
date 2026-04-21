package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"unicode"

	"writekit/internal/markdown"
)

type Engine struct {
	mu        sync.RWMutex
	templates map[string]*template.Template
	funcMap   template.FuncMap
	dev       bool
	fsys      fs.FS
}

func New(fsys fs.FS, dev bool) *Engine {
	return &Engine{
		templates: make(map[string]*template.Template),
		funcMap: template.FuncMap{
			"safe": func(s string) template.HTML { return template.HTML(s) },
			"chromaCSS": func(theme string) template.CSS {
				if theme == "" {
					theme = markdown.DefaultCodeTheme
				}
				css, _ := markdown.GenerateChromaCSS(theme)
				return template.CSS(css)
			},
			"pageURL": func(collectionSlug, pageSlug string) string {
				if collectionSlug != "" {
					return "/" + collectionSlug + "/" + pageSlug
				}
				return "/" + pageSlug
			},
			"parseTags": func(raw string) []map[string]string {
				if raw == "" {
					return nil
				}
				var tags []string
				if err := json.Unmarshal([]byte(raw), &tags); err != nil {
					return nil
				}
				out := make([]map[string]string, 0, len(tags))
				for _, name := range tags {
					name = strings.TrimSpace(name)
					if name == "" {
						continue
					}
					slug := tagSlug(name)
					if slug == "" {
						continue
					}
					out = append(out, map[string]string{"Name": name, "Slug": slug})
				}
				return out
			},
		},
		dev:  dev,
		fsys: fsys,
	}
}

func tagSlug(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	prevDash := true
	for _, r := range strings.ToLower(name) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func (e *Engine) Render(w io.Writer, name string, data any) error {
	tmpl, err := e.get(name)
	if err != nil {
		slog.Error("template load failed", "template", name, "err", err)
		if rw, ok := w.(http.ResponseWriter); ok {
			http.Error(rw, "internal error", http.StatusInternalServerError)
		}
		return err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		slog.Error("template render failed", "template", name, "err", err)
		if rw, ok := w.(http.ResponseWriter); ok {
			http.Error(rw, "internal error", http.StatusInternalServerError)
		}
		return err
	}

	_, err = buf.WriteTo(w)
	return err
}

func (e *Engine) get(name string) (*template.Template, error) {
	if !e.dev {
		e.mu.RLock()
		if t, ok := e.templates[name]; ok {
			e.mu.RUnlock()
			return t, nil
		}
		e.mu.RUnlock()
	}

	t, err := e.parse(name)
	if err != nil {
		return nil, err
	}

	if !e.dev {
		e.mu.Lock()
		e.templates[name] = t
		e.mu.Unlock()
	}

	return t, nil
}

func (e *Engine) parse(name string) (*template.Template, error) {
	t := template.New(name).Funcs(e.funcMap)

	partials, err := fs.Glob(e.fsys, "partials/*.html")
	if err != nil {
		return nil, fmt.Errorf("glob partials: %w", err)
	}
	for _, p := range partials {
		content, err := fs.ReadFile(e.fsys, p)
		if err != nil {
			return nil, fmt.Errorf("read partial %s: %w", p, err)
		}
		if _, err := t.New(p).Parse(string(content)); err != nil {
			return nil, fmt.Errorf("parse partial %s: %w", p, err)
		}
	}

	content, err := fs.ReadFile(e.fsys, name)
	if err != nil {
		return nil, fmt.Errorf("read template %s: %w", name, err)
	}

	if _, err := t.Parse(string(content)); err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}

	return t, nil
}
