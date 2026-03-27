package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerPrompts(mcpServer *mcp.Server) {
	mcpServer.AddPrompt(&mcp.Prompt{
		Name:        "write_page",
		Description: "Write a well-formatted page about a topic",
		Arguments: []*mcp.PromptArgument{
			{Name: "topic", Description: "What to write about", Required: true},
			{Name: "audience", Description: "Target audience (e.g., beginners, senior devs)"},
			{Name: "style", Description: "Writing style (e.g., tutorial, opinion, guide)"},
		},
	}, s.writePostPrompt)
}

func (s *Server) writePostPrompt(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	topic := req.Params.Arguments["topic"]
	audience := req.Params.Arguments["audience"]
	style := req.Params.Arguments["style"]

	if audience == "" {
		audience = "developers"
	}
	if style == "" {
		style = "tutorial"
	}

	prompt := fmt.Sprintf(`Write a page about: %s

Target audience: %s
Style: %s

Structure the post with:
1. An engaging introduction (2-3 sentences)
2. Clear sections with ## headings
3. Code examples in fenced blocks with language tags (e.g., `+"`"+`go`+"`"+`, `+"`"+`python`+"`"+`) — these render with syntax highlighting and a copy button
4. Use > [!TIP] or > [!NOTE] callout blocks for important tips or caveats
5. A conclusion or summary

Formatting guidelines:
- Use **bold** for key terms on first mention
- Use inline `+"`"+`code`+"`"+` for function names, variables, and CLI commands
- Use bullet lists for related items, numbered lists for sequential steps
- Use tables for comparisons
- Include links to relevant documentation
- Add a `+"`"+`> [!WARNING]`+"`"+` callout for common pitfalls

After writing, use create_page to save it as a draft, then share the preview URL.`, topic, audience, style)

	return &mcp.GetPromptResult{
		Description: fmt.Sprintf("Write a %s about %s for %s", style, topic, audience),
		Messages: []*mcp.PromptMessage{
			{Role: "user", Content: &mcp.TextContent{Text: prompt}},
		},
	}, nil
}
