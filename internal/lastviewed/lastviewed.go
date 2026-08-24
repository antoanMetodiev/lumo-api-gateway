// Package lastviewed mirrors LastViewedClient + LastViewedController. Same
// raw-passthrough-with-partial-validation approach as internal/watchlist —
// see that package's doc comment.
package lastviewed

import (
	"context"
	"encoding/json"
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

func (c *Client) GetLastViewed(ctx context.Context, clerkID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/last-viewed", nil, map[string]string{"X-Clerk-User-Id": clerkID})
}

func (c *Client) RecordView(ctx context.Context, clerkID string, body []byte) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodPost, c.baseURL+"/last-viewed", body, map[string]string{"X-Clerk-User-Id": clerkID})
}

// Handlers mirrors LastViewedController — protected by clerkauth.Middleware
// (SecurityConfig.PROTECTED_PATHS includes "/last-viewed").
type Handlers struct {
	client *Client
}

func NewHandlers(client *Client) *Handlers {
	return &Handlers{client: client}
}

func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /last-viewed", h.getLastViewed)
	mux.HandleFunc("POST /last-viewed", h.recordView)
}

func (h *Handlers) getLastViewed(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	res, err := h.client.GetLastViewed(r.Context(), clerkID)
	if err != nil {
		log.Printf("lumo-user-svc call failed for GET /last-viewed: %v", err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if !res.OK() {
		log.Printf("lumo-user-svc call failed for GET /last-viewed: status %d", res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	downstream.WriteProxy(w, res)
}

func (h *Handlers) recordView(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var probe struct {
		Type  string `json:"type"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if isBlank(probe.Type) {
		http.Error(w, "type: must not be blank", http.StatusBadRequest)
		return
	}
	if isBlank(probe.Title) {
		http.Error(w, "title: must not be blank", http.StatusBadRequest)
		return
	}

	res, err := h.client.RecordView(r.Context(), clerkID, body)
	if err != nil {
		log.Printf("lumo-user-svc call failed for POST /last-viewed: %v", err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if !res.OK() {
		log.Printf("lumo-user-svc call failed for POST /last-viewed: status %d", res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	downstream.WriteProxy(w, res)
}

func isBlank(s string) bool {
	for _, r := range s {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
	}
	return true
}
