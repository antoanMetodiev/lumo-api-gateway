package reqlog

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// captureLog redirects the standard logger's output for the duration of the
// test and restores it afterward — Middleware logs via the package-level
// log.Printf, so this is the only way to observe what it wrote.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	orig := log.Writer()
	origFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(orig)
		log.SetFlags(origFlags)
	})
	return &buf
}

func TestMiddlewarePassesThroughToNextHandler(t *testing.T) {
	captureLog(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("hello"))
	})

	req := httptest.NewRequest(http.MethodGet, "/brew", nil)
	rec := httptest.NewRecorder()
	Middleware(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTeapot)
	}
	if rec.Body.String() != "hello" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "hello")
	}
}

func TestMiddlewareLogsActualStatusCode(t *testing.T) {
	buf := captureLog(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	Middleware(next).ServeHTTP(httptest.NewRecorder(), req)

	if !strings.Contains(buf.String(), "404") {
		t.Errorf("log output = %q, want it to mention status 404", buf.String())
	}
}

func TestMiddlewareDefaultsToStatusOKWhenHandlerNeverCallsWriteHeader(t *testing.T) {
	buf := captureLog(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodGet, "/implicit", nil)
	Middleware(next).ServeHTTP(httptest.NewRecorder(), req)

	if !strings.Contains(buf.String(), "200") {
		t.Errorf("log output = %q, want it to mention the implicit 200", buf.String())
	}
}

func TestMiddlewareLogsMethodAndPath(t *testing.T) {
	buf := captureLog(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	req := httptest.NewRequest(http.MethodPost, "/watchlist", nil)
	Middleware(next).ServeHTTP(httptest.NewRecorder(), req)

	out := buf.String()
	if !strings.Contains(out, "POST") {
		t.Errorf("log output = %q, want it to mention the method POST", out)
	}
	if !strings.Contains(out, "/watchlist") {
		t.Errorf("log output = %q, want it to mention the path", out)
	}
}

func TestMiddlewareLogsQueryStringWithLeadingQuestionMark(t *testing.T) {
	buf := captureLog(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	req := httptest.NewRequest(http.MethodGet, "/search?query=ada&limit=5", nil)
	Middleware(next).ServeHTTP(httptest.NewRecorder(), req)

	out := buf.String()
	if !strings.Contains(out, "/search?query=ada&limit=5") {
		t.Errorf("log output = %q, want the path with its query string intact", out)
	}
}

func TestMiddlewareNoQueryStringHasNoTrailingQuestionMark(t *testing.T) {
	buf := captureLog(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	req := httptest.NewRequest(http.MethodGet, "/heartbeat", nil)
	Middleware(next).ServeHTTP(httptest.NewRecorder(), req)

	if strings.Contains(buf.String(), "/heartbeat?") {
		t.Errorf("log output = %q, want no trailing '?' when there's no query string", buf.String())
	}
}
