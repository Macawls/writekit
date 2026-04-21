package email

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
)

//go:embed templates/*.html
var templatesFS embed.FS

var layoutTmpl = template.Must(template.ParseFS(templatesFS, "templates/layout.html"))

func render(name string, data any) (string, error) {
	t, err := layoutTmpl.Clone()
	if err != nil {
		return "", fmt.Errorf("clone layout: %w", err)
	}
	if _, err := t.ParseFS(templatesFS, "templates/"+name+".html"); err != nil {
		return "", fmt.Errorf("parse %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout.html", data); err != nil {
		return "", fmt.Errorf("execute %s: %w", name, err)
	}
	return buf.String(), nil
}
