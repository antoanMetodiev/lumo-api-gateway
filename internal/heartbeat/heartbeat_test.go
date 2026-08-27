package heartbeat

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// callTracker records whether/how a fire-and-forget downstream call fired,
// synchronized so tests can safely wait for the goroutine without a race.
type callTracker struct {
	mu           sync.Mutex
	called       bool
	done         chan struct{}
	err          error
	ctxErrAtCall error // ctx.Err() captured synchronously inside fn, before the caller's deferred cancel can race with it
}

func newCallTracker(err error) *callTracker {
	return &callTracker{done: make(chan struct{}), err: err}
}

func (c *callTracker) fn(ctx context.Context) error {
	c.mu.Lock()
	c.called = true
	c.ctxErrAtCall = ctx.Err()
	c.mu.Unlock()
	close(c.done)
	return c.err
}

func (c *callTracker) waitCalled(t *testing.T) {
	t.Helper()
	select {
	case <-c.done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the fire-and-forget call")
	}
}

func TestHeartbeatReturns200Immediately(t *testing.T) {
	movies := newCallTracker(nil)
	series := newCallTracker(nil)
	users := newCallTracker(nil)
	payments := newCallTracker(nil)

	mux := http.NewServeMux()
	NewHandlers(movies.fn, series.fn, users.fn, payments.fn).Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/heartbeat", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

// TestHeartbeatFiresAllFourDownstreamCalls checks every one of the four
// injected calls actually fires, not just the response status.
func TestHeartbeatFiresAllFourDownstreamCalls(t *testing.T) {
	movies := newCallTracker(nil)
	series := newCallTracker(nil)
	users := newCallTracker(nil)
	payments := newCallTracker(nil)

	mux := http.NewServeMux()
	NewHandlers(movies.fn, series.fn, users.fn, payments.fn).Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/heartbeat", nil)
	mux.ServeHTTP(httptest.NewRecorder(), req)

	movies.waitCalled(t)
	series.waitCalled(t)
	users.waitCalled(t)
	payments.waitCalled(t)
}

// TestHeartbeatCallsAreDetachedFromRequestContext checks the fire-and-forget
// calls get a context that's still alive at call time, built from
// context.Background() rather than derived from the (already-returned) HTTP
// request's context — ServeHTTP returning (and net/http canceling the
// request's context right after) must not affect a call still in flight.
func TestHeartbeatCallsAreDetachedFromRequestContext(t *testing.T) {
	movies := newCallTracker(nil)
	mux := http.NewServeMux()
	NewHandlers(movies.fn, func(context.Context) error { return nil }, func(context.Context) error { return nil }, func(context.Context) error { return nil }).Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/heartbeat", nil)
	mux.ServeHTTP(httptest.NewRecorder(), req)

	movies.waitCalled(t)
	movies.mu.Lock()
	ctxErr := movies.ctxErrAtCall
	movies.mu.Unlock()

	if ctxErr != nil {
		t.Errorf("expected the call's context to still be alive at call time, got Err() = %v", ctxErr)
	}
}

// TestHeartbeatOneFailingCallDoesNotAffectOthers checks failures are only
// logged, never surfaced to the HTTP response or to the other three calls.
func TestHeartbeatOneFailingCallDoesNotAffectOthers(t *testing.T) {
	movies := newCallTracker(errors.New("movies-svc is down"))
	series := newCallTracker(nil)
	users := newCallTracker(nil)
	payments := newCallTracker(nil)

	mux := http.NewServeMux()
	NewHandlers(movies.fn, series.fn, users.fn, payments.fn).Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/heartbeat", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 even though one downstream call fails", rec.Code)
	}
	series.waitCalled(t)
	users.waitCalled(t)
	payments.waitCalled(t)
}

// TestFireAndForgetRecoversFromPanic checks a panicking call is recovered
// and logged rather than crashing the process — heartbeat is a periodic,
// unattended background task, so a single bad downstream call must never
// take the whole gateway down.
func TestFireAndForgetRecoversFromPanic(t *testing.T) {
	done := make(chan struct{})
	panicky := func(ctx context.Context) error {
		defer close(done)
		panic("boom")
	}

	mux := http.NewServeMux()
	NewHandlers(panicky, func(context.Context) error { return nil }, func(context.Context) error { return nil }, func(context.Context) error { return nil }).Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/heartbeat", nil)
	rec := httptest.NewRecorder()

	// If the panic isn't recovered inside the goroutine, it terminates the
	// whole test binary rather than failing this one test — that failure
	// mode itself is the thing under test here.
	mux.ServeHTTP(rec, req)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the panicking call")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}
