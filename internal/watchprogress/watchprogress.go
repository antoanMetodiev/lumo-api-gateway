// Package watchprogress passes /watch-progress through to lumo-user-svc,
// which validates it (new feature, see that service's watchprogress package
// doc comment). Protected by clerkauth.Middleware ("/watch-progress" is in
// cmd/server/main.go's clerkProtectedPaths).
package watchprogress

import (
	"io"
	"log"
	"net/http"

	"lumo-api-gateway/internal/clerkauth"
	"lumo-api-gateway/internal/downstream"
)

type Handlers struct {
	ds      *downstream.Client
	baseURL string
}

func NewHandlers(ds *downstream.Client, baseURL string) *Handlers {
	return &Handlers{ds: ds, baseURL: baseURL}
}

func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /watch-progress", h.proxy)
	mux.HandleFunc("PUT /watch-progress", h.proxy)
	mux.HandleFunc("DELETE /watch-progress", h.proxy)
}

func (h *Handlers) proxy(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Method == http.MethodPut {
		var err error
		if body, err = io.ReadAll(http.MaxBytesReader(w, r.Body, 8<<10)); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	url := h.baseURL + "/watch-progress"
	if r.URL.RawQuery != "" {
		url += "?" + r.URL.RawQuery
	}
	res, err := h.ds.Do(r.Context(), r.Method, url, body, map[string]string{"X-Clerk-User-Id": clerkauth.ClerkUserID(r)})
	if err != nil {
		log.Printf("lumo-user-svc call failed for %s /watch-progress: %v", r.Method, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	// A 400 is the caller's to see; anything else unexpected is ours.
	if !res.OK() && res.StatusCode != http.StatusBadRequest {
		log.Printf("lumo-user-svc call failed for %s /watch-progress: status %d", r.Method, res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	downstream.WriteProxy(w, res)
}
