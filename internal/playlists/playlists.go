// Package playlists mirrors lumo-user-svc's own internal/playlists routes
// 1:1 — same path strings, same pure-passthrough shape as internal/friends.
// Request bodies are validated for their one required field, then forwarded
// as the exact bytes the client sent.
package playlists

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

func (c *Client) ListMine(ctx context.Context, clerkID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/playlists", nil, map[string]string{"X-Clerk-User-Id": clerkID})
}

func (c *Client) Create(ctx context.Context, clerkID string, body []byte) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodPost, c.baseURL+"/playlists", body, map[string]string{"X-Clerk-User-Id": clerkID})
}

func (c *Client) Update(ctx context.Context, clerkID, id string, body []byte) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodPatch, c.baseURL+"/playlists/"+url.PathEscape(id), body, map[string]string{"X-Clerk-User-Id": clerkID})
}

func (c *Client) Delete(ctx context.Context, clerkID, id string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodDelete, c.baseURL+"/playlists/"+url.PathEscape(id), nil, map[string]string{"X-Clerk-User-Id": clerkID})
}

func (c *Client) ListItems(ctx context.Context, clerkID, id string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/playlists/"+url.PathEscape(id)+"/items", nil, map[string]string{"X-Clerk-User-Id": clerkID})
}

func (c *Client) AddItem(ctx context.Context, clerkID, id string, body []byte) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodPost, c.baseURL+"/playlists/"+url.PathEscape(id)+"/items", body, map[string]string{"X-Clerk-User-Id": clerkID})
}

func (c *Client) RemoveItem(ctx context.Context, clerkID, id, recordID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodDelete, c.baseURL+"/playlists/"+url.PathEscape(id)+"/items/"+url.PathEscape(recordID), nil, map[string]string{"X-Clerk-User-Id": clerkID})
}

// ListForUser is deliberately public among any signed-in caller — see
// lumo-user-svc's listForUser doc comment; the friendship/public gating
// happens downstream, not here.
func (c *Client) ListForUser(ctx context.Context, callerID, targetClerkID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/playlists/"+url.PathEscape(targetClerkID)+"/visible", nil, map[string]string{"X-Clerk-User-Id": callerID})
}

// Handlers mirrors friends.Handlers — protected by clerkauth.Middleware
// ("/playlists" is listed in cmd/server/main.go's clerkProtectedPaths).
type Handlers struct {
	client *Client
}

func NewHandlers(client *Client) *Handlers {
	return &Handlers{client: client}
}

func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /playlists", h.listMine)
	mux.HandleFunc("POST /playlists", h.create)
	mux.HandleFunc("PATCH /playlists/{id}", h.update)
	mux.HandleFunc("DELETE /playlists/{id}", h.delete)
	mux.HandleFunc("GET /playlists/{id}/items", h.listItems)
	mux.HandleFunc("POST /playlists/{id}/items", h.addItem)
	mux.HandleFunc("DELETE /playlists/{id}/items/{recordId}", h.removeItem)
	// See lumo-user-svc's handlers.go Register doc comment — not
	// "/playlists/user/{clerkId}", net/http's ServeMux can't disambiguate
	// that from "/playlists/{id}/items".
	mux.HandleFunc("GET /playlists/{clerkId}/visible", h.listForUser)
}

// proxyOrBadGateway mirrors friends' own copy — passes 400/404/409 through
// verbatim (these backend endpoints emit meaningful validation/not-found/
// at-capacity bodies), 502 for anything else that isn't a clean 2xx.
func proxyOrBadGateway(w http.ResponseWriter, label string, res *downstream.Result, err error) {
	if err != nil {
		log.Printf("lumo-user-svc call failed for %s: %v", label, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if !res.OK() && res.StatusCode != http.StatusNotFound && res.StatusCode != http.StatusBadRequest && res.StatusCode != http.StatusConflict {
		log.Printf("lumo-user-svc call failed for %s: status %d", label, res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	downstream.WriteProxy(w, res)
}

func (h *Handlers) listMine(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	res, err := h.client.ListMine(r.Context(), clerkID)
	proxyOrBadGateway(w, "GET /playlists", res, err)
}

func (h *Handlers) create(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var probe struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if isBlank(probe.Name) {
		http.Error(w, "name: must not be blank", http.StatusBadRequest)
		return
	}

	res, err := h.client.Create(r.Context(), clerkID, body)
	proxyOrBadGateway(w, "POST /playlists", res, err)
}

func (h *Handlers) update(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	id := r.PathValue("id")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var probe struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if isBlank(probe.Name) {
		http.Error(w, "name: must not be blank", http.StatusBadRequest)
		return
	}

	res, err := h.client.Update(r.Context(), clerkID, id, body)
	proxyOrBadGateway(w, "PATCH /playlists/"+id, res, err)
}

func (h *Handlers) delete(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	id := r.PathValue("id")

	res, err := h.client.Delete(r.Context(), clerkID, id)
	proxyOrBadGateway(w, "DELETE /playlists/"+id, res, err)
}

func (h *Handlers) listItems(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	id := r.PathValue("id")

	res, err := h.client.ListItems(r.Context(), clerkID, id)
	proxyOrBadGateway(w, "GET /playlists/"+id+"/items", res, err)
}

func (h *Handlers) addItem(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	id := r.PathValue("id")

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

	res, err := h.client.AddItem(r.Context(), clerkID, id, body)
	proxyOrBadGateway(w, "POST /playlists/"+id+"/items", res, err)
}

func (h *Handlers) removeItem(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	id := r.PathValue("id")
	recordID := r.PathValue("recordId")

	res, err := h.client.RemoveItem(r.Context(), clerkID, id, recordID)
	proxyOrBadGateway(w, "DELETE /playlists/"+id+"/items/"+recordID, res, err)
}

func (h *Handlers) listForUser(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	target := r.PathValue("clerkId")

	res, err := h.client.ListForUser(r.Context(), clerkID, target)
	proxyOrBadGateway(w, "GET /playlists/user/"+target, res, err)
}

func isBlank(s string) bool {
	for _, r := range s {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
	}
	return true
}
