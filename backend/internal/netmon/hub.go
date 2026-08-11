package netmon

import (
	"encoding/json"
	"sync"
)

// Hub fans out live network events to WebSocket subscribers per session.
type Hub struct {
	mu   sync.RWMutex
	subs map[string]map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{subs: make(map[string]map[chan []byte]struct{})}
}

func (h *Hub) Subscribe(sessionID string) chan []byte {
	ch := make(chan []byte, 64)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs[sessionID] == nil {
		h.subs[sessionID] = make(map[chan []byte]struct{})
	}
	h.subs[sessionID][ch] = struct{}{}
	return ch
}

func (h *Hub) Unsubscribe(sessionID string, ch chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m := h.subs[sessionID]; m != nil {
		delete(m, ch)
		if len(m) == 0 {
			delete(h.subs, sessionID)
		}
	}
	close(ch)
}

func (h *Hub) Publish(sessionID string, event any) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subs[sessionID] {
		select {
		case ch <- data:
		default:
			// drop if slow consumer
		}
	}
}
