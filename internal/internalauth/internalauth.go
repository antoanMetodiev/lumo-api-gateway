// Package internalauth authenticates INCOMING service-to-service calls into
// this gateway — the reverse direction from clerkauth (which handles
// browser/frontend-originated requests) and from internal/downstream (which
// stamps this same secret onto every OUTGOING call this gateway makes).
//
// Currently only lumo-payments-svc calls back in like this, to tell
// lumo-user-svc (via this gateway) that a Stripe webhook changed someone's
// plan — there's no live Clerk session for that call, so it can't go through
// clerkauth at all. Same X-Internal-Api-Key secret already shared with
// lumo-user-svc / lumo-payments-svc, just checked in the opposite direction
// here. Mirrors InternalServiceAuthenticationFilter.
package internalauth

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

const InternalAPIKeyHeader = "X-Internal-Api-Key"

// Middleware enforces the internal secret only on the path prefixes listed
// in protectedPaths (exact match or "/prefix/..."), exactly like
// SecurityConfig.INTERNAL_PATHS.
func Middleware(internalSecret string, protectedPaths []string) func(http.Handler) http.Handler {
	secretBytes := []byte(internalSecret)

	isProtected := func(path string) bool {
		for _, p := range protectedPaths {
			if path == p || strings.HasPrefix(path, p+"/") {
				return true
			}
		}
		return false
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isProtected(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			provided := r.Header.Get(InternalAPIKeyHeader)
			if provided == "" || subtle.ConstantTimeCompare([]byte(provided), secretBytes) != 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
