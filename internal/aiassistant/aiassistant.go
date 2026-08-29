// Package aiassistant proxies the AI chat feature's Gemini API key draw to
// lumo-user-svc's own internal/aiassistant — pure passthrough, same shape as
// internal/likes.
package aiassistant

import (
	"context"
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

func (c *Client) GetApiKey(ctx context.Context, clerkID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/ai-assistant/api-key", nil, map[string]string{"X-Clerk-User-Id": clerkID})
}

// Handlers mirrors likes.Handlers — protected by clerkauth.Middleware
// ("/ai-assistant" is listed in cmd/server/main.go's clerkProtectedPaths).
type Handlers struct {
	client *Client
}

func NewHandlers(client *Client) *Handlers {
	return &Handlers{client: client}
}

func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /ai-assistant/api-key", h.getApiKey)
}

func (h *Handlers) getApiKey(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)

	res, err := h.client.GetApiKey(r.Context(), clerkID)
	if err != nil {
		log.Printf("lumo-user-svc call failed for GET /ai-assistant/api-key: %v", err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	// 503 (no active keys configured) passes through verbatim — the caller
	// needs that status, not a generic 502.
	if !res.OK() && res.StatusCode != http.StatusServiceUnavailable {
		log.Printf("lumo-user-svc call failed for GET /ai-assistant/api-key: status %d", res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	downstream.WriteProxy(w, res)
}
