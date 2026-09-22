// Package heartbeat mirrors Heartbeat (the GET /heartbeat fan-out) and
// replaces SelfHeartBeatScheduler with StartKeepAlive — one scheduled job
// that keeps every service (2 min) and every database (10 h) in the fleet
// warm on Render’s free tier / Aiven, and regenerates the homepage’s
// trending lists once a day (see trendingCheckInterval).
package heartbeat

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// Handlers fans this gateway's own self-ping out to every downstream
// service, so lumo-user-svc and lumo-payments-svc stay warm too — Render's
// free tier puts an idle service to sleep after ~15 minutes, and a cold
// Spring Boot boot can easily take 30-60+ seconds, which used to blow
// straight through CheckoutService's post-checkout polling timeout and show
// a real successful purchase as "failed" on the frontend.
type Handlers struct {
	movies   func(context.Context) error
	series   func(context.Context) error
	users    func(context.Context) error
	payments func(context.Context) error
}

func NewHandlers(movies, series, users, payments func(context.Context) error) *Handlers {
	return &Handlers{movies: movies, series: series, users: users, payments: payments}
}

func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /heartbeat", h.heartbeat)
}

// heartbeat mirrors Heartbeat.heartbeat: fires all four downstream calls as
// fire-and-forget goroutines (Thread.ofVirtual().start(...)) and returns
// immediately — it never waits for them, so a slow/dead downstream service
// can't make this endpoint itself slow.
func (h *Handlers) heartbeat(w http.ResponseWriter, r *http.Request) {
	fireAndForget("movies", h.movies)
	fireAndForget("series", h.series)
	fireAndForget("users", h.users)
	fireAndForget("payments", h.payments)
	w.WriteHeader(http.StatusOK)
}

func fireAndForget(name string, call func(context.Context) error) {
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("heartbeat: %s panic: %v", name, rec)
			}
		}()
		// Detached from the incoming request's context on purpose — the
		// point is to keep the downstream service warm regardless of
		// whether the triggering request is still in flight.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := call(ctx); err != nil {
			log.Printf("heartbeat: %s failed: %v", name, err)
		}
	}()
}

// servicePingInterval is how often every service in the fleet (this gateway
// included) gets a GET /health. Render's free tier puts an idle instance to
// sleep after ~15 minutes; 2 minutes beats that with margin for a couple of
// missed or slow ticks. The two movies-generator services and
// lumo-movies-subtitles-taker-svc are only kept warm by this — nothing else
// ever calls them on a schedule.
const servicePingInterval = 2 * time.Minute

// dbPingInterval is deliberately coarse: this only exists so each service's
// own Postgres connection sees regular traffic and doesn't get treated as
// idle by Aiven, not to keep the Render instance awake (the service tick
// above already does that far more often). One target per distinct
// database, not per service — movies-svc, movies-generator and
// movies-generator-by-actors all share one Aiven database, so
// movies-svc's /db-keepalive alone covers it.
const dbPingInterval = 10 * time.Hour

// trendingMaxAge is how stale the homepage's trending lists are allowed to
// get before they're regenerated from TMDB.
const trendingMaxAge = 24 * time.Hour

// trendingCheckInterval is how often that age is checked — deliberately much
// shorter than trendingMaxAge itself. A plain 24h ticker would restart its
// countdown on every deploy or Render restart, so a service that gets
// redeployed most days would regenerate rarely or never. Checking hourly and
// deciding from the data's own timestamp makes the schedule survive
// restarts: whenever the fleet comes back up, the next check catches up.
const trendingCheckInterval = time.Hour

// trendingRefreshTimeout is generous because GenerateTrending{Movies,Series}
// walks 10+ titles with several sequential TMDB calls each, and pulls any
// missing one into the catalog on the way — comfortably minutes, not seconds.
const trendingRefreshTimeout = 10 * time.Minute

// TrendingTarget is one catalog's trending list: ListURL serves the current
// rows (whose newest createdAt is the freshness signal) and RefreshURL
// regenerates them. Both are full URLs.
type TrendingTarget struct {
	Name       string
	ListURL    string
	RefreshURL string
}

// StartKeepAlive is the gateway's single scheduled keep-alive job: every
// servicePingInterval it GETs each URL in services, and every
// dbPingInterval it GETs each URL in databases. Both maps hold full URLs,
// not base URLs — each target's own /db-keepalive (lumo-movies-svc,
// lumo-series-svc, lumo-user-svc, lumo-payments-svc) runs a trivial
// `SELECT 1` against its pool when hit, while
// lumo-movies-subtitles-taker-svc pings its database from /health instead,
// so that's the URL it's registered under. Every ping is fire-and-forget
// (see pingOne), so one slow or dead target never delays the rest of the
// tick. Replaces the three separate tickers this used to run (self-ping,
// fleet /health, fleet /db-keepalive).
//
// Each target in `trending` is additionally checked on its own cadence and
// regenerated when its data has gone stale — see trendingCheckInterval and
// refreshTrendingIfStale. Runs until ctx is canceled.
func StartKeepAlive(ctx context.Context, services, databases map[string]string, trending []TrendingTarget) {
	client := &http.Client{Timeout: 30 * time.Second}

	serviceTicker := time.NewTicker(servicePingInterval)
	defer serviceTicker.Stop()
	dbTicker := time.NewTicker(dbPingInterval)
	defer dbTicker.Stop()
	trendingTicker := time.NewTicker(trendingCheckInterval)
	defer trendingTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-serviceTicker.C:
			for name, url := range services {
				pingOne(ctx, client, "service keep-alive", name, url)
			}
		case <-dbTicker.C:
			for name, url := range databases {
				pingOne(ctx, client, "db keep-alive", name, url)
			}
		case <-trendingTicker.C:
			for _, target := range trending {
				refreshTrendingIfStale(ctx, target)
			}
		}
	}
}

// refreshTrendingIfStale reads one trending list, and if its newest row is
// older than trendingMaxAge (or there are no rows at all — an empty table
// right after the migration, or every candidate rejected last run), POSTs
// the regenerate endpoint. Runs detached and panic-guarded like pingOne, so
// a refresh that takes minutes never holds up the keep-alive ticks.
//
// Deliberately fire-and-forget with no retry: if a run fails, the list stays
// stale and the next hourly check simply tries again.
func refreshTrendingIfStale(parent context.Context, target TrendingTarget) {
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("trending refresh: %s panic: %v", target.Name, rec)
			}
		}()

		ctx := context.WithoutCancel(parent)

		age, ok := newestTrendingAge(ctx, target)
		if ok && age < trendingMaxAge {
			return
		}

		log.Printf("trending refresh: %s is stale, regenerating", target.Name)

		refreshCtx, cancel := context.WithTimeout(ctx, trendingRefreshTimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(refreshCtx, http.MethodPost, target.RefreshURL, nil)
		if err != nil {
			log.Printf("trending refresh: %s: building request failed: %v", target.Name, err)
			return
		}
		resp, err := (&http.Client{Timeout: trendingRefreshTimeout}).Do(req)
		if err != nil {
			log.Printf("trending refresh: %s failed: %v", target.Name, err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			log.Printf("trending refresh: %s returned status %d", target.Name, resp.StatusCode)
			return
		}
		log.Printf("trending refresh: %s regenerated", target.Name)
	}()
}

// newestTrendingAge returns how long ago the freshest row in a trending list
// was created. ok is false when that can't be determined — an unreachable
// service, a non-200, unparseable JSON, or an empty list — which the caller
// treats as "refresh it". Only createdAt is decoded; the rest of each row is
// irrelevant here, and the two services' trending shapes differ elsewhere.
func newestTrendingAge(ctx context.Context, target TrendingTarget) (time.Duration, bool) {
	listCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(listCtx, http.MethodGet, target.ListURL, nil)
	if err != nil {
		log.Printf("trending refresh: %s: building list request failed: %v", target.Name, err)
		return 0, false
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		log.Printf("trending refresh: %s: list fetch failed: %v", target.Name, err)
		return 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("trending refresh: %s: list returned status %d", target.Name, resp.StatusCode)
		return 0, false
	}

	var rows []struct {
		CreatedAt time.Time `json:"createdAt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		log.Printf("trending refresh: %s: decoding list failed: %v", target.Name, err)
		return 0, false
	}
	if len(rows) == 0 {
		return 0, false
	}

	newest := rows[0].CreatedAt
	for _, row := range rows[1:] {
		if row.CreatedAt.After(newest) {
			newest = row.CreatedAt
		}
	}
	if newest.IsZero() {
		return 0, false
	}
	return time.Since(newest), true
}

// pingOne fires one fire-and-forget GET to url — same detached-context,
// panic-recovering shape as fireAndForget above, so one slow or dead target
// can never delay or fail the ping to any other, and a failure here just
// gets logged, never surfaced anywhere else. label distinguishes the two
// tickers in StartKeepAlive in the log line.
func pingOne(parent context.Context, client *http.Client, label, name, url string) {
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("%s: %s panic: %v", label, name, rec)
			}
		}()

		ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 30*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			log.Printf("%s: %s: building request failed: %v", label, name, err)
			return
		}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("%s: %s failed: %v", label, name, err)
			return
		}
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			log.Printf("%s: %s returned status %d", label, name, resp.StatusCode)
		}
	}()
}
