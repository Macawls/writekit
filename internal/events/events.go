package events

import (
	"log/slog"
	"sync"
)

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
		go func(h Handler) {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("event handler panic", "type", e.Type, "tenant", e.TenantID, "page_id", e.PageID, "panic", r)
				}
			}()
			h(e)
		}(s.handler)
	}
}

const (
	PageCreated       = "page.created"
	PageUpdated       = "page.updated"
	PageContentSaved  = "page.content_saved"
	PageDeleted       = "page.deleted"
	PagePublished     = "page.published"
	CollectionCreated = "collection.created"
	CollectionUpdated = "collection.updated"
	CollectionDeleted = "collection.deleted"
	TenantRenamed     = "tenant.renamed"

	TeamInvitationCreated  = "team.invitation_created"
	TeamInvitationAccepted = "team.invitation_accepted"
)

type TenantRenamePayload struct {
	OldID string
	NewID string
}

type TeamInvitationCreatedPayload struct {
	InvitationID string
	Email        string
	Role         string
	Token        string
	TenantName   string
	InviterName  string
}

type TeamInvitationAcceptedPayload struct {
	InvitationID   string
	Email          string
	Role           string
	TenantName     string
	InviteeDisplay string
	InviterEmail   string
}
