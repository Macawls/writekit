package markdown

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

type headingRenderer struct {
	html.Config
}

func newHeadingRenderer() renderer.NodeRenderer {
	return &headingRenderer{Config: html.NewConfig()}
}

func (r *headingRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindHeading, r.renderHeading)
}

func (r *headingRenderer) renderHeading(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Heading)
	if entering {
		w.WriteString("<h")
		w.WriteByte("0123456"[n.Level])
		if n.Attributes() != nil {
			html.RenderAttributes(w, node, html.HeadingAttributeFilter)
		}
		w.WriteByte('>')

		if id, ok := n.AttributeString("id"); ok {
			var idStr string
			switch v := id.(type) {
			case []byte:
				idStr = string(v)
			case string:
				idStr = v
			}
			if idStr != "" {
				w.WriteString(`<a class="heading-anchor" href="#`)
				w.WriteString(idStr)
				w.WriteString(`">`)
			}
		}
	} else {
		if _, ok := n.AttributeString("id"); ok {
			w.WriteString("</a>")
		}
		w.WriteString("</h")
		w.WriteByte("0123456"[n.Level])
		w.WriteString(">\n")
	}
	return ast.WalkContinue, nil
}

type headingExtension struct{}

func (e *headingExtension) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(newHeadingRenderer(), 1),
		),
	)
}

func NewHeadingExtension() goldmark.Extender {
	return &headingExtension{}
}
