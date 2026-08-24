// Package watchlist mirrors WatchlistClient + WatchlistController. Request
// bodies are validated for their one required field, then forwarded to
// lumo-user-svc as the exact bytes the client sent — this gateway never
// needs to read or transform the other fields, so there's no separate DTO
// here to keep in sync with WatchlistItemRequest/WatchlistItemResponse.
package watchlist

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

func (c *Client) GetWatchlist(ctx context.Context, clerkID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/watchlist", nil, map[string]string{"X-Clerk-User-Id": clerkID})
}

func (c *Client) AddToWatchlist(ctx context.Context, clerkID string, body []byte) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodPost, c.baseURL+"/watchlist", body, map[string]string{"X-Clerk-User-Id": clerkID})
}

func (c *Client) RemoveFromWatchlist(ctx context.Context, clerkID, recordID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodDelete, c.baseURL+"/watchlist/"+url.PathEscape(recordID), nil, map[string]string{"X-Clerk-User-Id": clerkID})
}

// Handlers mirrors WatchlistController — protected by clerkauth.Middleware
// (SecurityConfig.PROTECTED_PATHS includes "/watchlist").
type Handlers struct {
	client *Client
}

func NewHandlers(client *Client) *Handlers {
	return &Handlers{client: client}
}

func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /watchlist", h.getWatchlist)
	mux.HandleFunc("POST /watchlist", h.addToWatchlist)
	mux.HandleFunc("DELETE /watchlist/{recordId}", h.removeFromWatchlist)
}

func (h *Handlers) getWatchlist(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	res, err := h.client.GetWatchlist(r.Context(), clerkID)
	if err != nil {
		log.Printf("lumo-user-svc call failed for GET /watchlist: %v", err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if !res.OK() {
		log.Printf("lumo-user-svc call failed for GET /watchlist: status %d", res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	downstream.WriteProxy(w, res)
}

func (h *Handlers) addToWatchlist(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var probe struct {
		RecordID string `json:"recordId"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if isBlank(probe.RecordID) {
		http.Error(w, "recordId: must not be blank", http.StatusBadRequest)
		return
	}

	res, err := h.client.AddToWatchlist(r.Context(), clerkID, body)
	if err != nil {
		log.Printf("lumo-user-svc call failed for POST /watchlist: %v", err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if !res.OK() {
		log.Printf("lumo-user-svc call failed for POST /watchlist: status %d", res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	downstream.WriteProxy(w, res)
}

func (h *Handlers) removeFromWatchlist(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	recordID := r.PathValue("recordId")

	res, err := h.client.RemoveFromWatchlist(r.Context(), clerkID, recordID)
	if err != nil {
		log.Printf("lumo-user-svc call failed for DELETE /watchlist/%s: %v", recordID, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if !res.OK() {
		log.Printf("lumo-user-svc call failed for DELETE /watchlist/%s: status %d", recordID, res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func isBlank(s string) bool {
	for _, r := range s {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
	}
	return true
}
