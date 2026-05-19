# WriteKit — Agent Guidelines

## What WriteKit Is

WriteKit is a markdown publishing platform managed primarily through an MCP server — users (and their AI assistants) create, organise, and publish pages by calling tools rather than clicking around a dashboard. Content is structured as **pages** (markdown documents) and **collections** (ordered groups of pages), suitable for documentation, guides, tutorials, changelogs, and long-form writing. It ships as two artifacts from one codebase: a hosted multi-tenant service (`writekit.dev`, Postgres + per-tenant SQLite, OAuth, Stripe) and a local-first desktop app (Wails, SQLite only, loopback-trust MCP).

Avoid "blog" terminology in UI, docs, and code — prefer "site", "page", "doc", or "collection".

## Go Style

- Follow [Google's Go style guide](https://google.github.io/styleguide/go).
- Prefer the standard library. Only reach for third-party packages when the stdlib genuinely cannot do the job.
- No comments unless an LLM cannot understand what the code is doing from the code alone. If you need a comment, the code is probably too clever. No section-header or what-it-does comments.

## MCP Protocol

- WriteKit exposes an MCP server as its primary content API. It implements the [Model Context Protocol](https://modelcontextprotocol.io).
- SDK: `github.com/modelcontextprotocol/go-sdk`. Before changing transports, JSON-RPC handling, or capability negotiation, check the latest spec.
- Tools live under `internal/mcp/` (`tools_pages.go`, `tools_collections.go`, `tools_settings.go`, `tools_teams.go`). `resolver.go` maps an inbound request to `(tenant, user, role)` — hosted uses OAuth 2.0 + PKCE, local uses loopback trust. New tools must be wired through `server.go` and the resolver must supply everything they need.

## Before Committing

- Run tests: `go test ./...`
- Run a single package/test: `go test ./internal/tenant -run TestName`
- `go vet ./...` and `gofmt -s -w .` before pushing.
- Fix failures rather than skipping or suppressing them.

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

# Infra (self-hosting)
docker compose up -d                  # postgres
```

The Go binary embeds `templates/`, `static/`, `apps/user/dist/`, and `apps/admin/dist/` via `embed.go` — SPA dists must be built before `go build` in release workflows.

## Dev loop: iterate on `apps/user` SPA + test MCP locally

`apps/user` is the same SPA served at `app.writekit.dev` (hosted) and inside the Wails desktop window. `wails dev` is the only supported way to run the local backend — `LOCAL=true go run ./cmd/writekit` boots an HTTP server but skips the workspace bootstrap in `desktop/main.go` (`auth.SetLocalWorkspaces`), so `/api/me` returns `site: null` forever and the SPA spins on "Setting up your site...".

`desktop/wails.json` is configured with `frontend:dev:watcher` and `frontend:dev:serverUrl: http://localhost:5173`, so `wails dev` spawns Vite itself and the AssetServer proxies the webview through it — CSS and React edits hot-reload inside the desktop window. The same Vite process also serves `http://localhost:5173` for browser-based iteration, with `/api` proxied to `:8787`.

```bash
# One terminal — backend, MCP, Vite, and the webview all start from here
cd desktop && wails dev
```

Then:

- The Wails window has live HMR (CSS + React) — edit `apps/user/src/**` and changes appear without restart.
- Open `http://localhost:5173` for the same SPA in a regular browser, also with HMR.
- Point Claude (Desktop / Code) at `http://127.0.0.1:8787/mcp` — loopback trust, no OAuth. The tray menu's "Copy MCP URL" gives you this.
- MCP tool calls mutate the SQLite store; HMR updates the SPA code, but data still requires a refetch/refresh to show.

If you only want the SPA against the hosted server (no local MCP), run Vite alone: `cd apps/user && VITE_BACKEND_URL=http://localhost:8080 bun run dev`.

Hosted dev (`go run ./cmd/writekit` on `:8080` with Postgres, OAuth, and subdomain host matching for `app.localhost` / `mcp.localhost`) is heavier — only reach for it when debugging hosted-only paths.

## Architecture

**Two build targets, one codebase.** `cmd/writekit` is the hosted multi-tenant server; `desktop/` is a Wails wrapper that imports the same `internal/app` package with `LOCAL=true`. Branching happens in `internal/app/app.go` (`buildLocalRouter` vs `buildRouter`). Local mode has no `platform` DB, no OAuth, no Stripe, no subdomains — identity is injected via context from loopback.

**Request routing (hosted):** `internal/app/app.go` dispatches by host:

- apex (`writekit.dev`) → `web` (auth, billing, docs, settings) + `admin` SPA + MCP endpoint
- subdomain (`{slug}.writekit.dev`) → `site` (rendered pages) + `api` (graph/embeddings) + user SPA
- custom domain → resolved through `platform.DB` to a tenant, served as site

**Data layers:**

- `internal/platform/` — Postgres. Users, tenants, sessions, OAuth linked accounts, team members, magic links, Stripe. Hosted only.
- `internal/tenant/` — per-tenant SQLite file in `DataDir`, managed by a WAL+LRU `Pool`. Holds pages, collections, page_versions, page_embeddings, settings. The only DB in desktop mode.
- Migrations are embedded SQL under each package's `migrations/` directory and run at startup.

**Events + cache:** `internal/events.Bus` is an in-process pub/sub. `site.Cache` subscribes to invalidate rendered pages on writes. Embeddings are computed client-side (`apps/user/src/embedding/`) against `/api/embedding-source`; no server-side embedding worker. Any mutation path must still publish the appropriate event or renders go stale.

**Rendering:** `internal/render` wraps Go templates with dev-reload. `internal/markdown` is goldmark + chroma + a D2 renderer + embed/callout extensions — changes to markdown extensions must stay in sync with the content guidelines the MCP server advertises to clients.

**Config:** all env vars declared in `internal/config/config.go`. `cfg.Local` is the desktop/single-tenant switch — check it before touching anything platform-specific.

## Conventions

- Desktop/local mode must not introduce a `platform` abstraction, LocalDB shim, or `PlatformStore` interface. Use context-injected identity.
- Dashboards are React/Vite SPAs (`apps/user`, `apps/admin`), not server-rendered templates. The marketing/auth/billing surface under `templates/web/` stays server-rendered.
- Keep MVP scope minimal — no features, fallbacks, or abstractions beyond what the task requires.
