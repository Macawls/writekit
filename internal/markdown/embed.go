package markdown

import (
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var embedTagRegex = regexp.MustCompile(`<embed\s+src=["']([^"']+)["']\s*/?>`)

type EmbedProvider string

const (
	ProviderYouTube    EmbedProvider = "youtube"
	ProviderSpotify    EmbedProvider = "spotify"
	ProviderSoundCloud EmbedProvider = "soundcloud"
	ProviderTwitter    EmbedProvider = "twitter"
	ProviderGist       EmbedProvider = "gist"
	ProviderUnknown    EmbedProvider = "unknown"
)

func detectEmbedProvider(rawURL string) EmbedProvider {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ProviderUnknown
	}
	host := strings.ToLower(u.Host)

	switch {
	case strings.Contains(host, "youtube.com") || strings.Contains(host, "youtu.be"):
		return ProviderYouTube
	case strings.Contains(host, "spotify.com"):
		return ProviderSpotify
	case strings.Contains(host, "soundcloud.com"):
		return ProviderSoundCloud
	case strings.Contains(host, "twitter.com") || strings.Contains(host, "x.com"):
		return ProviderTwitter
	case strings.Contains(host, "gist.github.com"):
		return ProviderGist
	default:
		return ProviderUnknown
	}
}

var KindEmbed = ast.NewNodeKind("Embed")

type EmbedNode struct {
	ast.BaseBlock
	URL      string
	Provider EmbedProvider
}

func NewEmbed(rawURL string) *EmbedNode {
	return &EmbedNode{
		URL:      rawURL,
		Provider: detectEmbedProvider(rawURL),
	}
}

func (n *EmbedNode) Kind() ast.NodeKind {
	return KindEmbed
}

func (n *EmbedNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{
		"URL":      n.URL,
		"Provider": string(n.Provider),
	}, nil)
}

type embedParser struct{}

func (p *embedParser) Trigger() []byte {
	return []byte{'<'}
}

func (p *embedParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, _ := reader.PeekLine()
	if match := embedTagRegex.FindSubmatch(line); match != nil {
		rawURL := string(match[1])
		reader.Advance(len(match[0]))
		return NewEmbed(rawURL), parser.NoChildren
	}
	return nil, parser.NoChildren
}

func (p *embedParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	return parser.Close
}

func (p *embedParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {}

func (p *embedParser) CanInterruptParagraph() bool { return true }

func (p *embedParser) CanAcceptIndentedLine() bool { return false }

type embedRenderer struct{}

func newEmbedRenderer() renderer.NodeRenderer {
	return &embedRenderer{}
}

func (r *embedRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindEmbed, r.renderEmbed)
}

func (r *embedRenderer) renderEmbed(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	embed := node.(*EmbedNode)

	switch embed.Provider {
	case ProviderYouTube:
		r.renderYouTube(w, embed.URL)
	case ProviderSpotify:
		r.renderSpotify(w, embed.URL)
	case ProviderSoundCloud:
		r.renderSoundCloud(w, embed.URL)
	case ProviderTwitter:
		r.renderTwitter(w, embed.URL)
	case ProviderGist:
		r.renderGist(w, embed.URL)
	default:
		r.renderGeneric(w, embed.URL)
	}

	return ast.WalkContinue, nil
}

func (r *embedRenderer) renderYouTube(w util.BufWriter, rawURL string) {
	id := extractYouTubeID(rawURL)
	if id == "" {
		r.renderGeneric(w, rawURL)
		return
	}

	thumbnail := fmt.Sprintf("https://img.youtube.com/vi/%s/maxresdefault.jpg", id)
	w.WriteString(`<div class="embed embed-youtube" data-id="`)
	w.WriteString(html.EscapeString(id))
	w.WriteString(`">`)
	w.WriteString(`<img src="`)
	w.WriteString(html.EscapeString(thumbnail))
	w.WriteString(`" alt="Video thumbnail" loading="lazy">`)
	w.WriteString(`<button class="embed-play" aria-label="Play video">`)
	w.WriteString(`<svg viewBox="0 0 24 24" fill="currentColor"><path d="M8 5v14l11-7z"/></svg>`)
	w.WriteString(`</button>`)
	w.WriteString(`</div>`)
}

func extractYouTubeID(rawURL string) string {
	patterns := []string{
		`(?:youtube\.com/watch\?v=|youtu\.be/|youtube\.com/embed/)([a-zA-Z0-9_-]{11})`,
		`youtube\.com/shorts/([a-zA-Z0-9_-]{11})`,
	}
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindStringSubmatch(rawURL); match != nil {
			return match[1]
		}
	}
	return ""
}

func (r *embedRenderer) renderSpotify(w util.BufWriter, rawURL string) {
	re := regexp.MustCompile(`open\.spotify\.com/(track|album|playlist|episode|show)/([a-zA-Z0-9]+)`)
	match := re.FindStringSubmatch(rawURL)
	if match == nil {
		r.renderGeneric(w, rawURL)
		return
	}

	embedType := match[1]
	id := match[2]
	embedURL := fmt.Sprintf("https://open.spotify.com/embed/%s/%s", embedType, id)

	height := "152"
	if embedType != "track" {
		height = "352"
	}

	w.WriteString(`<div class="embed embed-spotify">`)
	w.WriteString(`<iframe src="`)
	w.WriteString(html.EscapeString(embedURL))
	w.WriteString(`" height="`)
	w.WriteString(height)
	w.WriteString(`" frameborder="0" allowtransparency="true" allow="encrypted-media" loading="lazy"></iframe>`)
	w.WriteString(`</div>`)
}

func (r *embedRenderer) renderSoundCloud(w util.BufWriter, rawURL string) {
	encodedURL := url.QueryEscape(rawURL)
	embedURL := fmt.Sprintf("https://w.soundcloud.com/player/?url=%s&color=%%23ff5500&auto_play=false&hide_related=false&show_comments=true&show_user=true&show_reposts=false&show_teaser=true", encodedURL)

	w.WriteString(`<div class="embed embed-soundcloud">`)
	w.WriteString(`<iframe src="`)
	w.WriteString(html.EscapeString(embedURL))
	w.WriteString(`" height="166" scrolling="no" frameborder="no" allow="autoplay" loading="lazy"></iframe>`)
	w.WriteString(`</div>`)
}

func (r *embedRenderer) renderTwitter(w util.BufWriter, rawURL string) {
	w.WriteString(`<div class="embed embed-twitter">`)
	w.WriteString(`<blockquote class="twitter-tweet" data-dnt="true">`)
	w.WriteString(`<a href="`)
	w.WriteString(html.EscapeString(rawURL))
	w.WriteString(`">Loading tweet...</a>`)
	w.WriteString(`</blockquote>`)
	w.WriteString(`</div>`)
}

func (r *embedRenderer) renderGist(w util.BufWriter, rawURL string) {
	w.WriteString(`<div class="embed embed-gist" data-url="`)
	w.WriteString(html.EscapeString(rawURL))
	w.WriteString(`">`)
	w.WriteString(`<noscript><a href="`)
	w.WriteString(html.EscapeString(rawURL))
	w.WriteString(`">View Gist on GitHub</a></noscript>`)
	w.WriteString(`</div>`)
}

func (r *embedRenderer) renderGeneric(w util.BufWriter, rawURL string) {
	w.WriteString(`<div class="embed">`)
	w.WriteString(`<a href="`)
	w.WriteString(html.EscapeString(rawURL))
	w.WriteString(`" target="_blank" rel="noopener">`)
	w.WriteString(html.EscapeString(rawURL))
	w.WriteString(`</a>`)
	w.WriteString(`</div>`)
}

type embedExtension struct{}

func (e *embedExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithBlockParsers(
			util.Prioritized(&embedParser{}, 50),
		),
	)
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(newEmbedRenderer(), 100),
		),
	)
}

func NewEmbedExtension() goldmark.Extender {
	return &embedExtension{}
}
