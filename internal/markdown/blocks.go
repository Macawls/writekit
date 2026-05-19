package markdown

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

type Block struct {
	Kind   string
	Anchor string
	HTML   string
	Text   string
	Hash   string
}

func ExtractBlocks(source string) []Block {
	src := []byte(source)
	md := getRenderer(DefaultCodeTheme)
	doc := md.Parser().Parse(text.NewReader(src))

	var blocks []Block
	idx := 0
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		var buf bytes.Buffer
		if err := md.Renderer().Render(&buf, src, n); err != nil {
			continue
		}
		html := buf.String()
		plain := nodeText(n, src)
		sum := sha1.Sum([]byte(n.Kind().String() + "\x00" + html))
		hash := hex.EncodeToString(sum[:])
		anchor := fmt.Sprintf("b-%d-%s", idx, hash[:8])
		blocks = append(blocks, Block{
			Kind:   n.Kind().String(),
			Anchor: anchor,
			HTML:   html,
			Text:   plain,
			Hash:   hash,
		})
		idx++
	}
	return blocks
}

func RenderBlocksHTML(source string) string {
	blocks := ExtractBlocks(source)
	var sb strings.Builder
	for _, b := range blocks {
		fmt.Fprintf(&sb, `<div class="diff-block" data-block-anchor="%s">%s</div>`, b.Anchor, b.HTML)
	}
	return sb.String()
}

func RenderHunksHTML(hunks []Hunk) string {
	var sb strings.Builder
	for _, h := range hunks {
		switch h.Op {
		case OpKeep:
			fmt.Fprintf(&sb, `<div class="diff-block" data-block-anchor="%s">%s</div>`, h.Anchor, h.HTML)
		case OpAdd:
			fmt.Fprintf(&sb, `<div class="diff-block diff-static-add" data-block-anchor="%s">%s</div>`, h.Anchor, h.HTML)
		case OpDel:
			fmt.Fprintf(&sb, `<div class="diff-block diff-static-del" data-block-anchor="%s">%s</div>`, h.Anchor, h.OldHTML)
		case OpReplace:
			fmt.Fprintf(&sb, `<div class="diff-block diff-static-replace-old">%s</div>`, h.OldHTML)
			fmt.Fprintf(&sb, `<div class="diff-block diff-static-replace-new" data-block-anchor="%s">%s</div>`, h.Anchor, h.HTML)
		}
	}
	return sb.String()
}

func nodeText(n ast.Node, src []byte) string {
	var sb strings.Builder
	_ = ast.Walk(n, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := node.(type) {
		case *ast.Text:
			sb.Write(v.Segment.Value(src))
			sb.WriteByte(' ')
		case *ast.String:
			sb.Write(v.Value)
			sb.WriteByte(' ')
		case *ast.AutoLink:
			sb.Write(v.URL(src))
			sb.WriteByte(' ')
		case *ast.FencedCodeBlock:
			lines := v.Lines()
			for i := 0; i < lines.Len(); i++ {
				seg := lines.At(i)
				sb.Write(seg.Value(src))
				sb.WriteByte(' ')
			}
		case *ast.CodeBlock:
			lines := v.Lines()
			for i := 0; i < lines.Len(); i++ {
				seg := lines.At(i)
				sb.Write(seg.Value(src))
				sb.WriteByte(' ')
			}
		}
		return ast.WalkContinue, nil
	})
	return strings.Join(strings.Fields(sb.String()), " ")
}
