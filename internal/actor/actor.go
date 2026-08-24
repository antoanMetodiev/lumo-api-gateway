// Package actor mirrors ActorController — the one endpoint that combines
// results from both lumo-movies-svc and lumo-series-svc. Both downstream
// responses are already-Go JSON arrays of the ActorLatestWorks shape, so
// this only needs to concatenate the raw array elements, never decode their
// fields.
package actor

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"lumo-api-gateway/internal/downstream"
)

type MoviesClient interface {
	GetActorLatestMovies(ctx context.Context, imdbID string) (*downstream.Result, error)
}

type SeriesClient interface {
	GetActorLatestSeries(ctx context.Context, imdbID string) (*downstream.Result, error)
}

type Handlers struct {
	movies MoviesClient
	series SeriesClient
}

func NewHandlers(movies MoviesClient, series SeriesClient) *Handlers {
	return &Handlers{movies: movies, series: series}
}

func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /actor/latest-works/{imdb_id}", h.getActorLatestWorks)
}

// getActorLatestWorks mirrors ActorController.getActorLatestWorks:
// movieClient.getActorLatestMovies, then seriesClient.getActorLatestSeries,
// then actorLatestMovies.addAll(actorLatestSeries) — called sequentially, so
// a failure on either call aborts the whole request with no partial result,
// same as the uncaught-exception default (500) the Java controller falls
// back to since it has no try/catch of its own.
func (h *Handlers) getActorLatestWorks(w http.ResponseWriter, r *http.Request) {
	imdbID := r.PathValue("imdb_id")

	moviesRes, err := h.movies.GetActorLatestMovies(r.Context(), imdbID)
	if err != nil || !moviesRes.OK() {
		logActorError("movies", imdbID, moviesRes, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	seriesRes, err := h.series.GetActorLatestSeries(r.Context(), imdbID)
	if err != nil || !seriesRes.OK() {
		logActorError("series", imdbID, seriesRes, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var movieWorks, seriesWorks []json.RawMessage
	if err := json.Unmarshal(moviesRes.Body, &movieWorks); err != nil {
		log.Printf("GET /actor/latest-works/%s: decoding movies-svc response: %v", imdbID, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := json.Unmarshal(seriesRes.Body, &seriesWorks); err != nil {
		log.Printf("GET /actor/latest-works/%s: decoding series-svc response: %v", imdbID, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	combined := append(movieWorks, seriesWorks...)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(combined); err != nil {
		log.Printf("GET /actor/latest-works/%s: %v", imdbID, err)
	}
}

func logActorError(svc, imdbID string, res *downstream.Result, err error) {
	if err != nil {
		log.Printf("GET /actor/latest-works/%s: %s call failed: %v", imdbID, svc, err)
		return
	}
	log.Printf("GET /actor/latest-works/%s: %s call failed: status %d", imdbID, svc, res.StatusCode)
}
