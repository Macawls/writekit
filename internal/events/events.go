package events

import "sync"

type Event struct {
	Type     string
	TenantID string
	PageID   string
	Payload  any
}

type Handler func(Event)

type subscription struct {
	id      int
	handler Handler
}

type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]subscription
	nextID   int
}

func NewBus() *Bus {
	return &Bus{handlers: make(map[string][]subscription)}
}

func (b *Bus) On(eventType string, h Handler) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	b.handlers[eventType] = append(b.handlers[eventType], subscription{id: b.nextID, handler: h})
	return b.nextID
}

func (b *Bus) Off(eventType string, id int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	subs := b.handlers[eventType]
	for i, s := range subs {
		if s.id == id {
			b.handlers[eventType] = append(subs[:i], subs[i+1:]...)
			return
		}
	}
}

func (b *Bus) Emit(e Event) {
	b.mu.RLock()
	subs := b.handlers[e.Type]
	b.mu.RUnlock()
	for _, s := range subs {
		go s.handler(e)
	}
}

const (
	PageCreated       = "page.created"
	PageUpdated       = "page.updated"
	PageDeleted       = "page.deleted"
	PagePublished     = "page.published"
CollectionCreated = "collection.created"
	CollectionUpdated = "collection.updated"
	CollectionDeleted = "collection.deleted"
)
