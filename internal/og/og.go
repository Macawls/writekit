package og

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"strings"
	"sync"

	"github.com/macawls/ogre"
)

//go:embed template.html
var templateSrc string

//go:embed landing.html
var landingTemplateSrc string

const (
	Width  = 1200
	Height = 630

	titleMaxChars     = 60
	titleFallbackSize = 48
	titleLargeSize    = 64
	titleMaxTags      = 5
	subtitleMaxChars  = 140
	slugPathMaxChars  = 36
)

var fontSources = []ogre.FontSource{
	{Name: "Plus Jakarta Sans", Weight: 400, Style: "normal",
		URL: "https://raw.githubusercontent.com/tokotype/PlusJakartaSans/master/fonts/ttf/PlusJakartaSans-Regular.ttf"},
	{Name: "Plus Jakarta Sans", Weight: 500, Style: "normal",
		URL: "https://raw.githubusercontent.com/tokotype/PlusJakartaSans/master/fonts/ttf/PlusJakartaSans-Medium.ttf"},
	{Name: "Plus Jakarta Sans", Weight: 600, Style: "normal",
		URL: "https://raw.githubusercontent.com/tokotype/PlusJakartaSans/master/fonts/ttf/PlusJakartaSans-SemiBold.ttf"},
	{Name: "Plus Jakarta Sans", Weight: 700, Style: "normal",
		URL: "https://raw.githubusercontent.com/tokotype/PlusJakartaSans/master/fonts/ttf/PlusJakartaSans-Bold.ttf"},
	{Name: "JetBrains Mono", Weight: 400, Style: "normal",
		URL: "https://raw.githubusercontent.com/JetBrains/JetBrainsMono/master/fonts/ttf/JetBrainsMono-Regular.ttf"},
}

type Data struct {
	Subdomain string
	DateText  string
	Title     string
	Subtitle  string
	Tags      []string
	SlugPath  string
}

type templateData struct {
	Data
	TitleSize int
}

type LandingData struct {
	Domain  string
	Title   string
	Eyebrow string
	Tagline string
}

type Renderer struct {
	mu          sync.Mutex
	ogre        *ogre.Renderer
	tmpl        *template.Template
	landingTmpl *template.Template
}

func New() (*Renderer, error) {
	tmpl, err := template.New("og").Parse(templateSrc)
	if err != nil {
		return nil, fmt.Errorf("parse og template: %w", err)
	}
	landingTmpl, err := template.New("og-landing").Parse(landingTemplateSrc)
	if err != nil {
		return nil, fmt.Errorf("parse og landing template: %w", err)
	}

	r := ogre.NewRenderer()
	for _, f := range fontSources {
		if err := r.LoadFont(f); err != nil {
			return nil, fmt.Errorf("load font %s %d: %w", f.Name, f.Weight, err)
		}
	}
	return &Renderer{ogre: r, tmpl: tmpl, landingTmpl: landingTmpl}, nil
}

func (r *Renderer) RenderLanding(data LandingData) ([]byte, error) {
	var buf bytes.Buffer
	if err := r.landingTmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute og landing template: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	result, err := r.ogre.Render(buf.String(), ogre.Options{
		Width:  Width,
		Height: Height,
		Format: ogre.FormatPNG,
	})
	if err != nil {
		return nil, fmt.Errorf("ogre landing render: %w", err)
	}
	return result.Data, nil
}

func (r *Renderer) Render(data Data) ([]byte, error) {
	td := templateData{Data: normalize(data), TitleSize: titleSizeFor(data.Title)}

	var buf bytes.Buffer
	if err := r.tmpl.Execute(&buf, td); err != nil {
		return nil, fmt.Errorf("execute og template: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	result, err := r.ogre.Render(buf.String(), ogre.Options{
		Width:  Width,
		Height: Height,
		Format: ogre.FormatPNG,
	})
	if err != nil {
		return nil, fmt.Errorf("ogre render: %w", err)
	}
	return result.Data, nil
}

func normalize(d Data) Data {
	d.Title = strings.TrimSpace(d.Title)
	if d.Title == "" {
		d.Title = d.Subdomain
	}
	d.Subtitle = truncate(strings.TrimSpace(d.Subtitle), subtitleMaxChars)
	d.SlugPath = truncate(d.SlugPath, slugPathMaxChars)
	if len(d.Tags) > titleMaxTags {
		d.Tags = d.Tags[:titleMaxTags]
	}
	return d
}

func titleSizeFor(title string) int {
	if len([]rune(title)) > titleMaxChars {
		return titleFallbackSize
	}
	return titleLargeSize
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return strings.TrimRight(string(runes[:max-1]), " ") + "…"
}
