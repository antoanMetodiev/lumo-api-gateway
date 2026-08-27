package clerkauth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testIssuer = "https://test.clerk.accounts.dev"

// jwksTestServer serves a mutable JWKS over HTTP so tests can simulate
// Clerk rotating its signing key mid-test (keyCache.get's reactive-refresh
// path only re-fetches when it's asked for a kid it hasn't cached yet).
type jwksTestServer struct {
	*httptest.Server
	mu   sync.Mutex
	keys []jwk
}

func newJWKSTestServer(t *testing.T) *jwksTestServer {
	t.Helper()
	s := &jwksTestServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		json.NewEncoder(w).Encode(jwkSet{Keys: s.keys})
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *jwksTestServer) setKeys(keys ...jwk) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys = keys
}

// newRSAKey generates a fresh RSA keypair and its JWK representation under kid.
func newRSAKey(t *testing.T, kid string) (*rsa.PrivateKey, jwk) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}
	return priv, jwk{
		Kty: "RSA",
		Kid: kid,
		N:   base64.RawURLEncoding.EncodeToString(priv.PublicKey.N.Bytes()),
		E:   encodeE(priv.PublicKey.E),
	}
}

func encodeE(e int) string {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(e))
	i := 0
	for i < len(buf)-1 && buf[i] == 0 {
		i++
	}
	return base64.RawURLEncoding.EncodeToString(buf[i:])
}

// signToken builds and signs an RS256 JWT with kid in its header — the
// signature Clerk's real tokens carry that keyfunc uses to pick which
// cached public key to verify against.
func signToken(t *testing.T, priv *rsa.PrivateKey, kid string, claims jwt.RegisteredClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	ss, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("signing token: %v", err)
	}
	return ss
}

func validClaims(sub string) jwt.RegisteredClaims {
	now := time.Now()
	return jwt.RegisteredClaims{
		Issuer:    testIssuer,
		Subject:   sub,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}
}

func TestVerifyAndGetUserIDValidToken(t *testing.T) {
	srv := newJWKSTestServer(t)
	priv, k := newRSAKey(t, "key-1")
	srv.setKeys(k)

	v := New(srv.URL, testIssuer)
	token := signToken(t, priv, "key-1", validClaims("user_abc123"))

	if got := v.VerifyAndGetUserID(token); got != "user_abc123" {
		t.Errorf("VerifyAndGetUserID = %q, want %q", got, "user_abc123")
	}
}

func TestVerifyAndGetUserIDEmptyOrBlankToken(t *testing.T) {
	v := New("http://unused.invalid", testIssuer)
	for _, tok := range []string{"", "   "} {
		if got := v.VerifyAndGetUserID(tok); got != "" {
			t.Errorf("VerifyAndGetUserID(%q) = %q, want empty", tok, got)
		}
	}
}

func TestVerifyAndGetUserIDExpiredToken(t *testing.T) {
	srv := newJWKSTestServer(t)
	priv, k := newRSAKey(t, "key-1")
	srv.setKeys(k)

	v := New(srv.URL, testIssuer)
	claims := jwt.RegisteredClaims{
		Issuer:    testIssuer,
		Subject:   "user_abc123",
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
	}
	token := signToken(t, priv, "key-1", claims)

	if got := v.VerifyAndGetUserID(token); got != "" {
		t.Errorf("VerifyAndGetUserID on an expired token = %q, want empty", got)
	}
}

func TestVerifyAndGetUserIDMissingExpiration(t *testing.T) {
	srv := newJWKSTestServer(t)
	priv, k := newRSAKey(t, "key-1")
	srv.setKeys(k)

	v := New(srv.URL, testIssuer)
	claims := jwt.RegisteredClaims{
		Issuer:   testIssuer,
		Subject:  "user_abc123",
		IssuedAt: jwt.NewNumericDate(time.Now()),
		// ExpiresAt deliberately omitted — jwt.WithExpirationRequired() must
		// reject this outright.
	}
	token := signToken(t, priv, "key-1", claims)

	if got := v.VerifyAndGetUserID(token); got != "" {
		t.Errorf("VerifyAndGetUserID on a token with no exp claim = %q, want empty", got)
	}
}

func TestVerifyAndGetUserIDWrongIssuer(t *testing.T) {
	srv := newJWKSTestServer(t)
	priv, k := newRSAKey(t, "key-1")
	srv.setKeys(k)

	v := New(srv.URL, testIssuer)
	claims := validClaims("user_abc123")
	claims.Issuer = "https://someone-elses-clerk-instance.accounts.dev"
	token := signToken(t, priv, "key-1", claims)

	if got := v.VerifyAndGetUserID(token); got != "" {
		t.Errorf("VerifyAndGetUserID with a mismatched issuer = %q, want empty", got)
	}
}

// TestVerifyAndGetUserIDMissingSubject checks the explicit
// claims.Subject == "" guard — jwt.ParseWithClaims itself doesn't require a
// non-empty "sub", so this must be enforced separately, mirroring
// DefaultJWTClaimsVerifier requiring sub/exp/iat to be present.
func TestVerifyAndGetUserIDMissingSubject(t *testing.T) {
	srv := newJWKSTestServer(t)
	priv, k := newRSAKey(t, "key-1")
	srv.setKeys(k)

	v := New(srv.URL, testIssuer)
	claims := validClaims("")
	token := signToken(t, priv, "key-1", claims)

	if got := v.VerifyAndGetUserID(token); got != "" {
		t.Errorf("VerifyAndGetUserID with an empty sub claim = %q, want empty", got)
	}
}

// TestVerifyAndGetUserIDMissingIssuedAt checks the explicit
// claims.IssuedAt == nil guard, the same "required claims" story as the
// subject check above.
func TestVerifyAndGetUserIDMissingIssuedAt(t *testing.T) {
	srv := newJWKSTestServer(t)
	priv, k := newRSAKey(t, "key-1")
	srv.setKeys(k)

	v := New(srv.URL, testIssuer)
	claims := jwt.RegisteredClaims{
		Issuer:    testIssuer,
		Subject:   "user_abc123",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		// IssuedAt deliberately omitted.
	}
	token := signToken(t, priv, "key-1", claims)

	if got := v.VerifyAndGetUserID(token); got != "" {
		t.Errorf("VerifyAndGetUserID with no iat claim = %q, want empty", got)
	}
}

func TestVerifyAndGetUserIDMissingKidHeader(t *testing.T) {
	srv := newJWKSTestServer(t)
	priv, k := newRSAKey(t, "key-1")
	srv.setKeys(k)

	v := New(srv.URL, testIssuer)
	// Sign directly rather than via signToken, so the "kid" header is never
	// set at all.
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, validClaims("user_abc123"))
	ss, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("signing token: %v", err)
	}

	if got := v.VerifyAndGetUserID(ss); got != "" {
		t.Errorf("VerifyAndGetUserID on a token with no kid header = %q, want empty", got)
	}
}

func TestVerifyAndGetUserIDUnknownKid(t *testing.T) {
	srv := newJWKSTestServer(t)
	_, k := newRSAKey(t, "key-1")
	srv.setKeys(k)

	// Sign with a kid the JWKS server has never advertised at all.
	otherPriv, _ := newRSAKey(t, "key-does-not-exist")
	token := signToken(t, otherPriv, "key-does-not-exist", validClaims("user_abc123"))

	v := New(srv.URL, testIssuer)
	if got := v.VerifyAndGetUserID(token); got != "" {
		t.Errorf("VerifyAndGetUserID with an unknown kid = %q, want empty", got)
	}
}

// TestVerifyAndGetUserIDReactiveKeyRotation is the scenario keyCache exists
// for: Clerk rotates its signing key, a token arrives signed with the new
// key before this process has ever seen it, and verification must succeed
// by refetching the JWKS on the cache miss — not just fail until the next
// unrelated refresh.
func TestVerifyAndGetUserIDReactiveKeyRotation(t *testing.T) {
	srv := newJWKSTestServer(t)
	oldPriv, oldKey := newRSAKey(t, "key-old")
	srv.setKeys(oldKey)

	v := New(srv.URL, testIssuer)

	oldToken := signToken(t, oldPriv, "key-old", validClaims("user_abc123"))
	if got := v.VerifyAndGetUserID(oldToken); got != "user_abc123" {
		t.Fatalf("VerifyAndGetUserID with the original key = %q, want %q", got, "user_abc123")
	}

	// Rotate: the server now serves a brand new key under a new kid.
	newPriv, newKey := newRSAKey(t, "key-new")
	srv.setKeys(newKey)

	newToken := signToken(t, newPriv, "key-new", validClaims("user_xyz789"))
	if got := v.VerifyAndGetUserID(newToken); got != "user_xyz789" {
		t.Errorf("VerifyAndGetUserID after rotation = %q, want %q (should have refetched the JWKS)", got, "user_xyz789")
	}
}

func TestVerifyAndGetUserIDTamperedSignature(t *testing.T) {
	srv := newJWKSTestServer(t)
	priv, k := newRSAKey(t, "key-1")
	srv.setKeys(k)

	v := New(srv.URL, testIssuer)
	token := signToken(t, priv, "key-1", validClaims("user_abc123"))
	// Flip the last character of the signature segment.
	tampered := token[:len(token)-1] + flipChar(token[len(token)-1])

	if got := v.VerifyAndGetUserID(tampered); got != "" {
		t.Errorf("VerifyAndGetUserID on a tampered signature = %q, want empty", got)
	}
}

func flipChar(c byte) string {
	if c == 'A' {
		return "B"
	}
	return "A"
}

// TestVerifyAndGetUserIDWrongSigningMethod checks that a token signed with
// a symmetric HS256 key is rejected outright by
// jwt.WithValidMethods([]string{"RS256"}) — otherwise an attacker who
// guessed (or brute-forced) nothing at all could forge a token by "signing"
// it with the RSA public key treated as an HMAC secret (the classic
// alg-confusion attack this option exists to close).
func TestVerifyAndGetUserIDWrongSigningMethod(t *testing.T) {
	srv := newJWKSTestServer(t)
	_, k := newRSAKey(t, "key-1")
	srv.setKeys(k)

	v := New(srv.URL, testIssuer)

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims("user_abc123"))
	tok.Header["kid"] = "key-1"
	ss, err := tok.SignedString([]byte("some-arbitrary-hmac-secret"))
	if err != nil {
		t.Fatalf("signing HS256 token: %v", err)
	}

	if got := v.VerifyAndGetUserID(ss); got != "" {
		t.Errorf("VerifyAndGetUserID on an HS256-signed token = %q, want empty", got)
	}
}

func TestVerifyAndGetUserIDJWKSUnreachable(t *testing.T) {
	v := New("http://127.0.0.1:1", testIssuer) // nothing listens here
	priv, _ := newRSAKey(t, "key-1")
	token := signToken(t, priv, "key-1", validClaims("user_abc123"))

	if got := v.VerifyAndGetUserID(token); got != "" {
		t.Errorf("VerifyAndGetUserID with an unreachable JWKS endpoint = %q, want empty", got)
	}
}

func TestVerifyAndGetUserIDJWKSNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	v := New(srv.URL, testIssuer)
	priv, _ := newRSAKey(t, "key-1")
	token := signToken(t, priv, "key-1", validClaims("user_abc123"))

	if got := v.VerifyAndGetUserID(token); got != "" {
		t.Errorf("VerifyAndGetUserID when the JWKS endpoint returns 500 = %q, want empty", got)
	}
}

// TestRefreshSkipsNonRSAAndBlankKidKeys checks that a JWKS containing a
// non-RSA entry (e.g. an EC key Clerk might list alongside its RSA one) or
// one with a blank kid doesn't break parsing the rest of the set — it's
// just skipped, and the valid RSA key alongside it still works.
func TestRefreshSkipsNonRSAAndBlankKidKeys(t *testing.T) {
	srv := newJWKSTestServer(t)
	priv, goodKey := newRSAKey(t, "key-1")
	srv.setKeys(
		jwk{Kty: "EC", Kid: "ec-key", N: "irrelevant", E: "irrelevant"},
		jwk{Kty: "RSA", Kid: "", N: goodKey.N, E: goodKey.E}, // blank kid, otherwise valid
		goodKey,
	)

	v := New(srv.URL, testIssuer)
	token := signToken(t, priv, "key-1", validClaims("user_abc123"))

	if got := v.VerifyAndGetUserID(token); got != "user_abc123" {
		t.Errorf("VerifyAndGetUserID = %q, want %q (the valid RSA key should still be usable)", got, "user_abc123")
	}
}

func TestRsaPublicKeyInvalidBase64(t *testing.T) {
	if _, err := rsaPublicKey("not-valid-base64url!!!", "AQAB"); err == nil {
		t.Error("expected an error decoding an invalid base64url modulus, got nil")
	}
	if _, err := rsaPublicKey(base64.RawURLEncoding.EncodeToString([]byte{1, 2, 3}), "not-valid!!!"); err == nil {
		t.Error("expected an error decoding an invalid base64url exponent, got nil")
	}
}

// --- Middleware ---

func newTestVerifier(t *testing.T) (*Verifier, *rsa.PrivateKey) {
	t.Helper()
	srv := newJWKSTestServer(t)
	priv, k := newRSAKey(t, "key-1")
	srv.setKeys(k)
	return New(srv.URL, testIssuer), priv
}

func TestMiddlewareUnprotectedPathBypassesAuth(t *testing.T) {
	v, _ := newTestVerifier(t)
	var reached bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true; w.WriteHeader(http.StatusOK) })

	h := Middleware(v, []string{"/user"})(next)
	req := httptest.NewRequest(http.MethodGet, "/heartbeat", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !reached || rec.Code != http.StatusOK {
		t.Errorf("expected an unprotected path to pass through untouched, reached=%v status=%d", reached, rec.Code)
	}
}

func TestMiddlewareProtectedPathValidBearerSetsContext(t *testing.T) {
	v, priv := newTestVerifier(t)
	token := signToken(t, priv, "key-1", validClaims("user_abc123"))

	var gotID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = ClerkUserID(r)
		w.WriteHeader(http.StatusOK)
	})

	h := Middleware(v, []string{"/user"})(next)
	req := httptest.NewRequest(http.MethodGet, "/user/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if gotID != "user_abc123" {
		t.Errorf("ClerkUserID(r) inside the handler = %q, want %q", gotID, "user_abc123")
	}
}

func TestMiddlewareProtectedPathMissingAuthHeaderIs401(t *testing.T) {
	v, _ := newTestVerifier(t)
	var reached bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true })

	h := Middleware(v, []string{"/user"})(next)
	req := httptest.NewRequest(http.MethodGet, "/user/me", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if reached {
		t.Error("handler must not run without a valid bearer token")
	}
}

func TestMiddlewareProtectedPathNonBearerAuthHeaderIs401(t *testing.T) {
	v, priv := newTestVerifier(t)
	token := signToken(t, priv, "key-1", validClaims("user_abc123"))

	h := Middleware(v, []string{"/user"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	req := httptest.NewRequest(http.MethodGet, "/user/me", nil)
	// A valid token, but not prefixed with "Bearer " — must still be
	// rejected, matching the exact-prefix check in Middleware.
	req.Header.Set("Authorization", token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestMiddlewareProtectedPathInvalidTokenIs401(t *testing.T) {
	v, _ := newTestVerifier(t)
	h := Middleware(v, []string{"/user"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))

	req := httptest.NewRequest(http.MethodGet, "/user/me", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-jwt")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// TestMiddlewarePathMatchingIsPrefixSlashBounded mirrors gatewayauth's
// equivalent check: "/users-something-else" must not match a "/user" entry.
func TestMiddlewarePathMatchingIsPrefixSlashBounded(t *testing.T) {
	v, _ := newTestVerifier(t)
	var reached bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true; w.WriteHeader(http.StatusOK) })

	h := Middleware(v, []string{"/user"})(next)
	req := httptest.NewRequest(http.MethodGet, "/users-something-else", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !reached {
		t.Error("expected /users-something-else to NOT match the /user protected-path entry")
	}
}

func TestMiddlewareExactAndSubpathMatches(t *testing.T) {
	v, priv := newTestVerifier(t)
	token := signToken(t, priv, "key-1", validClaims("user_abc123"))
	h := Middleware(v, []string{"/user"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))

	for _, path := range []string{"/user", "/user/me", "/user/anything/deeper"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("GET %s = %d, want 200", path, rec.Code)
			}
		})
	}
}

func TestClerkUserIDWithoutMiddleware(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/user/me", nil)
	if got := ClerkUserID(req); got != "" {
		t.Errorf("ClerkUserID on a request that never passed through Middleware = %q, want empty", got)
	}
}
