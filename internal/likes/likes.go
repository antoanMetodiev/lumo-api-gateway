// Package likes mirrors internal/watchlist's exact shape — pure passthrough
// to lumo-user-svc's own internal/likes.
package likes

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

func (c *Client) GetLikes(ctx context.Context, clerkID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/likes", nil, map[string]string{"X-Clerk-User-Id": clerkID})
}

func (c *Client) AddLike(ctx context.Context, clerkID string, body []byte) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodPost, c.baseURL+"/likes", body, map[string]string{"X-Clerk-User-Id": clerkID})
}

func (c *Client) RemoveLike(ctx context.Context, clerkID, recordID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodDelete, c.baseURL+"/likes/"+url.PathEscape(recordID), nil, map[string]string{"X-Clerk-User-Id": clerkID})
}

// GetLikesForUser is friendship-gated downstream — see lumo-user-svc's
// getLikesForUser doc comment. The gateway just forwards callerID; it
// doesn't itself decide who can see what.
func (c *Client) GetLikesForUser(ctx context.Context, callerID, targetClerkID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/likes/user/"+url.PathEscape(targetClerkID), nil, map[string]string{"X-Clerk-User-Id": callerID})
}

// Handlers mirrors watchlist.Handlers — protected by clerkauth.Middleware
// ("/likes" is listed in cmd/server/main.go's clerkProtectedPaths).
type Handlers struct {
	client *Client
}

func NewHandlers(client *Client) *Handlers {
	return &Handlers{client: client}
}

func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /likes", h.getLikes)
	mux.HandleFunc("POST /likes", h.addLike)
	mux.HandleFunc("DELETE /likes/{recordId}", h.removeLike)
	mux.HandleFunc("GET /likes/user/{clerkId}", h.getLikesForUser)
}

func (h *Handlers) getLikes(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	res, err := h.client.GetLikes(r.Context(), clerkID)
	if err != nil {
		log.Printf("lumo-user-svc call failed for GET /likes: %v", err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if !res.OK() {
		log.Printf("lumo-user-svc call failed for GET /likes: status %d", res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	downstream.WriteProxy(w, res)
}

func (h *Handlers) addLike(w http.ResponseWriter, r *http.Request) {
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

	res, err := h.client.AddLike(r.Context(), clerkID, body)
	if err != nil {
		log.Printf("lumo-user-svc call failed for POST /likes: %v", err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	// 409 passes through verbatim — the caller hit the like-limit (see
	// lumo-user-svc's likes.Handlers.addLike) and needs that status/body,
	// not a generic 502.
	if !res.OK() && res.StatusCode != http.StatusConflict {
		log.Printf("lumo-user-svc call failed for POST /likes: status %d", res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	downstream.WriteProxy(w, res)
}

func (h *Handlers) removeLike(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	recordID := r.PathValue("recordId")

	res, err := h.client.RemoveLike(r.Context(), clerkID, recordID)
	if err != nil {
		log.Printf("lumo-user-svc call failed for DELETE /likes/%s: %v", recordID, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if !res.OK() {
		log.Printf("lumo-user-svc call failed for DELETE /likes/%s: status %d", recordID, res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) getLikesForUser(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	target := r.PathValue("clerkId")

	res, err := h.client.GetLikesForUser(r.Context(), clerkID, target)
	if err != nil {
		log.Printf("lumo-user-svc call failed for GET /likes/user/%s: %v", target, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if !res.OK() {
		log.Printf("lumo-user-svc call failed for GET /likes/user/%s: status %d", target, res.StatusCode)
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
