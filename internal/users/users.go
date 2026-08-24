// Package users mirrors UserClient + UserController + InternalUserController.
package users

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"

	"lumo-api-gateway/internal/clerkauth"
	"lumo-api-gateway/internal/downstream"
)

// UserResponse mirrors lumo-user-svc's User entity. Needed as a typed
// struct (unlike most of this gateway's DTOs) because internal/movies and
// internal/series read DisplayName/ProfileImageURL off it when
// posting/reacting to comments; CreatedAt/UpdatedAt stay raw JSON so a
// passthrough response re-marshals byte-identical to what lumo-user-svc sent.
type UserResponse struct {
	ClerkID          string          `json:"clerkId"`
	Email            string          `json:"email"`
	DisplayName      string          `json:"displayName"`
	ProfileImageURL  string          `json:"profileImageURL"`
	SubscriptionType string          `json:"subscriptionType"`
	CreatedAt        json.RawMessage `json:"createdAt"`
	UpdatedAt        json.RawMessage `json:"updatedAt"`
}

// ParseUserResponse decodes a 2xx GetCurrentUser result — call sites that
// only proxy the response straight through (UserController,
// InternalUserController) never need this.
func ParseUserResponse(res *downstream.Result) (*UserResponse, error) {
	var out UserResponse
	if err := downstream.Decode(res, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateSubscriptionRequest mirrors feather.internal.dtos.UpdateSubscriptionRequest.
type UpdateSubscriptionRequest struct {
	SubscriptionType string `json:"subscriptionType"`
}

type Client struct {
	ds      *downstream.Client
	baseURL string
}

func NewClient(ds *downstream.Client, baseURL string) *Client {
	return &Client{ds: ds, baseURL: baseURL}
}

// GetCurrentUser mirrors UserClient.getCurrentUser — the internal secret is
// attached automatically by downstream.Client.Do; only the already-verified
// Clerk user id needs to be passed explicitly.
func (c *Client) GetCurrentUser(ctx context.Context, clerkID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/user/me", nil, map[string]string{"X-Clerk-User-Id": clerkID})
}

// UpdateSubscription mirrors UserClient.updateSubscription — no
// X-Clerk-User-Id, this is a service-to-service call (see
// InternalServiceAuthenticationFilter equivalent, internal/internalauth).
func (c *Client) UpdateSubscription(ctx context.Context, clerkID string, req UpdateSubscriptionRequest) (*downstream.Result, error) {
	return c.ds.DoJSON(ctx, http.MethodPost, c.baseURL+"/internal/users/"+url.PathEscape(clerkID)+"/subscription", req, nil)
}

func (c *Client) Heartbeat(ctx context.Context) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/heartbeat", nil, nil)
}

// Handlers mirrors UserController + InternalUserController.
type Handlers struct {
	client *Client
}

func NewHandlers(client *Client) *Handlers {
	return &Handlers{client: client}
}

// Register wires every route onto mux.
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /user/me", h.getCurrentUser)
	mux.HandleFunc("POST /internal/users/{clerkId}/subscription", h.updateSubscription)
	mux.HandleFunc("GET /internal/users/{clerkId}", h.getUser)
}

// authentication.getName() is the verified Clerk user id, set by
// clerkauth.Middleware. 404 here means lumo-user-svc's Clerk webhook hasn't
// synced this account yet — near-instant after sign-up, but a real race the
// frontend should tolerate rather than treat as a hard failure.
func (h *Handlers) getCurrentUser(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	res, err := h.client.GetCurrentUser(r.Context(), clerkID)
	if err != nil {
		log.Printf("lumo-user-svc call failed for GET /user/me: %v", err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if res.StatusCode == http.StatusNotFound {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if !res.OK() {
		log.Printf("lumo-user-svc call failed for GET /user/me: status %d", res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	downstream.WriteProxy(w, res)
}

// POST, not PATCH: Feign's default HTTP client (and net/http here too, for
// consistency with the rest of this gateway) has no issue with PATCH, but
// this mirrors the Java route exactly since lumo-user-svc's own route is POST.
func (h *Handlers) updateSubscription(w http.ResponseWriter, r *http.Request) {
	clerkID := r.PathValue("clerkId")

	var req UpdateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if isBlank(req.SubscriptionType) {
		http.Error(w, "subscriptionType: must not be blank", http.StatusBadRequest)
		return
	}

	res, err := h.client.UpdateSubscription(r.Context(), clerkID, req)
	if err != nil {
		log.Printf("lumo-user-svc call failed for POST /internal/users/%s/subscription: %v", clerkID, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if res.StatusCode == http.StatusNotFound {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if !res.OK() {
		log.Printf("lumo-user-svc call failed for POST /internal/users/%s/subscription: status %d", clerkID, res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) getUser(w http.ResponseWriter, r *http.Request) {
	clerkID := r.PathValue("clerkId")
	res, err := h.client.GetCurrentUser(r.Context(), clerkID)
	if err != nil {
		log.Printf("lumo-user-svc call failed for GET /internal/users/%s: %v", clerkID, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if res.StatusCode == http.StatusNotFound {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if !res.OK() {
		log.Printf("lumo-user-svc call failed for GET /internal/users/%s: status %d", clerkID, res.StatusCode)
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
