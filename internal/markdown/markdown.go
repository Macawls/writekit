package markdown

import (
	"bytes"
	stdhtml "html"
	"regexp"
	"strings"
	"sync"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

const DefaultCodeTheme = "github"

var (
	renderers   = make(map[string]goldmark.Markdown)
	renderersMu sync.RWMutex
)

func getRenderer(codeTheme string) goldmark.Markdown {
	if codeTheme == "" {
		codeTheme = DefaultCodeTheme
	}

	renderersMu.RLock()
	if md, ok := renderers[codeTheme]; ok {
		renderersMu.RUnlock()
		return md
	}
	renderersMu.RUnlock()

	renderersMu.Lock()
	defer renderersMu.Unlock()

	if md, ok := renderers[codeTheme]; ok {
		return md
	}

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Footnote,
			NewCodeBlockExtension(codeTheme),
			NewCalloutExtension(),
			NewEmbedExtension(),
			NewHeadingExtension(),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithUnsafe(),
		),
	)
	renderers[codeTheme] = md
	return md
}

func Render(source string) (string, error) {
	return RenderWithTheme(source, DefaultCodeTheme)
}

var (
	tagPattern   = regexp.MustCompile(`<[^>]+>`)
	spacePattern = regexp.MustCompile(`\s+`)
)

func Plain(source string) string {
	htmlOut, err := RenderWithTheme(source, DefaultCodeTheme)
	if err != nil {
		return strings.TrimSpace(spacePattern.ReplaceAllString(source, " "))
	}
	stripped := tagPattern.ReplaceAllString(htmlOut, " ")
	stripped = stdhtml.UnescapeString(stripped)
	return strings.TrimSpace(spacePattern.ReplaceAllString(stripped, " "))
}

func RenderWithTheme(source, codeTheme string) (string, error) {
	md := getRenderer(codeTheme)
	var buf bytes.Buffer
	if err := md.Convert([]byte(source), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func RenderWithErrors(source string) (string, []string) {
	var errs []string
	cbr := newCodeBlockRenderer(DefaultCodeTheme)
	cbr.errors = &errs

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Footnote,
			&codeBlockExtensionWithRenderer{cbr},
			NewCalloutExtension(),
			NewEmbedExtension(),
			NewHeadingExtension(),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithUnsafe(),
		),
	)

	var buf bytes.Buffer
	if err := md.Convert([]byte(source), &buf); err != nil {
		return source, []string{err.Error()}
	}
	return buf.String(), errs
}
