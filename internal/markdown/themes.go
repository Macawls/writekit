package markdown

import (
	"strings"
	"sync"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
)

var cssCache sync.Map

func GenerateChromaCSS(themeName string) (string, error) {
	if themeName == "" {
		themeName = DefaultCodeTheme
	}

	if cached, ok := cssCache.Load(themeName); ok {
		return cached.(string), nil
	}

	style := styles.Get(themeName)
	if style == nil {
		style = styles.Get(DefaultCodeTheme)
	}

	formatter := chromahtml.New(chromahtml.WithClasses(true))
	var css strings.Builder
	if err := formatter.WriteCSS(&css, style); err != nil {
		return "", err
	}

	result := css.String()
	cssCache.Store(themeName, result)
	return result, nil
}

func ListThemes() []string {
	return styles.Names()
}
