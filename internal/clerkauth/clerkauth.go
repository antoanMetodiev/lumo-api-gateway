// Package clerkauth verifies Clerk session tokens (the JWT the frontend
// sends as `Authorization: Bearer <token>`, from Clerk's `getToken()`)
// against Clerk's own public JWKS — this gateway is the ONLY service that
// does this; every downstream service trusts a header this gateway forwards
// instead (see ../internalauth and each downstream call site's
// X-Clerk-User-Id header). Mirrors ClerkJwtService + ClerkAuthenticationFilter.
package clerkauth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type ctxKey struct{}

var clerkUserIDKey = ctxKey{}

// ClerkUserID reads the verified caller id a handler behind Middleware can
// rely on once the request passed the protected-path check.
func ClerkUserID(r *http.Request) string {
	v, _ := r.Context().Value(clerkUserIDKey).(string)
	return v
}

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwkSet struct {
	Keys []jwk `json:"keys"`
}

// keyCache fetches and caches Clerk's JWKS, refetching whenever it's asked
// for a kid it hasn't seen (e.g. right after Clerk rotates its signing key)
// — the same reactive-refresh effect as Nimbus's RemoteJWKSet.
type keyCache struct {
	url  string
	http *http.Client

	mu   sync.RWMutex
	keys map[string]*rsa.PublicKey
}

func newKeyCache(url string) *keyCache {
	return &keyCache{url: url, http: &http.Client{Timeout: 10 * time.Second}, keys: map[string]*rsa.PublicKey{}}
}

func (c *keyCache) get(kid string) (*rsa.PublicKey, error) {
	c.mu.RLock()
	key, ok := c.keys[kid]
	c.mu.RUnlock()
	if ok {
		return key, nil
	}

	if err := c.refresh(); err != nil {
		return nil, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	key, ok = c.keys[kid]
	if !ok {
		return nil, fmt.Errorf("clerkauth: no key with kid %q in JWKS", kid)
	}
	return key, nil
}

func (c *keyCache) refresh() error {
	resp, err := c.http.Get(c.url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("clerkauth: JWKS fetch returned %d", resp.StatusCode)
	}

	var set jwkSet
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return err
	}

	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}
		pub, err := rsaPublicKey(k.N, k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}

	c.mu.Lock()
	c.keys = keys
	c.mu.Unlock()
	return nil
}

func rsaPublicKey(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, err
	}

	e := 0
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}

	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}, nil
}

// Verifier mirrors ClerkJwtService.
type Verifier struct {
	keys   *keyCache
	issuer string
}

func New(jwksURL, issuer string) *Verifier {
	return &Verifier{keys: newKeyCache(jwksURL), issuer: issuer}
}

func (v *Verifier) keyfunc(token *jwt.Token) (any, error) {
	if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
		return nil, fmt.Errorf("clerkauth: unexpected signing method %v", token.Header["alg"])
	}
	kid, _ := token.Header["kid"].(string)
	if kid == "" {
		return nil, fmt.Errorf("clerkauth: token has no kid header")
	}
	return v.keys.get(kid)
}

// VerifyAndGetUserID mirrors ClerkJwtService.verifyAndGetUserId: returns the
// Clerk user id (the `sub` claim) for a valid token, or "" if the token is
// missing, expired, or fails signature/issuer/required-claims verification
// (mirrors DefaultJWTClaimsVerifier requiring sub/exp/iat to be present).
func (v *Verifier) VerifyAndGetUserID(bearerToken string) string {
	if strings.TrimSpace(bearerToken) == "" {
		return ""
	}

	claims := jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(bearerToken, &claims, v.keyfunc,
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(v.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil || !token.Valid {
		return ""
	}
	if claims.Subject == "" || claims.IssuedAt == nil {
		return ""
	}

	return claims.Subject
}

// Middleware mirrors ClerkAuthenticationFilter: enforces a valid Clerk
// session only on the path prefixes listed in protectedPaths (exact match or
// "/prefix/..."), exactly like SecurityConfig.PROTECTED_PATHS, and stores
// the verified user id in the request context for ClerkUserID to read.
func Middleware(verifier *Verifier, protectedPaths []string) func(http.Handler) http.Handler {
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

			authHeader := r.Header.Get("Authorization")
			token := ""
			if strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			}

			clerkUserID := verifier.VerifyAndGetUserID(token)
			if clerkUserID == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), clerkUserIDKey, clerkUserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
