package markdown

import (
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

func PlainText(source string) string {
	md := getRenderer(DefaultCodeTheme)
	src := []byte(source)
	doc := md.Parser().Parse(text.NewReader(src))

	var sb strings.Builder
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := n.(type) {
		case *ast.Text:
			sb.Write(v.Segment.Value(src))
		case *ast.String:
			sb.Write(v.Value)
		case *ast.AutoLink:
			sb.Write(v.URL(src))
		}
		switch n.Kind() {
		case ast.KindParagraph, ast.KindHeading, ast.KindListItem, ast.KindCodeBlock, ast.KindFencedCodeBlock, ast.KindBlockquote:
			sb.WriteByte(' ')
		}
		return ast.WalkContinue, nil
	})

	return strings.Join(strings.Fields(sb.String()), " ")
}
