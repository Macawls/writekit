<div align="center">

# WriteKit

**Publish by conversation.**

A markdown publishing platform you operate through your AI assistant. Pages, collections, drafts, and previews — all driven by MCP tool calls instead of a dashboard.

[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue.svg?style=flat-square)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.25-00ADD8?style=flat-square&logo=go)](go.mod)
[![Release](https://img.shields.io/github/v/release/Macawls/writekit?style=flat-square&color=green)](https://github.com/Macawls/writekit/releases/latest)
[![MCP](https://img.shields.io/badge/MCP-native-7C3AED?style=flat-square)](https://modelcontextprotocol.io)
[![Website](https://img.shields.io/badge/site-writekit.dev-0f0f0f?style=flat-square)](https://writekit.dev)

</div>

---

WriteKit is built around the [Model Context Protocol](https://modelcontextprotocol.io). Point any MCP-capable client — Claude Code, Claude Desktop, Cursor, Windsurf, VS Code, Zed, OpenCode, Cline, Roo Code, Junie, or ChatGPT — at the server and your AI gets typed tools for creating pages, ordering collections, publishing, inviting team members, and configuring your site. No dashboard required.

It ships as two things from one codebase: a hosted multi-tenant service at [writekit.dev](https://writekit.dev), and a Wails desktop app that runs entirely on your machine.

## Get started

**Hosted — zero setup, $5/month.**

```bash
claude mcp add --transport http writekit https://mcp.writekit.dev
```

Sign in at [writekit.dev](https://writekit.dev) with Google, GitHub, or Discord, pick a subdomain, and tell your assistant what to write. OAuth 2.0 + PKCE handles MCP auth — no API keys to copy around.

**Desktop — local-first, no account.**

Grab a build for your OS from [Releases](https://github.com/Macawls/writekit/releases/latest). Everything stays on your machine: pages live in a single SQLite file under your user data directory, and the desktop app exposes a loopback MCP server (`127.0.0.1`) so Claude Desktop or any MCP client can read and write to it. The tray menu has a "Copy MCP URL" item to wire up a client in one paste.

## What's in the box

- **Pages** — markdown documents with a title, slug, optional excerpt, tags, visibility, and draft/published status. Drafts never appear on the live site; share them via 24-hour preview links.
- **Collections** — ordered groups of pages, sorted manually or by date. Use one for docs, one for a changelog, one for a tutorial series.
- **Tags** — free-form labels per page; filter the page list, surface related work, group by topic.
- **Visibility** — `public`, `unlisted` (URL-only), or `private` (team members only), per page and per collection.
- **Versions** — every edit is stored as `v1, v2, v3…`. Rewind by asking your AI to "go back to v3."
- **Live preview** — preview tab auto-reloads via SSE every time the page is saved. Side-panel it next to your chat.
- **Teams** — `owner` / `editor` / `viewer` roles on the hosted service; invite by email.
- **Custom domains** — point a CNAME at `cname.writekit.dev` and certs are issued automatically. The original subdomain keeps working.
- **Semantic graph** — embedding-powered similarity graph of your pages in 2D or 3D. Embeddings run client-side in the browser via [transformers.js](https://huggingface.co/docs/transformers.js); no server-side embedding worker required.

## The MCP surface

**Tools** (`internal/mcp/`):

| Pages | Collections | Settings | Team |
|---|---|---|---|
| `create_page` | `create_collection` | `get_settings` | `list_members` |
| `update_page` | `update_collection` | `update_settings` | `invite_member` |
| `append_to_page` | `delete_collection` | `rename_subdomain` | `update_member_role` |
| `delete_page` | `list_collections` | | `remove_member` |
| `publish_page` | `get_collection` | | `list_invitations` |
| `unpublish_page` | `reorder_pages` | | `revoke_invitation` |
| `list_pages` | | | `resend_invitation` |
| `get_page` | | | |
| `search_pages` | | | |

**Resources** (read-only context the AI can pull on demand):

- `writekit://site/stats` — published / draft / collection counts
- `writekit://site/settings` — current site settings
- `writekit://site/recent-pages` — last 10 published
- `writekit://site/drafts` — every draft
- `writekit://site/collections` — collections with page counts

**Prompts**: `write_page` scaffolds a new page from a topic, optional audience, and optional style.

Full reference at [writekit.dev/docs](https://writekit.dev/docs).

## Markdown, plus

Standard CommonMark + GFM, with a few extensions that matter:

- **Syntax-highlighted code blocks** (chroma) with language tags and a copy button
- **Callouts** — `> [!NOTE]`, `> [!TIP]`, `> [!WARNING]`, `> [!DANGER]`
- **Media embeds** — `<embed src="https://...">` for YouTube, Spotify, SoundCloud, Twitter/X, GitHub Gists
- **D2 diagrams** — ```` ```d2 ```` fenced blocks rendered server-side to interactive SVG
- **Footnotes**, task lists, tables, image lightbox, raw HTML

## The two builds

| | Hosted | Desktop |
|---|---|---|
| URL | `writekit.dev` | `http://127.0.0.1:<port>` |
| Storage | Postgres (platform) + per-tenant SQLite | SQLite only |
| Accounts | Google / GitHub / Discord OAuth + magic link | None — single user, loopback trust |
| MCP auth | OAuth 2.0 + PKCE | Unauthenticated (loopback only) |
| Multi-tenant | Yes — subdomain or custom domain per site | No — single site |
| Teams | Yes | No |
| Entry | `cmd/writekit` | `desktop/` (Wails v2 wrapper) |

Both share `internal/`. The mode switch lives in [`internal/app/app.go`](internal/app/app.go) (`buildLocalRouter` vs `buildRouter`); local mode skips OAuth, Stripe, subdomains, and the platform DB entirely, injecting identity from loopback context.

## Architecture

- **`internal/platform/`** — Postgres. Users, tenants, sessions, OAuth linked accounts, team members, magic links, Stripe. Hosted only.
- **`internal/tenant/`** — per-tenant SQLite (WAL + LRU pool). Pages, collections, `page_versions`, `page_embeddings`, settings. The only DB in desktop mode.
- **`internal/mcp/`** — MCP server, tools, resources, prompts; uses [`github.com/modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk). `resolver.go` maps each request to `(tenant, user, role)`.
- **`internal/render` / `internal/markdown`** — goldmark + chroma + D2 + embed/callout extensions, fronted by a cache that invalidates on `events.Bus` mutations.
- **`internal/site` / `internal/api`** — public site rendering and the JSON API the SPAs talk to.
- **`apps/user`** — React/Vite SPA served at `app.writekit.dev` (hosted) and inside the desktop window. `wails dev` proxies its webview through Vite, so React + CSS HMR work live.
- **`apps/admin`** — platform admin SPA.
- **`templates/web/`** — server-rendered marketing, auth, billing, docs.

Embedded into the Go binary via [`embed.go`](embed.go): `templates/`, `static/`, `apps/user/dist/`, `apps/admin/dist/`. Release builds run `bun run build` for both SPAs before `go build`.

## Local development

```bash
# Desktop loop — backend, MCP, Vite, and the webview start from one command
cd desktop && wails dev
```

That gives you:

- Live HMR for `apps/user/src/**` inside the Wails window
- The same SPA at `http://localhost:5173` in a regular browser (`/api` is proxied to `:8787`)
- A loopback MCP endpoint at `http://127.0.0.1:8787/mcp` — point Claude Code at it and start mutating SQLite

> `LOCAL=true go run ./cmd/writekit` will boot, but it skips the desktop workspace bootstrap and `/api/me` returns `site: null` forever. Use `wails dev`.

For hosted dev (Postgres, OAuth, subdomain matching on `app.localhost` / `mcp.localhost`):

```bash
docker compose up -d
export DATABASE_URL=postgres://writekit:writekit@localhost/writekit
export SESSION_SECRET=$(openssl rand -hex 32)
go run ./cmd/writekit
```

Before pushing: `go test ./...`, `go vet ./...`, `gofmt -s -w .`. See [`AGENTS.md`](AGENTS.md) for the rest of the conventions.

## Self-hosting

Same command as hosted dev, plus a wildcard `*.yourdomain.tld` DNS record pointed at your box. OAuth providers and AWS SES for magic-link mail are optional — the server boots with any subset configured. Embeddings run in the user's browser, so no embedding model needs to live on the server.

All env vars are declared in [`internal/config/config.go`](internal/config/config.go).

## License

[AGPL-3.0-or-later](LICENSE). The whole codebase is open source. AGPL §13 (network-use) means: if you modify WriteKit and run your fork as a service, you publish your changes under the same license. Self-hosting, personal use, and modification are unrestricted.
