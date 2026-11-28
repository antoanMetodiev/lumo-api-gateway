package actor

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lumo-api-gateway/internal/downstream"
)

type fakeMoviesClient struct {
	res       *downstream.Result
	err       error
	counts    *downstream.Result
	countsErr error
}

func (f *fakeMoviesClient) GetActor(ctx context.Context, imdbID string) (*downstream.Result, error) {
	return &downstream.Result{StatusCode: http.StatusNotFound}, nil
}

func (f *fakeMoviesClient) GetActorWorkCounts(ctx context.Context) (*downstream.Result, error) {
	return f.counts, f.countsErr
}

func (f *fakeMoviesClient) GetActorLatestMovies(ctx context.Context, imdbID string) (*downstream.Result, error) {
	return f.res, f.err
}

type fakeSeriesClient struct {
	res       *downstream.Result
	err       error
	counts    *downstream.Result
	countsErr error
}

func (f *fakeSeriesClient) GetActor(ctx context.Context, imdbID string) (*downstream.Result, error) {
	return &downstream.Result{StatusCode: http.StatusOK, Body: []byte(`{"imdbId":"nm1"}`), ContentType: "application/json"}, nil
}

func (f *fakeSeriesClient) GetActorWorkCounts(ctx context.Context) (*downstream.Result, error) {
	return f.counts, f.countsErr
}

func (f *fakeSeriesClient) GetActorLatestSeries(ctx context.Context, imdbID string) (*downstream.Result, error) {
	return f.res, f.err
}

func newTestServer(movies MoviesClient, series SeriesClient) http.Handler {
	mux := http.NewServeMux()
	NewHandlers(movies, series).Register(mux)
	return mux
}

func TestGetActorLatestWorksCombinesBothLists(t *testing.T) {
	movies := &fakeMoviesClient{res: &downstream.Result{StatusCode: http.StatusOK, Body: []byte(`[{"title":"Movie A"}]`)}}
	series := &fakeSeriesClient{res: &downstream.Result{StatusCode: http.StatusOK, Body: []byte(`[{"title":"Series A"},{"title":"Series B"}]`)}}

	srv := newTestServer(movies, series)
	req := httptest.NewRequest(http.MethodGet, "/actor/latest-works/nm123", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Movie A") || !strings.Contains(body, "Series A") || !strings.Contains(body, "Series B") {
		t.Errorf("body = %s, want all three combined entries", body)
	}
}

func TestGetActorLatestWorksEmptyLists(t *testing.T) {
	movies := &fakeMoviesClient{res: &downstream.Result{StatusCode: http.StatusOK, Body: []byte(`[]`)}}
	series := &fakeSeriesClient{res: &downstream.Result{StatusCode: http.StatusOK, Body: []byte(`[]`)}}

	srv := newTestServer(movies, series)
	req := httptest.NewRequest(http.MethodGet, "/actor/latest-works/nm123", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("body = %q, want an empty JSON array", rec.Body.String())
	}
}

// TestGetActorLatestWorksMoviesFailureAbortsEntirely checks the sequential,
// no-partial-result behavior: a movies-svc failure must short-circuit
// before series-svc is ever called at all.
func TestGetActorLatestWorksMoviesFailureAbortsEntirely(t *testing.T) {
	movies := &fakeMoviesClient{err: errors.New("connection refused")}
	seriesCalled := false
	series := seriesClientFunc(func(ctx context.Context, imdbID string) (*downstream.Result, error) {
		seriesCalled = true
		return &downstream.Result{StatusCode: http.StatusOK, Body: []byte(`[]`)}, nil
	})

	srv := newTestServer(movies, series)
	req := httptest.NewRequest(http.MethodGet, "/actor/latest-works/nm123", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if seriesCalled {
		t.Error("expected the series-svc call to be skipped entirely after the movies-svc call failed")
	}
}

func TestGetActorLatestWorksMoviesNon2xxIs500(t *testing.T) {
	movies := &fakeMoviesClient{res: &downstream.Result{StatusCode: http.StatusInternalServerError, Body: []byte(`oops`)}}
	series := &fakeSeriesClient{res: &downstream.Result{StatusCode: http.StatusOK, Body: []byte(`[]`)}}

	srv := newTestServer(movies, series)
	req := httptest.NewRequest(http.MethodGet, "/actor/latest-works/nm123", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestGetActorLatestWorksSeriesFailureIs500(t *testing.T) {
	movies := &fakeMoviesClient{res: &downstream.Result{StatusCode: http.StatusOK, Body: []byte(`[]`)}}
	series := &fakeSeriesClient{err: errors.New("timeout")}

	srv := newTestServer(movies, series)
	req := httptest.NewRequest(http.MethodGet, "/actor/latest-works/nm123", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestGetActorLatestWorksMalformedMoviesJSONIs500(t *testing.T) {
	movies := &fakeMoviesClient{res: &downstream.Result{StatusCode: http.StatusOK, Body: []byte(`not json`)}}
	series := &fakeSeriesClient{res: &downstream.Result{StatusCode: http.StatusOK, Body: []byte(`[]`)}}

	srv := newTestServer(movies, series)
	req := httptest.NewRequest(http.MethodGet, "/actor/latest-works/nm123", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestGetActorLatestWorksMalformedSeriesJSONIs500(t *testing.T) {
	movies := &fakeMoviesClient{res: &downstream.Result{StatusCode: http.StatusOK, Body: []byte(`[]`)}}
	series := &fakeSeriesClient{res: &downstream.Result{StatusCode: http.StatusOK, Body: []byte(`not json`)}}

	srv := newTestServer(movies, series)
	req := httptest.NewRequest(http.MethodGet, "/actor/latest-works/nm123", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

// seriesClientFunc adapts a plain function to the SeriesClient interface,
// so TestGetActorLatestWorksMoviesFailureAbortsEntirely can observe whether
// it was ever called without needing a stateful fake struct.
type seriesClientFunc func(ctx context.Context, imdbID string) (*downstream.Result, error)

func (f seriesClientFunc) GetActorLatestSeries(ctx context.Context, imdbID string) (*downstream.Result, error) {
	return f(ctx, imdbID)
}

func (f seriesClientFunc) GetActor(ctx context.Context, imdbID string) (*downstream.Result, error) {
	return f(ctx, imdbID)
}

func (f seriesClientFunc) GetActorWorkCounts(ctx context.Context) (*downstream.Result, error) {
	return f(ctx, "")
}

func TestGetActorFallsBackToSeries(t *testing.T) {
	server := newTestServer(&fakeMoviesClient{}, &fakeSeriesClient{})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/actor/nm1", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "nm1") {
		t.Fatalf("got %d %q, want the series-svc actor", rec.Code, rec.Body.String())
	}
}

func TestGetSitemapActorIDsSumsBothServices(t *testing.T) {
	movies := &fakeMoviesClient{counts: &downstream.Result{StatusCode: http.StatusOK,
		Body: []byte(`[{"imdbId":"nm2","works":1},{"imdbId":"nm1","works":1},{"imdbId":"nm3","works":5},{"imdbId":"","works":9}]`)}}
	series := &fakeSeriesClient{counts: &downstream.Result{StatusCode: http.StatusOK,
		Body: []byte(`[{"imdbId":"nm2","works":1},{"imdbId":"nm4","works":1}]`)}}

	rec := httptest.NewRecorder()
	newTestServer(movies, series).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/actors/sitemap-ids", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	// nm3: 5 → first; nm2: 1 movie + 1 series = 2 → in; nm1, nm4: 1 → out; "" → never.
	if got, want := strings.TrimSpace(rec.Body.String()), `["nm3","nm2"]`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestGetSitemapActorIDsFailsWhenAServiceFails(t *testing.T) {
	movies := &fakeMoviesClient{counts: &downstream.Result{StatusCode: http.StatusOK, Body: []byte(`[{"imdbId":"nm3","works":5}]`)}}
	series := &fakeSeriesClient{countsErr: errors.New("boom")}

	rec := httptest.NewRecorder()
	newTestServer(movies, series).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/actors/sitemap-ids", nil))

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502 (no partial list)", rec.Code)
	}
}
