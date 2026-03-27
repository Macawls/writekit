package markdown

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"

	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/d2layouts/d2dagrelayout"
	"oss.terrastruct.com/d2/d2lib"
	"oss.terrastruct.com/d2/d2renderers/d2svg"
	"oss.terrastruct.com/d2/d2themes/d2themescatalog"
	"oss.terrastruct.com/d2/lib/textmeasure"
)

var languageIcons = map[string]string{
	"javascript": "javascript",
	"js":         "javascript",
	"typescript": "typescript",
	"ts":         "typescript",
	"html":       "html-5",
	"css":        "css-3",
	"scss":       "sass",
	"sass":       "sass",
	"react":      "react",
	"jsx":        "react",
	"tsx":        "react",
	"vue":        "vue",
	"svelte":     "svelte-icon",
	"angular":    "angular-icon",
	"python":     "python",
	"py":         "python",
	"go":         "go",
	"golang":     "go",
	"rust":       "rust",
	"java":       "java",
	"kotlin":     "kotlin-icon",
	"scala":      "scala",
	"ruby":       "ruby",
	"rb":         "ruby",
	"php":        "php",
	"csharp":     "c-sharp",
	"cs":         "c-sharp",
	"cpp":        "c-plusplus",
	"c":          "c",
	"swift":      "swift",
	"json":       "json",
	"yaml":       "yaml",
	"yml":        "yaml",
	"bash":       "bash-icon",
	"sh":         "bash-icon",
	"shell":      "bash-icon",
	"zsh":        "bash-icon",
	"sql":        "postgresql",
	"mysql":      "mysql-icon",
	"postgres":   "postgresql",
	"postgresql": "postgresql",
	"mongodb":    "mongodb-icon",
	"redis":      "redis",
	"graphql":    "graphql",
	"gql":        "graphql",
	"docker":     "docker-icon",
	"dockerfile": "docker-icon",
	"markdown":   "markdown",
	"md":         "markdown",
	"lua":        "lua",
	"elixir":     "elixir",
	"haskell":    "haskell-icon",
	"dart":       "dart",
	"flutter":    "flutter",
	"terraform":  "terraform-icon",
	"nginx":      "nginx",
}

var titleRegex = regexp.MustCompile(`title=["']([^"']+)["']`)

func parseCodeInfo(info string) (language, title string) {
	info = strings.TrimSpace(info)
	if info == "" {
		return "", ""
	}

	if match := titleRegex.FindStringSubmatch(info); match != nil {
		title = match[1]
		info = strings.TrimSpace(titleRegex.ReplaceAllString(info, ""))
	}

	parts := strings.Fields(info)
	if len(parts) > 0 {
		language = parts[0]
	}

	return language, title
}

func getLanguageIconURL(language string) string {
	slug, ok := languageIcons[strings.ToLower(language)]
	if !ok {
		return ""
	}
	return fmt.Sprintf("https://api.iconify.design/logos/%s.svg", slug)
}

type codeBlockRenderer struct {
	style  string
	ruler  *textmeasure.Ruler
	errors *[]string
}

func (r *codeBlockRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, r.renderFencedCodeBlock)
}

func (r *codeBlockRenderer) renderFencedCodeBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	n := node.(*ast.FencedCodeBlock)
	var info string
	if n.Info != nil {
		info = string(n.Info.Segment.Value(source))
	}
	language, title := parseCodeInfo(info)

	var codeContent strings.Builder
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		codeContent.Write(line.Value(source))
	}
	code := codeContent.String()

	// Handle D2 diagrams
	if strings.ToLower(language) == "d2" {
		return r.renderD2Diagram(w, code)
	}

	// Regular code block rendering
	hasHeader := title != "" || language != ""

	w.WriteString(`<div class="code-block">`)

	if hasHeader {
		w.WriteString(`<div class="code-header">`)

		if language != "" {
			iconURL := getLanguageIconURL(language)
			if iconURL != "" {
				w.WriteString(fmt.Sprintf(`<img src="%s" alt="%s" class="code-icon" />`, iconURL, html.EscapeString(language)))
			}
		}

		if title != "" {
			w.WriteString(fmt.Sprintf(`<span class="code-title">%s</span>`, html.EscapeString(title)))
		} else if language != "" {
			w.WriteString(fmt.Sprintf(`<span class="code-title">%s</span>`, html.EscapeString(language)))
		}

		w.WriteString(`<button class="code-copy" aria-label="Copy code" onclick="navigator.clipboard.writeText(this.closest('.code-block').querySelector('code').textContent)"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg></button>`)
		w.WriteString(`</div>`)
	}

	highlighted := r.highlightCode(code, language)
	w.WriteString(highlighted)

	w.WriteString(`</div>`)

	return ast.WalkContinue, nil
}

func (r *codeBlockRenderer) highlightCode(code, language string) string {
	lexer := lexers.Get(language)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get(r.style)
	if style == nil {
		style = styles.Fallback
	}

	formatter := chromahtml.New(
		chromahtml.WithClasses(true),
		chromahtml.PreventSurroundingPre(false),
	)

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return fmt.Sprintf("<pre><code>%s</code></pre>", html.EscapeString(code))
	}

	var buf strings.Builder
	if err := formatter.Format(&buf, style, iterator); err != nil {
		return fmt.Sprintf("<pre><code>%s</code></pre>", html.EscapeString(code))
	}

	return buf.String()
}

func (r *codeBlockRenderer) renderD2Diagram(w util.BufWriter, code string) (ast.WalkStatus, error) {
	svg, err := r.renderD2(code)
	if err != nil {
		if r.errors != nil {
			*r.errors = append(*r.errors, "D2 diagram error: "+err.Error())
		}
		w.WriteString(`<div class="d2-diagram d2-error">`)
		w.WriteString(`<p>Diagram error: `)
		w.WriteString(html.EscapeString(err.Error()))
		w.WriteString(`</p>`)
		w.WriteString(`<pre><code>`)
		w.WriteString(html.EscapeString(code))
		w.WriteString(`</code></pre>`)
		w.WriteString(`</div>`)
		return ast.WalkSkipChildren, nil
	}

	w.WriteString(`<div class="d2-diagram">`)
	w.WriteString(`<div class="d2-controls">`)
	w.WriteString(`<button type="button" class="d2-fullscreen" aria-label="Fullscreen">`)
	w.WriteString(`<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M8 3H5a2 2 0 0 0-2 2v3m18 0V5a2 2 0 0 0-2-2h-3m0 18h3a2 2 0 0 0 2-2v-3M3 16v3a2 2 0 0 0 2 2h3"/></svg>`)
	w.WriteString(`</button>`)
	w.WriteString(`<button type="button" class="d2-zoom-in" aria-label="Zoom in">`)
	w.WriteString(`<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35M11 8v6M8 11h6"/></svg>`)
	w.WriteString(`</button>`)
	w.WriteString(`<button type="button" class="d2-zoom-out" aria-label="Zoom out">`)
	w.WriteString(`<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35M8 11h6"/></svg>`)
	w.WriteString(`</button>`)
	w.WriteString(`<button type="button" class="d2-reset" aria-label="Reset view">`)
	w.WriteString(`<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/></svg>`)
	w.WriteString(`</button>`)
	w.WriteString(`</div>`)
	w.WriteString(`<div class="d2-viewport">`)
	w.WriteString(svg)
	w.WriteString(`</div>`)
	w.WriteString(`</div>`)

	return ast.WalkSkipChildren, nil
}

func (r *codeBlockRenderer) renderD2(code string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	layoutName := "dagre"
	compileOpts := &d2lib.CompileOptions{
		Layout: &layoutName,
		Ruler:  r.ruler,
		LayoutResolver: func(engine string) (d2graph.LayoutGraph, error) {
			return func(ctx context.Context, g *d2graph.Graph) error {
				return d2dagrelayout.Layout(ctx, g, nil)
			}, nil
		},
	}

	diagram, _, err := d2lib.Compile(ctx, code, compileOpts, nil)
	if err != nil {
		return "", fmt.Errorf("compile error: %w", err)
	}

	themeID := d2themescatalog.NeutralGrey.ID
	pad := int64(d2svg.DEFAULT_PADDING)
	renderOpts := &d2svg.RenderOpts{
		ThemeID: &themeID,
		Pad:     &pad,
	}

	svg, err := d2svg.Render(diagram, renderOpts)
	if err != nil {
		return "", fmt.Errorf("render error: %w", err)
	}

	return string(svg), nil
}

type codeBlockExtension struct {
	style string
}

func (e *codeBlockExtension) Extend(m goldmark.Markdown) {
	ruler, _ := textmeasure.NewRuler()
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(&codeBlockRenderer{style: e.style, ruler: ruler}, 100),
		),
	)
}

func NewCodeBlockExtension(style string) goldmark.Extender {
	return &codeBlockExtension{style: style}
}

func newCodeBlockRenderer(style string) *codeBlockRenderer {
	ruler, _ := textmeasure.NewRuler()
	return &codeBlockRenderer{style: style, ruler: ruler}
}

type codeBlockExtensionWithRenderer struct {
	renderer *codeBlockRenderer
}

func (e *codeBlockExtensionWithRenderer) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(e.renderer, 100),
		),
	)
}
