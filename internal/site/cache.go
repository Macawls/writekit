package site

import (
	"sync"

	"writekit/internal/events"
)

type Cache struct {
	mu          sync.RWMutex
	invalidated map[string]bool
}

func NewCache(bus *events.Bus) *Cache {
	c := &Cache{invalidated: make(map[string]bool)}

	for _, eventType := range []string{
		events.PageCreated, events.PageUpdated, events.PageDeleted,
		events.PagePublished,
		events.CollectionCreated, events.CollectionUpdated, events.CollectionDeleted,
	} {
		bus.On(eventType, func(e events.Event) {
			c.Invalidate(e.TenantID)
		})
	}

	bus.On(events.TenantRenamed, func(e events.Event) {
		p, ok := e.Payload.(events.TenantRenamePayload)
		if !ok {
			return
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		delete(c.invalidated, p.OldID)
		c.invalidated[p.NewID] = true
	})

	return c
}

func (c *Cache) Invalidate(tenantID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.invalidated[tenantID] = true
}

func (c *Cache) IsValid(tenantID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return !c.invalidated[tenantID]
}

func (c *Cache) MarkValid(tenantID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.invalidated, tenantID)
}
