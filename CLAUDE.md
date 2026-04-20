# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Hosted build (Postgres + multi-tenant)
go run ./cmd/writekit                 # requires DATABASE_URL, SESSION_SECRET
go build -o writekit ./cmd/writekit

# Desktop build (Wails v2, local-only, SQLite only)
cd desktop && wails dev               # dev with hot-reload
cd desktop && wails build             # produces WriteKit binary

# Frontends (React + Vite, bun)
cd apps/user && bun install && bun run dev     # user SPA (localhost site UI + desktop)
cd apps/admin && bun install && bun run build  # platform admin SPA

# Tests
go test ./...
go test ./internal/tenant -run TestName    # single test

# Infra (self-hosting)
docker compose up -d                  # postgres
```

The Go binary embeds `templates/`, `static/`, `apps/user/dist/`, and `apps/admin/dist/` via `embed.go` — SPA dists must be built before `go build` in release workflows.

## Architecture

**Two build targets, one codebase.** `cmd/writekit` is the hosted multi-tenant server; `desktop/` is a Wails wrapper that imports the same `internal/app` package with `LOCAL=true`. Branching happens in `internal/app/app.go` (`buildLocalRouter` vs `buildRouter`). Local mode has no `platform` DB, no OAuth, no Stripe, no subdomains — identity is injected via context from loopback, not via a `platform` abstraction or LocalDB shim.

**Request routing (hosted):** `internal/app/app.go` dispatches by host:
- apex (`writekit.dev`) → `web` (auth, billing, docs, settings) + `admin` SPA + MCP endpoint
- subdomain (`{slug}.writekit.dev`) → `site` (rendered blog) + `api` (graph/embeddings) + user SPA
- custom domain → resolved through `platform.DB` to a tenant, served as site

**Data layers:**
- `internal/platform/` — Postgres. Users, tenants, sessions, OAuth linked accounts, team members, magic links, Stripe. Only exists in hosted mode.
- `internal/tenant/` — per-tenant SQLite file in `DataDir`, managed by a WAL+LRU `Pool`. Holds pages, collections, page_versions, page_embeddings, settings. This is the only DB in desktop mode.
- Migrations are embedded SQL under each package's `migrations/` directory and run at startup.

**MCP is the primary content API.** `internal/mcp/` registers tools (`tools_pages.go`, `tools_collections.go`, `tools_settings.go`, `tools_teams.go`) via `github.com/modelcontextprotocol/go-sdk`. `resolver.go` maps an inbound MCP request to `(tenant, user, role)` — differently in hosted (OAuth 2.0 + PKCE) vs local (loopback trust). When adding a tool, wire it through `server.go` and ensure the resolver provides everything it needs.

**Events + cache:** `internal/events.Bus` is an in-process pub/sub. `site.Cache` subscribes to invalidate rendered pages on writes. Embeddings are computed client-side (`apps/user/src/embedding/`) against `/api/embedding-source`; no server-side embedding worker. Any mutation path must still publish the appropriate event or renders go stale.

**Rendering:** `internal/render` wraps Go templates with dev-reload. `internal/markdown` is goldmark + chroma + a D2 renderer + embed/callout extensions — changes to markdown extensions must be kept in sync with the content guidelines the MCP server advertises to clients.

**Config:** all env vars declared in `internal/config/config.go`. `cfg.Local` is the desktop/single-tenant switch — check it before touching anything platform-specific.

## Conventions

- Skip code comments unless the *why* is genuinely non-obvious. No section-header or what-it-does comments.
- Desktop/local mode must not introduce a `platform` abstraction, LocalDB shim, or `PlatformStore` interface. Use context-injected identity.
- Dashboards are React/Vite SPAs (`apps/user`, `apps/admin`), not server-rendered templates. The marketing/auth/billing surface under `templates/web/` stays server-rendered.
- Keep MVP scope minimal — don't add features, fallbacks, or abstractions beyond what the task requires.
