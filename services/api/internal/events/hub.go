package events

import (
	"sync"

	"github.com/steloit/cloud/services/api/internal/identity/store"
)

// Hub is the in-process SSE fanout (single api replica in MVP; cross-replica
// fanout is a later task behind the same Subscribe/Publish shape). Publish
// never blocks: a subscriber whose buffer is full is dropped — a reconnect
// with its cursor loses nothing, the ledger is the truth.
type Hub struct {
	mu   sync.Mutex
	subs map[string]map[chan store.Event]struct{} // orgID → subscribers
}

const subscriberBuffer = 64

func NewHub() *Hub {
	return &Hub{subs: map[string]map[chan store.Event]struct{}{}}
}

// Subscribe registers a live listener for one org. cancel is idempotent and
// closes the channel so ranging consumers terminate.
func (h *Hub) Subscribe(orgID string) (ch <-chan store.Event, cancel func()) {
	c := make(chan store.Event, subscriberBuffer)
	h.mu.Lock()
	if h.subs[orgID] == nil {
		h.subs[orgID] = map[chan store.Event]struct{}{}
	}
	h.subs[orgID][c] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	return c, func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.subs[orgID], c)
			if len(h.subs[orgID]) == 0 {
				delete(h.subs, orgID)
			}
			h.mu.Unlock()
			close(c)
		})
	}
}

// Publish fans an event out to the org's live subscribers, dropping any whose
// buffer is full (slow consumer must never block the append path).
func (h *Hub) Publish(evt store.Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.subs[evt.OrgID] {
		select {
		case c <- evt:
		default: // full buffer: drop; the subscriber's next reconnect replays from its cursor
		}
	}
}
