// Package cors mirrors SecurityConfig.corsConfigurationSource() exactly —
// same allowed origins/methods/headers, same credentials/exposed-headers
// behavior. Go's net/http has no built-in CORS handling (Spring Security's
// CorsFilter runs ahead of routing and answers preflight OPTIONS requests
// itself), so this middleware does both jobs: it stamps the response headers
// on every request and short-circuits OPTIONS preflight requests with a 204
// before they'd otherwise hit the mux (which only has GET/POST/etc. routes
// registered, never OPTIONS).
package cors

import (
	"net/http"
	"slices"
)

var (
	allowedOrigins = []string{
		"https://lumoflix.vercel.app",
		"http://localhost:5173",
	}
	allowedMethods = "GET, POST, PUT, DELETE, OPTIONS"
	allowedHeaders = "Authorization, Content-Type, X-Custom-Logout, X-Requested-With"
	exposedHeaders = "Set-Cookie, X-Custom-Logout"
)

// Middleware wraps a handler, applying the CORS policy above to every request.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && slices.Contains(allowedOrigins, origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
			w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
			w.Header().Set("Access-Control-Expose-Headers", exposedHeaders)
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
