package watching

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"lumo-api-gateway/internal/downstream"
)

// unreachableBaseURL is a URL nothing is listening on, for exercising the
// transport-error branches (mirrors downstream_test.go's approach).
const unreachableBaseURL = "http://127.0.0.1:1"

// errReader is an io.Reader that always fails, for exercising the
// io.ReadAll error path in Handlers.set without needing real I/O failure.
type errReader struct{}

func (errReader) Read(p []byte) (int, error) { return 0, errors.New("boom") }

func newWatchingTestServer(baseURL string) http.Handler {
	mux := http.NewServeMux()
	NewHandlers(NewClient(downstream.New("secret"), baseURL)).Register(mux)
	return mux
}

// --- Client ---

func TestClientSetSendsPutWithBodyAndClerkHeader(t *testing.T) {
	var gotMethod, gotPath, gotClerkID string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotClerkID = r.Header.Get("X-Clerk-User-Id")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(downstream.New("secret"), srv.URL)
	_, err := c.Set(context.Background(), "user_1", []byte(`{"tmdbId":1,"title":"Foo"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %q, want PUT", gotMethod)
	}
	if gotPath != "/watching" {
		t.Errorf("path = %q, want /watching", gotPath)
	}
	if gotClerkID != "user_1" {
		t.Errorf("X-Clerk-User-Id = %q, want user_1", gotClerkID)
	}
	if string(gotBody) != `{"tmdbId":1,"title":"Foo"}` {
		t.Errorf("body = %q", gotBody)
	}
}

func TestClientClearSendsDeleteWithClerkHeader(t *testing.T) {
	var gotMethod, gotPath, gotClerkID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotClerkID = r.Header.Get("X-Clerk-User-Id")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClient(downstream.New("secret"), srv.URL)
	_, err := c.Clear(context.Background(), "user_2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/watching" {
		t.Errorf("path = %q, want /watching", gotPath)
	}
	if gotClerkID != "user_2" {
		t.Errorf("X-Clerk-User-Id = %q, want user_2", gotClerkID)
	}
}

func TestClientListFriendsWatchingSendsGetWithClerkHeader(t *testing.T) {
	var gotMethod, gotPath, gotClerkID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotClerkID = r.Header.Get("X-Clerk-User-Id")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(downstream.New("secret"), srv.URL)
	_, err := c.ListFriendsWatching(context.Background(), "user_3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/watching/friends" {
		t.Errorf("path = %q, want /watching/friends", gotPath)
	}
	if gotClerkID != "user_3" {
		t.Errorf("X-Clerk-User-Id = %q, want user_3", gotClerkID)
	}
}

// --- Handlers.set (PUT /watching) ---

func TestHandlersSetForwardsBodyByteForByte(t *testing.T) {
	const rawBody = `{"tmdbId":42,"title":"Foo","type":"movie"}`
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	gw := newWatchingTestServer(srv.URL)
	req := httptest.NewRequest(http.MethodPut, "/watching", bytes.NewReader([]byte(rawBody)))
	rec := httptest.NewRecorder()
	gw.ServeHTTP(rec, req)

	if string(gotBody) != rawBody {
		t.Errorf("body forwarded to downstream = %q, want %q", gotBody, rawBody)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != `{"ok":true}` {
		t.Errorf("proxied body = %q", rec.Body.String())
	}
}

func TestHandlersSetDownstream400ProxiedVerbatim(t *testing.T) {
	const errBody = `{"error":"tmdbId must not be blank"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(errBody))
	}))
	defer srv.Close()

	gw := newWatchingTestServer(srv.URL)
	req := httptest.NewRequest(http.MethodPut, "/watching", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	gw.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if rec.Body.String() != errBody {
		t.Errorf("body = %q, want downstream's validation body proxied verbatim: %q", rec.Body.String(), errBody)
	}
}

func TestHandlersSetDownstream2xxProxiedVerbatim(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":7}`))
	}))
	defer srv.Close()

	gw := newWatchingTestServer(srv.URL)
	req := httptest.NewRequest(http.MethodPut, "/watching", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	gw.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
	if rec.Body.String() != `{"id":7}` {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestHandlersSetDownstream500IsBadGateway(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`boom`))
	}))
	defer srv.Close()

	gw := newWatchingTestServer(srv.URL)
	req := httptest.NewRequest(http.MethodPut, "/watching", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	gw.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec.Code)
	}
}

func TestHandlersSetTransportErrorIsBadGateway(t *testing.T) {
	gw := newWatchingTestServer(unreachableBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/watching", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	gw.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec.Code)
	}
}

// TestHandlersSetReadBodyErrorIsBadRequest exercises the io.ReadAll error
// path with a body reader that always fails, rather than needing a real
// I/O failure to trigger it.
func TestHandlersSetReadBodyErrorIsBadRequest(t *testing.T) {
	gw := newWatchingTestServer(unreachableBaseURL) // never reached
	req := httptest.NewRequest(http.MethodPut, "/watching", errReader{})
	rec := httptest.NewRecorder()
	gw.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// --- Handlers.clear (DELETE /watching) ---

func TestHandlersClearSuccessIsNoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	gw := newWatchingTestServer(srv.URL)
	req := httptest.NewRequest(http.MethodDelete, "/watching", nil)
	rec := httptest.NewRecorder()
	gw.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

func TestHandlersClearDownstreamErrorIsBadGateway(t *testing.T) {
	gw := newWatchingTestServer(unreachableBaseURL)
	req := httptest.NewRequest(http.MethodDelete, "/watching", nil)
	rec := httptest.NewRecorder()
	gw.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec.Code)
	}
}

func TestHandlersClearDownstream500IsBadGateway(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	gw := newWatchingTestServer(srv.URL)
	req := httptest.NewRequest(http.MethodDelete, "/watching", nil)
	rec := httptest.NewRecorder()
	gw.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec.Code)
	}
}

// TestHandlersClearDownstream400IsBadGatewayNotVerbatim proves clear does
// NOT route through proxyOrBadGateway: unlike set/listFriendsWatching, a
// downstream 400 here is collapsed to 502 rather than proxied verbatim.
func TestHandlersClearDownstream400IsBadGatewayNotVerbatim(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"should not be proxied verbatim here"}`))
	}))
	defer srv.Close()

	gw := newWatchingTestServer(srv.URL)
	req := httptest.NewRequest(http.MethodDelete, "/watching", nil)
	rec := httptest.NewRecorder()
	gw.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502 (clear inlines its own non-2xx check, no 400 passthrough)", rec.Code)
	}
}

// --- Handlers.listFriendsWatching (GET /watching/friends) ---

func TestHandlersListFriendsWatchingProxyOrBadGateway(t *testing.T) {
	cases := []struct {
		name       string
		unreachable bool
		status      int
		body        string
		wantStatus  int
		wantBody    string
	}{
		{name: "400 proxied verbatim", status: http.StatusBadRequest, body: `{"error":"bad"}`, wantStatus: http.StatusBadRequest, wantBody: `{"error":"bad"}`},
		{name: "200 proxied verbatim", status: http.StatusOK, body: `[{"tmdbId":1}]`, wantStatus: http.StatusOK, wantBody: `[{"tmdbId":1}]`},
		{name: "500 becomes 502", status: http.StatusInternalServerError, body: `oops`, wantStatus: http.StatusBadGateway},
		{name: "transport error becomes 502", unreachable: true, wantStatus: http.StatusBadGateway},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL := unreachableBaseURL
			if !tc.unreachable {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tc.status)
					w.Write([]byte(tc.body))
				}))
				defer srv.Close()
				baseURL = srv.URL
			}

			gw := newWatchingTestServer(baseURL)
			req := httptest.NewRequest(http.MethodGet, "/watching/friends", nil)
			rec := httptest.NewRecorder()
			gw.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantBody != "" && rec.Body.String() != tc.wantBody {
				t.Errorf("body = %q, want %q", rec.Body.String(), tc.wantBody)
			}
		})
	}
}
