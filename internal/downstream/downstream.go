// Package downstream is the gateway's outbound HTTP client for every
// downstream service call (lumo-movies-svc, lumo-series-svc,
// lumo-movies-generator[-by-actors], lumo-user-svc, lumo-payments-svc) — the
// Go analog of the Feign clients + FeignConfig's RequestInterceptor.
//
// Most gateway endpoints are pure proxies that never inspect the downstream
// response body, so Result carries raw bytes rather than a decoded struct;
// WriteProxy copies them straight onto the ResponseWriter. The handful of
// call sites that do need typed fields (internal/users' UserResponse) decode
// the bytes themselves with Decode.
package downstream

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

type Client struct {
	HTTP           *http.Client
	InternalSecret string // internal.service-secret, stamped on every call
}

func New(internalSecret string) *Client {
	return &Client{
		HTTP:           &http.Client{Timeout: 30 * time.Second},
		InternalSecret: internalSecret,
	}
}

// Result is a completed downstream HTTP response — any status code, not
// just 2xx. Mirrors how Feign surfaces a non-2xx response as a typed
// FeignException carrying the status, rather than a Go-style error.
type Result struct {
	StatusCode  int
	Body        []byte
	ContentType string
}

func (r *Result) OK() bool { return r.StatusCode >= 200 && r.StatusCode < 300 }

// Do issues a request against a fully-formed downstream URL. extraHeaders
// may be nil (e.g. X-Clerk-User-Id, only needed on calls the caller has
// already verified a session for). Content-Type is set to application/json
// automatically whenever body is non-nil. A non-nil error here means a
// transport-level failure (connection refused, timeout, ...), not a non-2xx
// response — check Result.StatusCode / Result.OK() for that.
func (c *Client) Do(ctx context.Context, method, url string, body []byte, extraHeaders map[string]string) (*Result, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Internal-Api-Key", c.InternalSecret)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &Result{
		StatusCode:  resp.StatusCode,
		Body:        respBody,
		ContentType: resp.Header.Get("Content-Type"),
	}, nil
}

// DoJSON is Do with the request body marshaled from payload.
func (c *Client) DoJSON(ctx context.Context, method, url string, payload any, extraHeaders map[string]string) (*Result, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return c.Do(ctx, method, url, body, extraHeaders)
}

// Decode JSON-decodes a Result's body into out.
func Decode(r *Result, out any) error {
	return json.Unmarshal(r.Body, out)
}

// WriteProxy copies a downstream Result verbatim onto w — same status code,
// same Content-Type, same body — for the pure-passthrough endpoints that
// have no Java try/catch reshaping the response.
func WriteProxy(w http.ResponseWriter, r *Result) {
	if r.ContentType != "" {
		w.Header().Set("Content-Type", r.ContentType)
	}
	w.WriteHeader(r.StatusCode)
	w.Write(r.Body)
}
