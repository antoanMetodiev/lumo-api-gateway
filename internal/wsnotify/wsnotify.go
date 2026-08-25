// Package wsnotify holds live WebSocket connections for signed-in users and
// pushes best-effort real-time events to them — currently just "you got a
// friend request" (see lumo-user-svc's friends.NotifyClient, the only
// caller of the internal notify endpoint below).
//
// Connections live in an in-memory map, so this only works correctly with a
// single gateway instance — a deliberate, temporary choice: once there's
// more than one instance to fan a push out across, this needs to move to a
// shared store (Redis pub/sub or similar) instead of process memory.
package wsnotify

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"lumo-api-gateway/internal/clerkauth"
)

// pingInterval keeps idle connections alive — many hosting platforms and
// intermediary proxies (Render included) silently drop a WebSocket that's
// gone quiet for too long, which otherwise only gets noticed on the next
// notify() write attempt (by then the recipient has already missed the
// push). A server-initiated ping keeps traffic flowing on connections that
// might otherwise sit idle for minutes between real events.
const pingInterval = 20 * time.Second

// allowedOrigins are the frontend origins permitted to open a WebSocket here
// — the browser WebSocket handshake can't be routed through my-app's own
// same-origin API proxy the way every other authenticated call in this app
// is (see app/api/backend's doc comment), so this is a genuine
// browser-to-gateway cross-origin connection and needs its own allowlist
// (coder/websocket's OriginPatterns), independent of internal/cors's
// currently-stale REST allowlist.
var allowedOrigins = []string{"localhost:3000", "auroraflix.cloud"}

type Hub struct {
	verifier *clerkauth.Verifier

	mu    sync.Mutex
	conns map[string]*websocket.Conn // clerkID -> live connection; a new connection replaces any old one for that id
}

func NewHub(verifier *clerkauth.Verifier) *Hub {
	return &Hub{verifier: verifier, conns: map[string]*websocket.Conn{}}
}

func (h *Hub) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /ws", h.serveWS)
	// "/internal/notify/{clerkId}" already matches internalauth's "/internal"
	// prefix (see cmd/server/main.go) — no separate path-list edit needed.
	mux.HandleFunc("POST /internal/notify/{clerkId}", h.notify)
}

// serveWS upgrades the connection after verifying the caller's Clerk token,
// passed as a query param (?token=...) since the browser WebSocket API
// can't set custom headers on the handshake request, unlike every other
// authenticated call in this app (which sends `Authorization: Bearer ...`).
func (h *Hub) serveWS(w http.ResponseWriter, r *http.Request) {
	clerkID := h.verifier.VerifyAndGetUserID(r.URL.Query().Get("token"))
	if clerkID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: allowedOrigins})
	if err != nil {
		return
	}

	h.mu.Lock()
	if old, ok := h.conns[clerkID]; ok {
		old.Close(websocket.StatusNormalClosure, "replaced by a new connection")
	}
	h.conns[clerkID] = conn
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		if h.conns[clerkID] == conn {
			delete(h.conns, clerkID)
		}
		h.mu.Unlock()
		conn.CloseNow()
	}()

	ctx := r.Context()

	pingCtx, stopPing := context.WithCancel(ctx)
	defer stopPing()
	go keepAlive(pingCtx, conn)

	// This connection has nothing else to read from the client — Read just
	// blocks until the socket closes (or errors, including a failed ping
	// below closing it), which is what keeps this handler (and its deferred
	// cleanup) alive for as long as the connection is open.
	for {
		if _, _, err := conn.Read(ctx); err != nil {
			return
		}
	}
}

// keepAlive pings on a fixed interval until ctx is done or a ping fails —
// coder/websocket's Conn is safe for one concurrent Read plus concurrent
// Writes/Pings from another goroutine, which is exactly this split (serveWS
// owns the Read loop, this owns the pings).
func keepAlive(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := conn.Ping(pingCtx)
			cancel()
			if err != nil {
				conn.CloseNow()
				return
			}
		}
	}
}

// notify is best-effort: whether the addressee is currently connected or
// not, the caller (lumo-user-svc) always gets a 200 — a missed push just
// means the recipient sees the event on their next poll/page load instead
// of instantly, not a failure worth surfacing or retrying.
func (h *Hub) notify(w http.ResponseWriter, r *http.Request) {
	clerkID := r.PathValue("clerkId")

	var payload json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	conn, ok := h.conns[clerkID]
	h.mu.Unlock()

	if ok {
		if err := wsjson.Write(r.Context(), conn, payload); err != nil {
			log.Printf("wsnotify: failed to push to clerkId=%s: %v", clerkID, err)
		}
	}

	w.WriteHeader(http.StatusOK)
}
