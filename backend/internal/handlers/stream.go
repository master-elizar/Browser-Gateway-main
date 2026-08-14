package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/browser-gateway/backend/internal/domain"
	"github.com/browser-gateway/backend/internal/metrics"
	"github.com/browser-gateway/backend/internal/sessions"
	"github.com/browser-gateway/backend/internal/signaling"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin:       func(r *http.Request) bool { return true },
	EnableCompression: false,
}

// ServeWS dispatches session websocket endpoints.
func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 || parts[0] != "ws" || parts[1] != "sessions" {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	sessionID := parts[2]
	kind := parts[3]

	switch kind {
	case "vnc":
		user, ok := h.authorizeWS(w, r)
		if !ok {
			return
		}
		h.serveVNCProxy(w, r, user, sessionID)
	case "netmon":
		user, ok := h.authorizeWS(w, r)
		if !ok {
			return
		}
		h.serveNetmonWS(w, r, user, sessionID)
	case "signaling":
		h.serveSignalingWS(w, r, sessionID)
	case "control":
		user, ok := h.authorizeWS(w, r)
		if !ok {
			return
		}
		h.serveControlWS(w, r, user, sessionID)
	default:
		http.Error(w, "unknown ws endpoint", http.StatusNotFound)
	}
}

func (h *Handler) authorizeWS(w http.ResponseWriter, r *http.Request) (*domain.User, bool) {
	token := r.URL.Query().Get("token")
	if token == "" {
		authz := r.Header.Get("Authorization")
		if strings.HasPrefix(authz, "Bearer ") {
			token = strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
		}
	}
	if token == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return nil, false
	}
	claims, err := h.tokens.ParseAccess(token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return nil, false
	}
	user, err := h.auth.GetUser(claims.UserID)
	if err != nil || !user.Active {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	return user, true
}

func (h *Handler) serveVNCProxy(w http.ResponseWriter, r *http.Request, user *domain.User, sessionID string) {
	if err := h.requireViewerFeature(h.loadSettings().ViewerNoVNCEnabled, "novnc"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	ip, err := h.sessions.ResolveStreamTarget(r.Context(), user.ID, user.Role, sessionID)
	if err != nil {
		writeSessionHTTPErr(w, err)
		return
	}
	h.sessions.TouchActivity(sessionID)

	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		metrics.VNCProxyErrors.Inc()
		log.Printf("vnc upgrade: %v", err)
		return
	}
	defer clientConn.Close()
	metrics.VNCProxyConnections.Inc()

	target := "ws://" + ip + ":6080/"
	remote, _, err := websocket.DefaultDialer.DialContext(r.Context(), target, nil)
	if err != nil {
		metrics.VNCProxyErrors.Inc()
		log.Printf("vnc dial %s: %v", target, err)
		_ = clientConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "stream unavailable"))
		return
	}
	defer remote.Close()

	// The idle-timeout policy (backend/internal/workers/policy.go) only keeps a session
	// RUNNING while TouchActivity keeps landing -- before this, that only happened once,
	// at connect time, above. Real mouse/keyboard use never touched it again, so a session
	// under continuous active noVNC use would still silently go IDLE (and the frontend's
	// netmon WS then refuses to reconnect, since it requires status === RUNNING) purely on
	// wall-clock time since this handler was entered. Rate-limited (not per-frame, to avoid
	// a DB write per input event) touch on the client->remote direction only -- that's the
	// real input side; the remote->client direction is just screen updates and would touch
	// activity even while the user is doing nothing but watching.
	var lastTouch time.Time
	touchInput := func() {
		now := time.Now()
		if now.Sub(lastTouch) < 20*time.Second {
			return
		}
		lastTouch = now
		h.sessions.TouchActivity(sessionID)
	}

	errc := make(chan error, 2)
	go proxyWS(clientConn, remote, errc, nil)
	go proxyWS(remote, clientConn, errc, touchInput)
	<-errc
}

func (h *Handler) serveNetmonWS(w http.ResponseWriter, r *http.Request, user *domain.User, sessionID string) {
	if err := h.requireViewerFeature(h.loadSettings().ViewerNetworkEnabled, "network"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if _, err := h.sessions.Get(user, sessionID); err != nil {
		writeSessionHTTPErr(w, err)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("netmon upgrade: %v", err)
		return
	}
	defer conn.Close()

	ch := h.netmon.Subscribe(sessionID)
	defer h.netmon.Unsubscribe(sessionID, ch)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	_ = conn.WriteJSON(map[string]any{"type": "status", "state": "subscribed"})
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-done:
			return
		case <-r.Context().Done():
			return
		}
	}
}

func (h *Handler) serveSignalingWS(w http.ResponseWriter, r *http.Request, sessionID string) {
	role := r.URL.Query().Get("role")
	if role == "agent" {
		token := r.URL.Query().Get("token")
		if token == "" {
			token = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			token = strings.TrimSpace(token)
		}
		if _, err := h.sessions.AuthenticateAgent(sessionID, token); err != nil {
			writeSessionHTTPErr(w, err)
			return
		}
	} else {
		user, ok := h.authorizeWS(w, r)
		if !ok {
			return
		}
		if _, err := h.sessions.Get(user, sessionID); err != nil {
			writeSessionHTTPErr(w, err)
			return
		}
		role = "viewer"
		h.sessions.TouchActivity(sessionID)
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("signaling upgrade: %v", err)
		return
	}
	defer conn.Close()
	h.signal.Register(sessionID, role, conn)
	defer h.signal.Unregister(sessionID, role, conn)

	_ = signaling.RelayJSON(conn, map[string]any{"type": "ready", "role": role})

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		h.sessions.TouchActivity(sessionID)
		peer := h.signal.Peer(sessionID, role)
		if peer == nil {
			_ = signaling.RelayJSON(conn, map[string]any{"type": "peer-missing"})
			continue
		}
		var msg map[string]any
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		_ = peer.WriteMessage(websocket.TextMessage, data)
	}
}

func (h *Handler) serveControlWS(w http.ResponseWriter, r *http.Request, user *domain.User, sessionID string) {
	ip, err := h.sessions.ResolveStreamTarget(r.Context(), user.ID, user.Role, sessionID)
	if err != nil {
		writeSessionHTTPErr(w, err)
		return
	}
	h.sessions.TouchActivity(sessionID)

	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("control upgrade: %v", err)
		return
	}
	defer clientConn.Close()

	agentURL := "ws://" + ip + ":8091/"
	remote, _, err := websocket.DefaultDialer.DialContext(r.Context(), agentURL, nil)
	if err != nil {
		log.Printf("control dial %s: %v", agentURL, err)
		_ = clientConn.WriteJSON(map[string]any{"type": "error", "message": "control unavailable"})
		return
	}
	defer remote.Close()

	var settings domain.AppSettings
	_ = h.st.DB.First(&settings).Error

	errc := make(chan error, 2)
	go func() {
		for {
			_, data, err := clientConn.ReadMessage()
			if err != nil {
				errc <- err
				return
			}
			h.sessions.TouchActivity(sessionID)
			var msg map[string]any
			if json.Unmarshal(data, &msg) == nil {
				typ, _ := msg["type"].(string)
				if typ == "keydown" || typ == "keypress" {
					if settings.LogKeystrokes {
						key, _ := msg["key"].(string)
						h.sessions.AuditKeystroke(user.ID, sessionID, key)
					}
				} else if typ == "mousedown" || typ == "mouseup" || typ == "click" {
					h.sessions.AuditControl(user.ID, sessionID, typ)
				}
			}
			if err := remote.WriteMessage(websocket.TextMessage, data); err != nil {
				errc <- err
				return
			}
		}
	}()
	go func() {
		for {
			msgType, data, err := remote.ReadMessage()
			if err != nil {
				errc <- err
				return
			}
			if err := clientConn.WriteMessage(msgType, data); err != nil {
				errc <- err
				return
			}
		}
	}()
	<-errc
}

// onFrame, when non-nil, is called after each frame successfully forwarded in this
// direction -- used by serveVNCProxy to touch session activity on real input traffic
// without touching it on every direction of the proxy (see call site).
func proxyWS(dst, src *websocket.Conn, errc chan<- error, onFrame func()) {
	for {
		msgType, data, err := src.ReadMessage()
		if err != nil {
			errc <- err
			return
		}
		if err := dst.WriteMessage(msgType, data); err != nil {
			errc <- err
			return
		}
		if onFrame != nil {
			onFrame()
		}
	}
}

func writeSessionHTTPErr(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	switch err {
	case sessions.ErrNotFound:
		code = http.StatusNotFound
	case sessions.ErrForbidden:
		code = http.StatusForbidden
	case sessions.ErrInvalidState:
		code = http.StatusConflict
	}
	http.Error(w, err.Error(), code)
}
