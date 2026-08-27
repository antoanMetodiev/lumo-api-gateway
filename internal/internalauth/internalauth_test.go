package internalauth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const testSecret = "shared-internal-secret"

func TestMiddlewareUnprotectedPathBypassesAuth(t *testing.T) {
	var reached bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true; w.WriteHeader(http.StatusOK) })
	h := Middleware(testSecret, []string{"/internal"})(next)

	req := httptest.NewRequest(http.MethodGet, "/heartbeat", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !reached || rec.Code != http.StatusOK {
		t.Errorf("expected an unprotected path through untouched, reached=%v status=%d", reached, rec.Code)
	}
}

func TestMiddlewareProtectedPathValidSecretPasses(t *testing.T) {
	var reached bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true; w.WriteHeader(http.StatusOK) })
	h := Middleware(testSecret, []string{"/internal"})(next)

	req := httptest.NewRequest(http.MethodPost, "/internal/subscriptions/user_1", nil)
	req.Header.Set(InternalAPIKeyHeader, testSecret)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !reached {
		t.Error("handler should run with a valid internal secret")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestMiddlewareProtectedPathMissingSecretIs401(t *testing.T) {
	var reached bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true })
	h := Middleware(testSecret, []string{"/internal"})(next)

	req := httptest.NewRequest(http.MethodPost, "/internal/subscriptions/user_1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if reached {
		t.Error("handler must not run without the internal secret")
	}
}

func TestMiddlewareProtectedPathWrongSecretIs401(t *testing.T) {
	h := Middleware(testSecret, []string{"/internal"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))

	req := httptest.NewRequest(http.MethodPost, "/internal/subscriptions/user_1", nil)
	req.Header.Set(InternalAPIKeyHeader, "totally-wrong-secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// TestMiddlewarePathMatchingIsPrefixSlashBounded checks matches() requires
// an exact match or "/prefix/..." — not a bare string-prefix — so
// "/internal-other-thing" must not be swept in by an "/internal" entry.
func TestMiddlewarePathMatchingIsPrefixSlashBounded(t *testing.T) {
	var reached bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true; w.WriteHeader(http.StatusOK) })
	h := Middleware(testSecret, []string{"/internal"})(next)

	req := httptest.NewRequest(http.MethodGet, "/internal-other-thing", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !reached {
		t.Error("expected /internal-other-thing to NOT match the /internal protected-path entry")
	}
}

func TestMiddlewareExactMatch(t *testing.T) {
	h := Middleware(testSecret, []string{"/internal"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))

	req := httptest.NewRequest(http.MethodGet, "/internal", nil)
	req.Header.Set(InternalAPIKeyHeader, testSecret)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for an exact protected-path match", rec.Code)
	}
}
