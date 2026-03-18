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
	PostCreated   = "post.created"
	PostUpdated   = "post.updated"
	PostDeleted   = "post.deleted"
	PostPublished = "post.published"
	CommentAdded  = "comment.added"
	CommentDelete = "comment.deleted"
)
