// Package watching mirrors lumo-user-svc's internal/watching routes 1:1 —
// same pure-passthrough shape as internal/friends. No Java original to
// mirror (new feature, see lumo-user-svc's watching package doc comment).
package watching

import (
	"context"
	"io"
	"log"
	"net/http"

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

func (c *Client) Set(ctx context.Context, clerkID string, body []byte) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodPut, c.baseURL+"/watching", body, map[string]string{"X-Clerk-User-Id": clerkID})
}

func (c *Client) Clear(ctx context.Context, clerkID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodDelete, c.baseURL+"/watching", nil, map[string]string{"X-Clerk-User-Id": clerkID})
}

func (c *Client) ListFriendsWatching(ctx context.Context, clerkID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/watching/friends", nil, map[string]string{"X-Clerk-User-Id": clerkID})
}

// Handlers mirrors watching.Handlers — protected by clerkauth.Middleware
// ("/watching" is listed in cmd/server/main.go's clerkProtectedPaths).
type Handlers struct {
	client *Client
}

func NewHandlers(client *Client) *Handlers {
	return &Handlers{client: client}
}

func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("PUT /watching", h.set)
	mux.HandleFunc("DELETE /watching", h.clear)
	mux.HandleFunc("GET /watching/friends", h.listFriendsWatching)
}

// proxyOrBadGateway is friends' proxy-or-502 shape, also passing 400
// through verbatim — lumo-user-svc's PUT /watching emits a meaningful
// validation body (blank tmdbId/title, bad type) the frontend needs to see.
func proxyOrBadGateway(w http.ResponseWriter, label string, res *downstream.Result, err error) {
	if err != nil {
		log.Printf("lumo-user-svc call failed for %s: %v", label, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if !res.OK() && res.StatusCode != http.StatusBadRequest {
		log.Printf("lumo-user-svc call failed for %s: status %d", label, res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	downstream.WriteProxy(w, res)
}

func (h *Handlers) set(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	res, err := h.client.Set(r.Context(), clerkID, body)
	proxyOrBadGateway(w, "PUT /watching", res, err)
}

func (h *Handlers) clear(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)

	res, err := h.client.Clear(r.Context(), clerkID)
	if err != nil {
		log.Printf("lumo-user-svc call failed for DELETE /watching: %v", err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if !res.OK() {
		log.Printf("lumo-user-svc call failed for DELETE /watching: status %d", res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) listFriendsWatching(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	res, err := h.client.ListFriendsWatching(r.Context(), clerkID)
	proxyOrBadGateway(w, "GET /watching/friends", res, err)
}
