// Package payments mirrors PaymentsClient + PaymentsController +
// InternalSubscriptionController.
package payments

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

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

func (c *Client) CreateCheckoutSession(ctx context.Context, clerkID string, body []byte) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodPost, c.baseURL+"/payments/checkout-session", body, map[string]string{"X-Clerk-User-Id": clerkID})
}

func (c *Client) CreatePortalSession(ctx context.Context, clerkID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodPost, c.baseURL+"/payments/portal-session", nil, map[string]string{"X-Clerk-User-Id": clerkID})
}

func (c *Client) GetSubscription(ctx context.Context, clerkID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/payments/subscription", nil, map[string]string{"X-Clerk-User-Id": clerkID})
}

func (c *Client) GetCheckoutResult(ctx context.Context, clerkID, sessionID string) (*downstream.Result, error) {
	u := c.baseURL + "/payments/checkout-result?sessionId=" + url.QueryEscape(sessionID)
	return c.ds.Do(ctx, http.MethodGet, u, nil, map[string]string{"X-Clerk-User-Id": clerkID})
}

// DeleteSubscription mirrors PaymentsClient.deleteSubscription — no
// X-Clerk-User-Id, service-to-service (see InternalSubscriptionController).
func (c *Client) DeleteSubscription(ctx context.Context, clerkID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodDelete, c.baseURL+"/internal/subscriptions/"+url.PathEscape(clerkID), nil, nil)
}

func (c *Client) Heartbeat(ctx context.Context) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/heartbeat", nil, nil)
}

// Handlers mirrors PaymentsController (protected by clerkauth.Middleware —
// SecurityConfig.PROTECTED_PATHS includes "/payments") and
// InternalSubscriptionController (protected by the internal-secret
// middleware instead — SecurityConfig.INTERNAL_PATHS includes "/internal").
type Handlers struct {
	client *Client
}

func NewHandlers(client *Client) *Handlers {
	return &Handlers{client: client}
}

func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /payments/checkout-session", h.createCheckoutSession)
	mux.HandleFunc("POST /payments/portal-session", h.createPortalSession)
	mux.HandleFunc("GET /payments/subscription", h.getSubscription)
	mux.HandleFunc("GET /payments/checkout-result", h.getCheckoutResult)
	mux.HandleFunc("DELETE /internal/subscriptions/{clerkId}", h.deleteSubscription)
}

func (h *Handlers) createCheckoutSession(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var probe struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if isBlank(probe.Email) {
		http.Error(w, "email: must not be blank", http.StatusBadRequest)
		return
	}
	if !looksLikeEmail(probe.Email) {
		http.Error(w, "email: must be a well-formed email address", http.StatusBadRequest)
		return
	}

	res, err := h.client.CreateCheckoutSession(r.Context(), clerkID, body)
	if err != nil {
		log.Printf("lumo-payments-svc call failed for POST /payments/checkout-session: %v", err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if !res.OK() {
		log.Printf("lumo-payments-svc call failed for POST /payments/checkout-session: status %d", res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	downstream.WriteProxy(w, res)
}

func (h *Handlers) createPortalSession(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	res, err := h.client.CreatePortalSession(r.Context(), clerkID)
	if err != nil {
		log.Printf("lumo-payments-svc call failed for POST /payments/portal-session: %v", err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if res.StatusCode == http.StatusNotFound {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if !res.OK() {
		log.Printf("lumo-payments-svc call failed for POST /payments/portal-session: status %d", res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	downstream.WriteProxy(w, res)
}

func (h *Handlers) getSubscription(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	res, err := h.client.GetSubscription(r.Context(), clerkID)
	if err != nil {
		log.Printf("lumo-payments-svc call failed for GET /payments/subscription: %v", err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if !res.OK() {
		log.Printf("lumo-payments-svc call failed for GET /payments/subscription: status %d", res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	downstream.WriteProxy(w, res)
}

func (h *Handlers) getCheckoutResult(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)
	sessionID := r.URL.Query().Get("sessionId")

	res, err := h.client.GetCheckoutResult(r.Context(), clerkID, sessionID)
	if err != nil {
		log.Printf("lumo-payments-svc call failed for GET /payments/checkout-result: %v", err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if res.StatusCode == http.StatusNotFound {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if !res.OK() {
		log.Printf("lumo-payments-svc call failed for GET /payments/checkout-result: status %d", res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	downstream.WriteProxy(w, res)
}

func (h *Handlers) deleteSubscription(w http.ResponseWriter, r *http.Request) {
	clerkID := r.PathValue("clerkId")
	res, err := h.client.DeleteSubscription(r.Context(), clerkID)
	if err != nil {
		log.Printf("lumo-payments-svc call failed for DELETE /internal/subscriptions/%s: %v", clerkID, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if !res.OK() {
		log.Printf("lumo-payments-svc call failed for DELETE /internal/subscriptions/%s: status %d", clerkID, res.StatusCode)
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

// looksLikeEmail is a light stand-in for Jakarta's @Email — not a full RFC
// validator, just enough to reject obviously-malformed input the same way
// @Email would (real validation/dedup happens at Stripe/lumo-payments-svc
// regardless).
func looksLikeEmail(s string) bool {
	at := strings.IndexByte(s, '@')
	if at <= 0 || at == len(s)-1 {
		return false
	}
	domain := s[at+1:]
	return strings.Contains(domain, ".") && !strings.ContainsAny(s, " \t")
}
