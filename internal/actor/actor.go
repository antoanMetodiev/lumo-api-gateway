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
	"sort"

	"lumo-api-gateway/internal/downstream"
)

type MoviesClient interface {
	GetActorLatestMovies(ctx context.Context, imdbID string) (*downstream.Result, error)
	GetActor(ctx context.Context, imdbID string) (*downstream.Result, error)
	GetActorWorkCounts(ctx context.Context) (*downstream.Result, error)
}

type SeriesClient interface {
	GetActorLatestSeries(ctx context.Context, imdbID string) (*downstream.Result, error)
	GetActor(ctx context.Context, imdbID string) (*downstream.Result, error)
	GetActorWorkCounts(ctx context.Context) (*downstream.Result, error)
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
	mux.HandleFunc("GET /actor/{imdb_id}", h.getActor)
	mux.HandleFunc("GET /actors/sitemap-ids", h.getSitemapActorIDs)
}

// SitemapMinWorks: an actor goes into the frontend's sitemap only with at
// least this many titles (movies + series). Their page is indexable from 2
// titles, or 1 + a biography — the sitemap takes the clear-cut half; the
// rest are still reachable through the cast links.
const SitemapMinWorks = 2

type actorWorkCount struct {
	ImdbID string `json:"imdbId"`
	Works  int    `json:"works"`
}

// getSitemapActorIDs returns the IMDb ids of actors cast in at least
// SitemapMinWorks titles across both databases — most titles first (ties by
// id), so a sitemap that has to cut the list keeps the best-known actors. The movies and
// series services each count only their own titles, so the counts are
// summed here. Either call failing fails the request (502) — a partial
// list would make the sitemap drop actors for no reason.
func (h *Handlers) getSitemapActorIDs(w http.ResponseWriter, r *http.Request) {
	totals := map[string]int{}
	for _, call := range []struct {
		svc string
		get func(context.Context) (*downstream.Result, error)
	}{{"movies", h.movies.GetActorWorkCounts}, {"series", h.series.GetActorWorkCounts}} {
		res, err := call.get(r.Context())
		if err != nil || !res.OK() {
			logActorError(call.svc, "sitemap-ids", res, err)
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		var counts []actorWorkCount
		if err := json.Unmarshal(res.Body, &counts); err != nil {
			log.Printf("GET /actors/sitemap-ids: decoding %s-svc response: %v", call.svc, err)
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		for _, c := range counts {
			if c.ImdbID != "" {
				totals[c.ImdbID] += c.Works
			}
		}
	}

	ids := make([]string, 0, len(totals))
	for id, works := range totals {
		if works >= SitemapMinWorks {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		if totals[ids[i]] != totals[ids[j]] {
			return totals[ids[i]] > totals[ids[j]]
		}
		return ids[i] < ids[j]
	})
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(ids); err != nil {
		log.Printf("GET /actors/sitemap-ids: %v", err)
	}
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

// getActor serves one actor (translated on ?lang, which the downstream
// middleware forwards). The movies and series databases each keep their own
// actors, so movies-svc is asked first and series-svc when it has no such actor.
func (h *Handlers) getActor(w http.ResponseWriter, r *http.Request) {
	imdbID := r.PathValue("imdb_id")

	res, err := h.movies.GetActor(r.Context(), imdbID)
	if err == nil && res.StatusCode == http.StatusNotFound {
		res, err = h.series.GetActor(r.Context(), imdbID)
	}
	if err != nil {
		log.Printf("GET /actor/%s: %v", imdbID, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	downstream.WriteProxy(w, res)
}
