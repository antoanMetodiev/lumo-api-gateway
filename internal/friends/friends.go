// Package friends mirrors lumo-user-svc's internal/friends routes 1:1 — same
// path strings, same pure-passthrough shape as internal/watchlist. Request
// bodies are validated for their one required field, then forwarded as the
// exact bytes the client sent.
package friends

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"

	"lumo-api-gateway/internal/clerkauth"
	"lumo-api-gateway/internal/downstream"
)

type Client struct {
	ds      *downstream.Client
	baseURL string
}

func NewClient(ds *downstream.Client, baseURL string) *Client {
	return &Client{ds: ds, baseURL: baseURL}
}

func (c *Client) ListFriends(ctx context.Context, clerkID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/friends", nil, map[string]string{"X-Clerk-User-Id": clerkID})
}

func (c *Client) Unfriend(ctx context.Context, clerkID, otherClerkID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodDelete, c.baseURL+"/friends/"+url.PathEscape(otherClerkID), nil, map[string]string{"X-Clerk-User-Id": clerkID})
}

func (c *Client) Search(ctx context.Context, clerkID, query string) (*downstream.Result, error) {
	q := url.Values{"query": {query}}
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/friends/search?"+q.Encode(), nil, map[string]string{"X-Clerk-User-Id": clerkID})
}

func (c *Client) ListIncoming(ctx context.Context, clerkID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/friends/requests/incoming", nil, map[string]string{"X-Clerk-User-Id": clerkID})
}

func (c *Client) ListOutgoing(ctx context.Context, clerkID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/friends/requests/outgoing", nil, map[string]string{"X-Clerk-User-Id": clerkID})
}

func (c *Client) SendRequest(ctx context.Context, clerkID string, body []byte) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodPost, c.baseURL+"/friends/requests", body, map[string]string{"X-Clerk-User-Id": clerkID})
}

func (c *Client) CancelOrDecline(ctx context.Context, clerkID, id string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodDelete, c.baseURL+"/friends/requests/"+url.PathEscape(id), nil, map[string]string{"X-Clerk-User-Id": clerkID})
}

func (c *Client) Accept(ctx context.Context, clerkID, id string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodPost, c.baseURL+"/friends/requests/"+url.PathEscape(id)+"/accept", nil, map[string]string{"X-Clerk-User-Id": clerkID})
}

func (c *Client) ConsumeNotifications(ctx context.Context, clerkID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/friends/notifications", nil, map[string]string{"X-Clerk-User-Id": clerkID})
}

func (c *Client) NotificationCount(ctx context.Context, clerkID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/friends/notifications/count", nil, map[string]string{"X-Clerk-User-Id": clerkID})
}

// Handlers mirrors FriendsController — protected by clerkauth.Middleware
// ("/friends" is listed in cmd/server/main.go's clerkProtectedPaths).
type Handlers struct {
	client *Client
}

func NewHandlers(client *Client) *Handlers {
	return &Handlers{client: client}
}

func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /friends", h.listFriends)
	mux.HandleFunc("DELETE /friends/{clerkId}", h.unfriend)
	mux.HandleFunc("GET /friends/search", h.search)
	mux.HandleFunc("GET /friends/requests/incoming", h.listIncoming)
	mux.HandleFunc("GET /friends/requests/outgoing", h.listOutgoing)
	mux.HandleFunc("POST /friends/requests", h.sendRequest)
	mux.HandleFunc("DELETE /friends/requests/{id}", h.cancelOrDecline)
	mux.HandleFunc("POST /friends/requests/{id}/accept", h.accept)
	mux.HandleFunc("GET /friends/notifications", h.consumeNotifications)
	mux.HandleFunc("GET /friends/notifications/count", h.notificationCount)
}

// proxyOrBadGateway is watchlist's proxy-or-502 shape, except it also passes
// through 400/404 verbatim — unlike watchlist, these backend endpoints emit
// meaningful validation/not-found bodies (self-request, unknown addressee,
// accept-target invalid) the frontend needs to show, not just "something
// went wrong upstream."
func proxyOrBadGateway(w http.ResponseWriter, label string, res *downstream.Result, err error) {
	if err != nil {
		log.Printf("lumo-user-svc call failed for %s: %v", label, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if !res.OK() && res.StatusCode != http.StatusNotFound && res.StatusCode != http.StatusBadRequest {
		log.Printf("lumo-user-svc call failed for %s: status %d", label, res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	downstream.WriteProxy(w, res)
}

func (h *Handlers) listFriends(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	res, err := h.client.ListFriends(r.Context(), clerkID)
	proxyOrBadGateway(w, "GET /friends", res, err)
}

func (h *Handlers) unfriend(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	other := r.PathValue("clerkId")

	res, err := h.client.Unfriend(r.Context(), clerkID, other)
	if err != nil {
		log.Printf("lumo-user-svc call failed for DELETE /friends/%s: %v", other, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if !res.OK() {
		log.Printf("lumo-user-svc call failed for DELETE /friends/%s: status %d", other, res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) search(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	res, err := h.client.Search(r.Context(), clerkID, r.URL.Query().Get("query"))
	proxyOrBadGateway(w, "GET /friends/search", res, err)
}

func (h *Handlers) listIncoming(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	res, err := h.client.ListIncoming(r.Context(), clerkID)
	proxyOrBadGateway(w, "GET /friends/requests/incoming", res, err)
}

func (h *Handlers) listOutgoing(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	res, err := h.client.ListOutgoing(r.Context(), clerkID)
	proxyOrBadGateway(w, "GET /friends/requests/outgoing", res, err)
}

func (h *Handlers) sendRequest(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var probe struct {
		AddresseeID string `json:"addresseeId"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if isBlank(probe.AddresseeID) {
		http.Error(w, "addresseeId: must not be blank", http.StatusBadRequest)
		return
	}

	res, err := h.client.SendRequest(r.Context(), clerkID, body)
	proxyOrBadGateway(w, "POST /friends/requests", res, err)
}

func (h *Handlers) cancelOrDecline(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	id := r.PathValue("id")

	res, err := h.client.CancelOrDecline(r.Context(), clerkID, id)
	if err != nil {
		log.Printf("lumo-user-svc call failed for DELETE /friends/requests/%s: %v", id, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if !res.OK() {
		log.Printf("lumo-user-svc call failed for DELETE /friends/requests/%s: status %d", id, res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) accept(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	id := r.PathValue("id")

	res, err := h.client.Accept(r.Context(), clerkID, id)
	proxyOrBadGateway(w, "POST /friends/requests/"+id+"/accept", res, err)
}

func (h *Handlers) consumeNotifications(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	res, err := h.client.ConsumeNotifications(r.Context(), clerkID)
	proxyOrBadGateway(w, "GET /friends/notifications", res, err)
}

func (h *Handlers) notificationCount(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	res, err := h.client.NotificationCount(r.Context(), clerkID)
	proxyOrBadGateway(w, "GET /friends/notifications/count", res, err)
}

func isBlank(s string) bool {
	for _, r := range s {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
	}
	return true
}
