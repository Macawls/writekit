<div align="center">

# WriteKit

**Publish by conversation.**

Pages, collections, and docs — managed entirely through your AI assistant.

[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue.svg?style=flat-square)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.25-00ADD8?style=flat-square&logo=go)](go.mod)
[![Release](https://img.shields.io/github/v/release/Macawls/writekit?style=flat-square&color=green)](https://github.com/Macawls/writekit/releases/latest)
[![MCP](https://img.shields.io/badge/MCP-native-7C3AED?style=flat-square)](https://modelcontextprotocol.io)
[![Website](https://img.shields.io/badge/site-writekit.dev-0f0f0f?style=flat-square)](https://writekit.dev)

</div>

---

WriteKit is an MCP-native publishing platform. Connect your AI assistant — Claude, Cursor, Windsurf, or anything else that speaks [Model Context Protocol](https://modelcontextprotocol.io) — and it creates pages, organises them into collections, and publishes them. No dashboard-clicking required; the dashboard is there when you want it.

## Get started

**Hosted — zero setup.**

```bash
claude mcp add --transport http writekit https://writekit.dev/mcp
```

Sign in at [writekit.dev](https://writekit.dev) with Google, GitHub, or Discord, pick a subdomain, and start telling your assistant what to write.

**Desktop — local-first, no account.**

Download WriteKit for your OS from [Releases](https://github.com/Macawls/writekit/releases/latest). It runs entirely on your machine — data lives in `%APPDATA%\WriteKit` (or `~/Library/Application Support/WriteKit`, `~/.local/share/WriteKit`). A local MCP server is exposed on `127.0.0.1` so Claude Desktop (or any MCP client) can read and write to it in the background.

## What you get

- **Pages**: any markdown content — a doc, blog post, tutorial. Start as drafts, publish when ready.
- **Collections**: group related pages (docs, a blog, a series). Ordered manually or by date.
- **Visibility**: `public`, `unlisted` (URL-only), or `private` (team members only) — per page and per collection.
- **Teams**: `owner` / `editor` / `viewer` roles on the hosted service; invite members by email.
- **Semantic graph**: embedding-powered similarity graph of your pages, visualised in 2D or 3D. Uses [Ollama](https://ollama.com/download) for local embeddings.
- **MCP tools**: `create_page`, `update_page`, `publish_page`, `list_pages`, `search_pages`, `create_collection`, `update_settings`, and more. See [writekit.dev/docs](https://writekit.dev/docs) for the full list.

## Markdown, plus

Standard CommonMark + GFM, with a few extensions that matter:

- **Syntax-highlighted code blocks** with language tags and a copy button
- **Callouts**: `> [!NOTE]`, `> [!TIP]`, `> [!WARNING]`, `> [!DANGER]`
- **Media embeds**: `<embed src="https://...">` for YouTube, Spotify, SoundCloud, Twitter/X, GitHub Gists
- **D2 diagrams**: write architecture diagrams in ```` ```d2 ```` fenced blocks, rendered server-side to interactive SVG
- **Footnotes**, task lists, tables, raw HTML

## The two builds

One codebase, two artifacts:

| | Hosted | Desktop |
|---|---|---|
| URL | `writekit.dev` | `http://127.0.0.1:<port>` |
| Storage | Postgres (platform) + per-tenant SQLite | SQLite only |
| Accounts | Google / GitHub / Discord OAuth | None — single user, loopback trust |
| MCP auth | OAuth 2.0 + PKCE | Unauthenticated (loopback only) |
| Multi-tenant | Yes — subdomain per site | No — single site |
| Entry | `cmd/writekit` | `desktop/` (Wails v2 wrapper) |

## Self-hosting

```bash
git clone https://github.com/Macawls/writekit
cd writekit
docker compose up -d            # starts postgres + ollama
export DATABASE_URL=postgres://writekit:writekit@localhost/writekit
export SESSION_SECRET=$(openssl rand -hex 32)
go run ./cmd/writekit
```

Point DNS at your box with a wildcard `*.yourdomain.tld` record and you've got a multi-tenant publishing service. OAuth providers (Google/GitHub/Discord), AWS SES for magic-link email, and Ollama for embeddings are all optional — the server boots with any subset configured.

All env vars are declared in [`internal/config/config.go`](internal/config/config.go).

## License

[AGPL-3.0-or-later](LICENSE). The entire codebase is open source. AGPL §13 (network-use) means: if you modify WriteKit and run your fork as a service, you must publish your changes under the same license. Self-hosting, personal use, and modification are unrestricted.

