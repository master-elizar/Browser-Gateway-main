package signaling

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
)

// Hub relays SDP/ICE JSON between viewer and agent peers per session.
type Hub struct {
	mu    sync.Mutex
	rooms map[string]*room
}

type room struct {
	viewer *websocket.Conn
	agent  *websocket.Conn
}

func NewHub() *Hub {
	return &Hub{rooms: make(map[string]*room)}
}

func (h *Hub) get(sessionID string) *room {
	h.mu.Lock()
	defer h.mu.Unlock()
	r := h.rooms[sessionID]
	if r == nil {
		r = &room{}
		h.rooms[sessionID] = r
	}
	return r
}

func (h *Hub) Register(sessionID, role string, conn *websocket.Conn) {
	r := h.get(sessionID)
	h.mu.Lock()
	defer h.mu.Unlock()
	if role == "agent" {
		if r.agent != nil {
			_ = r.agent.Close()
		}
		r.agent = conn
	} else {
		if r.viewer != nil {
			_ = r.viewer.Close()
		}
		r.viewer = conn
	}
}

func (h *Hub) Unregister(sessionID, role string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	r := h.rooms[sessionID]
	if r == nil {
		return
	}
	if role == "agent" && r.agent == conn {
		r.agent = nil
	}
	if role != "agent" && r.viewer == conn {
		r.viewer = nil
	}
	if r.agent == nil && r.viewer == nil {
		delete(h.rooms, sessionID)
	}
}

func (h *Hub) Peer(sessionID, role string) *websocket.Conn {
	h.mu.Lock()
	defer h.mu.Unlock()
	r := h.rooms[sessionID]
	if r == nil {
		return nil
	}
	if role == "agent" {
		return r.viewer
	}
	return r.agent
}

func RelayJSON(dst *websocket.Conn, payload any) error {
	if dst == nil {
		return nil
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return dst.WriteMessage(websocket.TextMessage, b)
}
