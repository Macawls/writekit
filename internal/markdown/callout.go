package markdown

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

var calloutTypeRegex = regexp.MustCompile(`^\[!(NOTE|TIP|WARNING|DANGER|INFO|IMPORTANT|CAUTION)\]`)

type CalloutType string

const (
	CalloutNote      CalloutType = "note"
	CalloutTip       CalloutType = "tip"
	CalloutWarning   CalloutType = "warning"
	CalloutDanger    CalloutType = "danger"
	CalloutInfo      CalloutType = "info"
	CalloutImportant CalloutType = "important"
	CalloutCaution   CalloutType = "caution"
)

func normalizeCalloutType(raw string) CalloutType {
	switch strings.ToLower(raw) {
	case "note":
		return CalloutNote
	case "tip":
		return CalloutTip
	case "warning":
		return CalloutWarning
	case "danger":
		return CalloutDanger
	case "info":
		return CalloutInfo
	case "important":
		return CalloutImportant
	case "caution":
		return CalloutCaution
	default:
		return CalloutNote
	}
}

func calloutTypeToClass(ct CalloutType) string {
	switch ct {
	case CalloutImportant:
		return "callout-warning"
	case CalloutCaution:
		return "callout-danger"
	default:
		return "callout-" + string(ct)
	}
}

type calloutBlockquoteRenderer struct {
	html.Config
}

func newCalloutBlockquoteRenderer() renderer.NodeRenderer {
	return &calloutBlockquoteRenderer{
		Config: html.NewConfig(),
	}
}

func (r *calloutBlockquoteRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindBlockquote, r.renderBlockquote)
}

func (r *calloutBlockquoteRenderer) renderBlockquote(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	blockquote := node.(*ast.Blockquote)

	if entering {
		calloutType, contentStart := detectCallout(blockquote, source)

		if calloutType == "" {
			w.WriteString("<blockquote>\n")
			return ast.WalkContinue, nil
		}

		w.WriteString(`<div class="callout `)
		w.WriteString(calloutTypeToClass(calloutType))
		w.WriteString(`">`)
		w.WriteString(`<div class="callout-icon">`)
		w.WriteString(calloutIcon(calloutType))
		w.WriteString(`</div>`)
		w.WriteString(`<div class="callout-content">`)

		renderCalloutContent(w, source, blockquote, contentStart)

		w.WriteString(`</div></div>`)
		return ast.WalkSkipChildren, nil
	}

	w.WriteString("</blockquote>\n")
	return ast.WalkContinue, nil
}

func detectCallout(blockquote *ast.Blockquote, source []byte) (CalloutType, int) {
	firstChild := blockquote.FirstChild()
	if firstChild == nil {
		return "", 0
	}

	para, ok := firstChild.(*ast.Paragraph)
	if !ok {
		return "", 0
	}

	var lineBuilder bytes.Buffer
	for child := para.FirstChild(); child != nil; child = child.NextSibling() {
		switch n := child.(type) {
		case *ast.Text:
			segment := n.Segment
			lineBuilder.Write(segment.Value(source))
			if n.SoftLineBreak() || n.HardLineBreak() {
				break
			}
		case *ast.String:
			lineBuilder.Write(n.Value)
		}
	}
	lineContent := lineBuilder.Bytes()

	match := calloutTypeRegex.FindSubmatch(lineContent)
	if match == nil {
		return "", 0
	}

	calloutType := normalizeCalloutType(string(match[1]))
	contentStart := len(match[0])

	return calloutType, contentStart
}

func renderCalloutContent(w util.BufWriter, source []byte, blockquote *ast.Blockquote, skipBytes int) {
	for child := blockquote.FirstChild(); child != nil; child = child.NextSibling() {
		skipBytes = renderCalloutNode(w, source, child, skipBytes, true)
	}
}

func renderCalloutNode(w util.BufWriter, source []byte, node ast.Node, skipBytes int, isFirstParagraph bool) int {
	switch n := node.(type) {
	case *ast.Paragraph:
		w.WriteString("<p>")
		remaining := skipBytes
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			if isFirstParagraph && remaining > 0 {
				if text, ok := child.(*ast.Text); ok {
					segment := text.Segment
					content := segment.Value(source)
					contentLen := len(content)

					if remaining >= contentLen {
						remaining -= contentLen
						continue
					} else if remaining > 0 {
						toWrite := content[remaining:]
						toWrite = bytes.TrimLeft(toWrite, " \t")
						if len(toWrite) > 0 {
							w.Write(toWrite)
						}
						remaining = 0
						if text.HardLineBreak() || text.SoftLineBreak() {
							w.WriteString("<br />\n")
						}
						continue
					}
				}
			}
			remaining = renderCalloutNode(w, source, child, remaining, false)
		}
		w.WriteString("</p>\n")
		return 0

	case *ast.Text:
		segment := n.Segment
		content := segment.Value(source)
		w.Write(content)
		if n.HardLineBreak() {
			w.WriteString("<br />\n")
		} else if n.SoftLineBreak() {
			w.WriteString("<br />\n")
		}
		return 0

	case *ast.CodeSpan:
		w.WriteString("<code>")
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			renderCalloutNode(w, source, child, 0, false)
		}
		w.WriteString("</code>")
		return 0

	case *ast.Emphasis:
		if n.Level == 2 {
			w.WriteString("<strong>")
		} else {
			w.WriteString("<em>")
		}
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			renderCalloutNode(w, source, child, 0, false)
		}
		if n.Level == 2 {
			w.WriteString("</strong>")
		} else {
			w.WriteString("</em>")
		}
		return 0

	case *ast.Link:
		w.WriteString(`<a href="`)
		w.Write(util.EscapeHTML(n.Destination))
		w.WriteString(`">`)
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			renderCalloutNode(w, source, child, 0, false)
		}
		w.WriteString("</a>")
		return 0

	case *ast.String:
		w.Write(n.Value)
		return 0

	default:
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			renderCalloutNode(w, source, child, 0, false)
		}
		return 0
	}
}

func calloutIcon(ct CalloutType) string {
	switch ct {
	case CalloutNote:
		return `<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>`
	case CalloutTip:
		return `<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M15 14c.2-1 .7-1.7 1.5-2.5 1-.9 1.5-2.2 1.5-3.5A6 6 0 0 0 6 8c0 1 .2 2.2 1.5 3.5.7.7 1.3 1.5 1.5 2.5"/><path d="M9 18h6"/><path d="M10 22h4"/></svg>`
	case CalloutWarning, CalloutImportant:
		return `<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3"/><path d="M12 9v4"/><path d="M12 17h.01"/></svg>`
	case CalloutDanger, CalloutCaution:
		return `<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10"/><path d="m14.5 9-5 5"/><path d="m9.5 9 5 5"/></svg>`
	case CalloutInfo:
		return `<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>`
	default:
		return `<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>`
	}
}

type calloutExtension struct{}

func (e *calloutExtension) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(newCalloutBlockquoteRenderer(), 1),
		),
	)
}

func NewCalloutExtension() goldmark.Extender {
	return &calloutExtension{}
}
