package downstream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientDoSetsInternalSecretHeader(t *testing.T) {
	var gotSecret string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSecret = r.Header.Get("X-Internal-Api-Key")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New("my-secret")
	_, err := c.Do(context.Background(), http.MethodGet, srv.URL, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotSecret != "my-secret" {
		t.Errorf("X-Internal-Api-Key = %q, want %q", gotSecret, "my-secret")
	}
}

func TestClientDoSetsContentTypeOnlyWhenBodyPresent(t *testing.T) {
	var gotContentType string
	var sawContentTypeHeader bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		_, sawContentTypeHeader = r.Header["Content-Type"]
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	c := New("secret")

	if _, err := c.Do(context.Background(), http.MethodGet, srv.URL, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sawContentTypeHeader {
		t.Errorf("expected no Content-Type header on a nil body, got %q", gotContentType)
	}

	if _, err := c.Do(context.Background(), http.MethodPost, srv.URL, []byte(`{"a":1}`), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type with a non-nil body = %q, want application/json", gotContentType)
	}
}

func TestClientDoExtraHeadersAreSet(t *testing.T) {
	var gotClerkID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClerkID = r.Header.Get("X-Clerk-User-Id")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	c := New("secret")

	_, err := c.Do(context.Background(), http.MethodGet, srv.URL, nil, map[string]string{"X-Clerk-User-Id": "user_abc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotClerkID != "user_abc" {
		t.Errorf("X-Clerk-User-Id = %q, want %q", gotClerkID, "user_abc")
	}
}

func TestClientDoNilExtraHeadersDoesNotPanic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer srv.Close()
	c := New("secret")

	if _, err := c.Do(context.Background(), http.MethodGet, srv.URL, nil, nil); err != nil {
		t.Fatalf("unexpected error with nil extraHeaders: %v", err)
	}
}

func TestClientDoCapturesResponseFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte(`{"ok":false}`))
	}))
	defer srv.Close()
	c := New("secret")

	res, err := c.Do(context.Background(), http.MethodGet, srv.URL, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.StatusCode != http.StatusTeapot {
		t.Errorf("StatusCode = %d, want %d", res.StatusCode, http.StatusTeapot)
	}
	if string(res.Body) != `{"ok":false}` {
		t.Errorf("Body = %q", res.Body)
	}
	if res.ContentType != "application/json" {
		t.Errorf("ContentType = %q", res.ContentType)
	}
	if res.OK() {
		t.Error("OK() should be false for a 418 response")
	}
}

func TestResultOKBoundaries(t *testing.T) {
	cases := []struct {
		status int
		want   bool
	}{
		{199, false},
		{200, true},
		{204, true},
		{299, true},
		{300, false},
		{404, false},
		{500, false},
	}
	for _, tc := range cases {
		r := &Result{StatusCode: tc.status}
		if got := r.OK(); got != tc.want {
			t.Errorf("Result{StatusCode: %d}.OK() = %v, want %v", tc.status, got, tc.want)
		}
	}
}

// TestClientDoTransportErrorReturnsError checks that a transport-level
// failure (nothing listening) surfaces as a Go error, not a Result with some
// sentinel status code — the whole point of Result vs error is that a
// non-nil error means "couldn't even complete the round trip."
func TestClientDoTransportErrorReturnsError(t *testing.T) {
	c := New("secret")
	_, err := c.Do(context.Background(), http.MethodGet, "http://127.0.0.1:1", nil, nil)
	if err == nil {
		t.Fatal("expected a transport error when nothing is listening, got nil")
	}
}

func TestClientDoJSONMarshalsPayload(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		r.Body.Read(buf)
		gotBody = string(buf)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	c := New("secret")

	payload := struct {
		Name string `json:"name"`
	}{Name: "Ada"}
	_, err := c.DoJSON(context.Background(), http.MethodPost, srv.URL, payload, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody != `{"name":"Ada"}` {
		t.Errorf("request body = %q, want %q", gotBody, `{"name":"Ada"}`)
	}
}

func TestClientDoJSONMarshalErrorPropagates(t *testing.T) {
	c := New("secret")
	// A channel is not JSON-marshalable — json.Marshal must fail before any
	// request is even attempted.
	_, err := c.DoJSON(context.Background(), http.MethodPost, "http://unused.invalid", make(chan int), nil)
	if err == nil {
		t.Fatal("expected a marshal error, got nil")
	}
}

func TestDecode(t *testing.T) {
	res := &Result{Body: []byte(`{"clerkId":"user_1","email":"a@example.com"}`)}
	var out struct {
		ClerkID string `json:"clerkId"`
		Email   string `json:"email"`
	}
	if err := Decode(res, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ClerkID != "user_1" || out.Email != "a@example.com" {
		t.Errorf("decoded = %+v", out)
	}
}

func TestDecodeInvalidJSONReturnsError(t *testing.T) {
	res := &Result{Body: []byte(`not json`)}
	var out map[string]any
	if err := Decode(res, &out); err == nil {
		t.Fatal("expected an error decoding invalid JSON, got nil")
	}
}

func TestWriteProxyCopiesStatusContentTypeAndBody(t *testing.T) {
	res := &Result{StatusCode: http.StatusCreated, ContentType: "application/json", Body: []byte(`{"id":1}`)}
	rec := httptest.NewRecorder()

	WriteProxy(rec, res)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
	if rec.Body.String() != `{"id":1}` {
		t.Errorf("body = %q", rec.Body.String())
	}
}

// TestWriteProxyOmitsContentTypeWhenBlank checks WriteProxy doesn't stamp an
// empty Content-Type header (which would otherwise mask whatever default
// net/http might apply) when the downstream response didn't send one.
func TestWriteProxyOmitsContentTypeWhenBlank(t *testing.T) {
	res := &Result{StatusCode: http.StatusNoContent, ContentType: "", Body: nil}
	rec := httptest.NewRecorder()

	WriteProxy(rec, res)

	if _, ok := rec.Result().Header["Content-Type"]; ok {
		t.Error("expected no Content-Type header to be set when the downstream result had none")
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}
