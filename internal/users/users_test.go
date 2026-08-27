package users

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"lumo-api-gateway/internal/downstream"
)

func newTestClient(baseURL string) *Client {
	return NewClient(downstream.New("secret"), baseURL)
}

func newTestMux(baseURL string) *http.ServeMux {
	mux := http.NewServeMux()
	NewHandlers(newTestClient(baseURL)).Register(mux)
	return mux
}

// --- Client ---

func TestClientGetCurrentUserHitsUserMeWithClerkHeader(t *testing.T) {
	var gotMethod, gotPath, gotClerkID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotClerkID = r.Header.Get("X-Clerk-User-Id")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if _, err := c.GetCurrentUser(context.Background(), "user_123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/user/me" {
		t.Errorf("path = %q, want /user/me", gotPath)
	}
	if gotClerkID != "user_123" {
		t.Errorf("X-Clerk-User-Id = %q, want user_123", gotClerkID)
	}
}

func TestClientUpdateSubscriptionPostsJSONWithoutClerkHeaderAndEscapesPath(t *testing.T) {
	var gotMethod, gotRequestURI string
	var gotBody []byte
	var sawClerkHeader bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotRequestURI = r.RequestURI
		_, sawClerkHeader = r.Header["X-Clerk-User-Id"]
		body := make([]byte, r.ContentLength)
		r.Body.Read(body)
		gotBody = body
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	// A clerk id with a slash proves url.PathEscape actually runs, rather
	// than just working by coincidence on a plain alphanumeric id.
	req := UpdateSubscriptionRequest{SubscriptionType: "PREMIUM"}
	if _, err := c.UpdateSubscription(context.Background(), "user/1", req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotRequestURI != "/internal/users/user%2F1/subscription" {
		t.Errorf("RequestURI = %q, want /internal/users/user%%2F1/subscription", gotRequestURI)
	}
	if string(gotBody) != `{"subscriptionType":"PREMIUM"}` {
		t.Errorf("body = %q", gotBody)
	}
	if sawClerkHeader {
		t.Error("expected no X-Clerk-User-Id header on this service-to-service call")
	}
}

func TestClientHeartbeatHitsHeartbeatPath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if _, err := c.Heartbeat(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/heartbeat" {
		t.Errorf("path = %q, want /heartbeat", gotPath)
	}
}

// --- ParseUserResponse ---

// TestParseUserResponseKeepsTimestampsAsRawJSON checks the documented
// byte-identical re-marshaling requirement: createdAt/updatedAt must survive
// as json.RawMessage, not get parsed (and potentially reformatted or
// precision-truncated) into time.Time.
func TestParseUserResponseKeepsTimestampsAsRawJSON(t *testing.T) {
	rawCreatedAt := `"2024-01-01T00:00:00.123456789Z"`
	rawUpdatedAt := `1700000000`
	body := `{"clerkId":"user_1","email":"a@example.com","displayName":"Ada","profileImageURL":"http://img/x.png","subscriptionType":"PREMIUM","createdAt":` + rawCreatedAt + `,"updatedAt":` + rawUpdatedAt + `}`

	out, err := ParseUserResponse(&downstream.Result{Body: []byte(body)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ClerkID != "user_1" || out.Email != "a@example.com" || out.DisplayName != "Ada" ||
		out.ProfileImageURL != "http://img/x.png" || out.SubscriptionType != "PREMIUM" {
		t.Errorf("decoded fields = %+v", out)
	}
	if string(out.CreatedAt) != rawCreatedAt {
		t.Errorf("CreatedAt = %s, want raw %s", out.CreatedAt, rawCreatedAt)
	}
	if string(out.UpdatedAt) != rawUpdatedAt {
		t.Errorf("UpdatedAt = %s, want raw %s", out.UpdatedAt, rawUpdatedAt)
	}

	// Re-marshaling the whole struct must reproduce the exact same raw bytes
	// for the timestamp fields.
	remarshaled, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("remarshal error: %v", err)
	}
	var roundTrip map[string]json.RawMessage
	if err := json.Unmarshal(remarshaled, &roundTrip); err != nil {
		t.Fatalf("unmarshal remarshaled: %v", err)
	}
	if string(roundTrip["createdAt"]) != rawCreatedAt {
		t.Errorf("remarshaled createdAt = %s, want %s", roundTrip["createdAt"], rawCreatedAt)
	}
	if string(roundTrip["updatedAt"]) != rawUpdatedAt {
		t.Errorf("remarshaled updatedAt = %s, want %s", roundTrip["updatedAt"], rawUpdatedAt)
	}
}

func TestParseUserResponseInvalidJSONReturnsError(t *testing.T) {
	if _, err := ParseUserResponse(&downstream.Result{Body: []byte(`not json`)}); err == nil {
		t.Fatal("expected an error for invalid JSON, got nil")
	}
}

// --- Handlers.getCurrentUser ---

func TestHandlersGetCurrentUserProxiesSuccessVerbatim(t *testing.T) {
	body := `{"clerkId":"user_1","email":"a@example.com"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))
	defer srv.Close()

	mux := newTestMux(srv.URL)
	req := httptest.NewRequest(http.MethodGet, "/user/me", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != body {
		t.Errorf("body = %q, want %q", rec.Body.String(), body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestHandlersGetCurrentUserReturns404WhenDownstream404s(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	mux := newTestMux(srv.URL)
	req := httptest.NewRequest(http.MethodGet, "/user/me", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestHandlersGetCurrentUserReturns502OnDownstreamServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	mux := newTestMux(srv.URL)
	req := httptest.NewRequest(http.MethodGet, "/user/me", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec.Code)
	}
}

func TestHandlersGetCurrentUserReturns502OnTransportError(t *testing.T) {
	mux := newTestMux("http://127.0.0.1:1")
	req := httptest.NewRequest(http.MethodGet, "/user/me", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec.Code)
	}
}

// --- Handlers.updateSubscription ---

func TestHandlersUpdateSubscriptionMalformedJSONReturns400(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("downstream should not be called when the request body is malformed")
	}))
	defer srv.Close()

	mux := newTestMux(srv.URL)
	req := httptest.NewRequest(http.MethodPost, "/internal/users/user_1/subscription", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandlersUpdateSubscriptionBlankSubscriptionTypeReturns400(t *testing.T) {
	cases := []struct {
		name             string
		subscriptionType string
	}{
		{"empty", ""},
		{"space", " "},
		{"tabs and newlines", "\t\n\r "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("downstream should not be called for a blank subscriptionType")
			}))
			defer srv.Close()

			mux := newTestMux(srv.URL)
			payload, _ := json.Marshal(UpdateSubscriptionRequest{SubscriptionType: tc.subscriptionType})
			req := httptest.NewRequest(http.MethodPost, "/internal/users/user_1/subscription", bytes.NewReader(payload))
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
			if rec.Body.String() != "subscriptionType: must not be blank\n" {
				t.Errorf("body = %q, want %q", rec.Body.String(), "subscriptionType: must not be blank\n")
			}
		})
	}
}

func TestHandlersUpdateSubscriptionReturns404WhenDownstream404s(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	mux := newTestMux(srv.URL)
	payload, _ := json.Marshal(UpdateSubscriptionRequest{SubscriptionType: "PREMIUM"})
	req := httptest.NewRequest(http.MethodPost, "/internal/users/user_1/subscription", bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestHandlersUpdateSubscriptionReturns502OnDownstreamServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	mux := newTestMux(srv.URL)
	payload, _ := json.Marshal(UpdateSubscriptionRequest{SubscriptionType: "PREMIUM"})
	req := httptest.NewRequest(http.MethodPost, "/internal/users/user_1/subscription", bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec.Code)
	}
}

// TestHandlersUpdateSubscriptionReturns204OnAnyDownstream2xx checks the
// handler always answers 204 regardless of which specific 2xx the
// downstream sent (here 201, not 200), since it never proxies this body.
func TestHandlersUpdateSubscriptionReturns204OnAnyDownstream2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	mux := newTestMux(srv.URL)
	payload, _ := json.Marshal(UpdateSubscriptionRequest{SubscriptionType: "PREMIUM"})
	req := httptest.NewRequest(http.MethodPost, "/internal/users/user_1/subscription", bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

// --- Handlers.getUser ---

func TestHandlersGetUserProxiesSuccessVerbatim(t *testing.T) {
	body := `{"clerkId":"user_1"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))
	defer srv.Close()

	mux := newTestMux(srv.URL)
	req := httptest.NewRequest(http.MethodGet, "/internal/users/user_1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != body {
		t.Errorf("body = %q, want %q", rec.Body.String(), body)
	}
}

func TestHandlersGetUserReturns404WhenDownstream404s(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	mux := newTestMux(srv.URL)
	req := httptest.NewRequest(http.MethodGet, "/internal/users/user_1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestHandlersGetUserReturns502OnDownstreamServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	mux := newTestMux(srv.URL)
	req := httptest.NewRequest(http.MethodGet, "/internal/users/user_1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec.Code)
	}
}

// --- isBlank ---

func TestIsBlank(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", true},
		{"   ", true},
		{"\t\n\r", true},
		{"a", false},
		{" a ", false},
	}
	for _, tc := range cases {
		if got := isBlank(tc.in); got != tc.want {
			t.Errorf("isBlank(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
