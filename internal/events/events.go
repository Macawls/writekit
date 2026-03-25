package events

import "sync"

type Event struct {
	Type     string
	TenantID string
	Payload  any
}

type Handler func(Event)

type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
}

func NewBus() *Bus {
	return &Bus{handlers: make(map[string][]Handler)}
}

func (b *Bus) On(eventType string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], h)
}

func (b *Bus) Emit(e Event) {
	b.mu.RLock()
	handlers := b.handlers[e.Type]
	b.mu.RUnlock()
	for _, h := range handlers {
		go h(e)
	}
}

const (
	PageCreated       = "page.created"
	PageUpdated       = "page.updated"
	PageDeleted       = "page.deleted"
	PagePublished     = "page.published"
	CommentAdded      = "comment.added"
	CommentDeleted    = "comment.deleted"
	CollectionCreated = "collection.created"
	CollectionUpdated = "collection.updated"
	CollectionDeleted = "collection.deleted"
)
