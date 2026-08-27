package cors

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareAllowedOriginGetsCORSHeaders(t *testing.T) {
	var reached bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true; w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/whatever", nil)
	req.Header.Set("Origin", "https://lumoflix.vercel.app")
	rec := httptest.NewRecorder()
	Middleware(next).ServeHTTP(rec, req)

	if !reached {
		t.Fatal("expected the wrapped handler to run for a non-OPTIONS request")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://lumoflix.vercel.app" {
		t.Errorf("Access-Control-Allow-Origin = %q", got)
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Errorf("Vary = %q, want %q", got, "Origin")
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Access-Control-Allow-Credentials = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, PUT, DELETE, OPTIONS" {
		t.Errorf("Access-Control-Allow-Methods = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "Authorization, Content-Type, X-Custom-Logout, X-Requested-With" {
		t.Errorf("Access-Control-Allow-Headers = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Expose-Headers"); got != "Set-Cookie, X-Custom-Logout" {
		t.Errorf("Access-Control-Expose-Headers = %q", got)
	}
}

func TestMiddlewareSecondAllowedOrigin(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/whatever", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	Middleware(next).ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("Access-Control-Allow-Origin = %q", got)
	}
}

func TestMiddlewareDisallowedOriginGetsNoCORSHeaders(t *testing.T) {
	var reached bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true; w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/whatever", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	Middleware(next).ServeHTTP(rec, req)

	if !reached {
		t.Fatal("expected the wrapped handler to still run for a disallowed origin")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty for a disallowed origin", got)
	}
}

func TestMiddlewareNoOriginHeaderSkipsCORSHeadersEntirely(t *testing.T) {
	var reached bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true; w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/whatever", nil)
	rec := httptest.NewRecorder()
	Middleware(next).ServeHTTP(rec, req)

	if !reached {
		t.Fatal("expected the wrapped handler to run when there's no Origin header at all")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty with no Origin header", got)
	}
}

func TestMiddlewareOptionsRequestShortCircuits(t *testing.T) {
	var reached bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true })

	req := httptest.NewRequest(http.MethodOptions, "/whatever", nil)
	req.Header.Set("Origin", "https://lumoflix.vercel.app")
	rec := httptest.NewRecorder()
	Middleware(next).ServeHTTP(rec, req)

	if reached {
		t.Fatal("expected an OPTIONS preflight to short-circuit before the wrapped handler")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://lumoflix.vercel.app" {
		t.Errorf("Access-Control-Allow-Origin on preflight = %q, want the echoed origin", got)
	}
}

func TestMiddlewareOptionsFromDisallowedOriginStillShortCircuits(t *testing.T) {
	var reached bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true })

	req := httptest.NewRequest(http.MethodOptions, "/whatever", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	Middleware(next).ServeHTTP(rec, req)

	if reached {
		t.Fatal("expected an OPTIONS request to short-circuit even from a disallowed origin")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty for a disallowed origin", got)
	}
}
