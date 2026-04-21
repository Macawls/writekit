package site

import (
	"bytes"
	"context"
	"log/slog"

	"writekit/internal/config"
	"writekit/internal/events"
	"writekit/internal/render"
	"writekit/internal/tenant"
)

type PreRenderer struct {
	Pool   *tenant.Pool
	Engine *render.Engine
	Config *config.Config
	Bus    *events.Bus
}

func (p *PreRenderer) Start() {
	handler := func(e events.Event) {
		if e.PageID == "" || e.TenantID == "" {
			return
		}
		p.renderPage(e.TenantID, e.PageID)
	}
	p.Bus.On(events.PageCreated, handler)
	p.Bus.On(events.PageUpdated, handler)
	p.Bus.On(events.PagePublished, handler)
	p.Bus.On(events.PageContentSaved, handler)

	p.Bus.On(events.CollectionUpdated, func(e events.Event) {
		if e.TenantID == "" {
			return
		}
		p.renderCollectionPages(e.TenantID)
	})
}

func (p *PreRenderer) renderPage(tenantID, pageID string) {
	ctx := context.Background()
	db, err := p.Pool.Get(tenantID)
	if err != nil {
		slog.Warn("prerender: get tenant db", "tenant", tenantID, "err", err)
		return
	}
	page, err := db.GetPage(ctx, pageID)
	if err != nil {
		slog.Debug("prerender: get page", "tenant", tenantID, "page", pageID, "err", err)
		return
	}
	if !isCachable(page) {
		return
	}
	settings, _ := db.GetSettings(ctx)

	collectionSlug := ""
	if page.CollectionID != nil && *page.CollectionID != "" {
		if col, err := db.GetCollection(ctx, *page.CollectionID); err == nil {
			collectionSlug = col.Slug
		}
	}

	var buf bytes.Buffer
	if err := p.Engine.Render(&buf, "page.html", map[string]any{
		"Page":            page,
		"PageTitle":       page.Title,
		"PageDescription": page.Excerpt,
		"Settings":        settings,
		"TenantID":        tenantID,
		"Host":            p.Config.Host,
		"OGImageURL":      ogImageURL(tenantID, p.Config.Host, collectionSlug, page.Slug),
	}); err != nil {
		slog.Warn("prerender: render", "tenant", tenantID, "page", pageID, "err", err)
		return
	}
	if err := db.SetPageRender(ctx, pageID, buf.Bytes()); err != nil {
		slog.Warn("prerender: store", "tenant", tenantID, "page", pageID, "err", err)
	}
}

func (p *PreRenderer) renderCollectionPages(tenantID string) {
	ctx := context.Background()
	db, err := p.Pool.Get(tenantID)
	if err != nil {
		return
	}
	pages, err := db.ListPages(ctx, tenant.PageFilter{Status: "published"})
	if err != nil {
		return
	}
	for _, pg := range pages {
		if pg.CollectionID != nil && *pg.CollectionID != "" {
			p.renderPage(tenantID, pg.ID)
		}
	}
}

func isCachable(p *tenant.Page) bool {
	if p.Status != "published" {
		return false
	}
	return p.Visibility == "public" || p.Visibility == "unlisted"
}

func ogImageURL(tenantID, host, collectionSlug, pageSlug string) string {
	if collectionSlug != "" {
		return "https://" + tenantID + "." + host + "/og/" + collectionSlug + "/" + pageSlug + ".png"
	}
	return "https://" + tenantID + "." + host + "/og/" + pageSlug + ".png"
}
