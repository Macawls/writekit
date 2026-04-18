# Welcome to WriteKit

> [!TIP]
> This post is **private** — only you can see it. Delete it when you've got the hang of things, or publish it as your first post by changing its visibility.

WriteKit is a publishing platform you drive with your AI assistant. You write in Markdown, the assistant takes care of the rest — creating pages, organising collections, publishing, updating.

## Try it

Open your MCP client (Claude, Cursor, Windsurf — whatever you've connected) and tell it something like:

> *"Write me a short introduction post about the book I just finished — The Three-Body Problem — and publish it as unlisted."*

Your assistant will call `create_page`, draft the content, set the right visibility, and publish.

## Visibility

Every page and collection has a visibility setting:

- **public** — visible to everyone, shown in your site index
- **unlisted** — accessible via URL, hidden from the index and sitemap
- **private** — only visible to team members (like this page)

## Markdown, plus

Standard Markdown, plus:

- **Callouts**: `> [!NOTE]`, `> [!TIP]`, `> [!WARNING]`, `> [!DANGER]`
- **Code blocks** with language tags:
  ```go
  package main
  import "fmt"
  func main() { fmt.Println("hello") }
  ```
- **D2 diagrams** in ` ```d2 ` fenced blocks
- **Media embeds**: `<embed src="https://...">` for YouTube, Spotify, Twitter/X, GitHub Gists

## Next steps

- Ask your assistant to write a real post
- Group related posts into a **collection** (`create_collection`)
- Check the **Graph** tab to see how your pages connect semantically (requires Ollama)

Happy writing.
