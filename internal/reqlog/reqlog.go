// Package reqlog logs every incoming HTTP request — method, path, query
// string, remote addr, resulting status code, and duration — so it's
// visible in Render's log tab that traffic is actually reaching this
// service, and exactly what it asked for.
package reqlog

import (
	"log"
	"net/http"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Middleware wraps a handler, logging one line per request before and after
// it's handled.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		query := ""
		if r.URL.RawQuery != "" {
			query = "?" + r.URL.RawQuery
		}
		log.Printf("--> %s %s%s (from %s)", r.Method, r.URL.Path, query, r.RemoteAddr)

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		log.Printf("<-- %s %s%s %d (%s)", r.Method, r.URL.Path, query, rec.status, time.Since(start))
	})
}
