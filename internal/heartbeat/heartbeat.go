// Package heartbeat mirrors Heartbeat (the GET /heartbeat fan-out) +
// SelfHeartBeatScheduler (the self-ping ticker).
package heartbeat

import (
	"context"
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

// fleetPingInterval is deliberately shorter than StartSelfPing's 4 minutes:
// this pings six services instead of one, and a couple of them (the two
// movies-generator services) aren't reachable through the existing
// /heartbeat fan-out at all (Handlers only fans out to four of the six
// downstream services the gateway knows about — movies-svc, series-svc,
// user-svc, payments-svc — missing lumo-movies-generator and
// lumo-movies-generator-by-actors, which never get a keep-alive from
// anywhere today). 2 minutes comfortably beats Render's free-tier ~15
// minute idle window with margin for one or two missed/slow ticks.
const fleetPingInterval = 2 * time.Minute

// StartFleetHealthPing is a second, independent keep-alive from
// StartSelfPing: rather than pinging this gateway's own /heartbeat (which
// then fans out to a subset of downstream services), it directly GETs
// /health on every service named in urls, every fleetPingInterval, so the
// full fleet — the two movies-generator services included — never goes
// idle long enough for Render's free tier to put it to sleep. Runs until
// ctx is canceled.
func StartFleetHealthPing(ctx context.Context, urls map[string]string) {
	client := &http.Client{Timeout: 30 * time.Second}

	ticker := time.NewTicker(fleetPingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for name, base := range urls {
				pingOne(ctx, client, "fleet health ping", name, base+"/health")
			}
		}
	}
}

// dbPingInterval is much coarser than fleetPingInterval on purpose: this
// only exists so each service's own Postgres connection sees regular
// traffic and doesn't get treated as idle by Aiven, not to keep the Render
// instance itself awake (StartFleetHealthPing's /health already does that
// far more often). Movies-svc, movies-generator and
// movies-generator-by-actors all share one Aiven database, so pinging
// movies-svc's /db-keepalive alone already covers it — no need to hit all
// three separately for the same database.
const dbPingInterval = 8 * time.Hour

// StartFleetDBKeepAlive GETs /db-keepalive on every service named in urls
// every dbPingInterval — one entry per distinct database, not per service
// (see dbPingInterval's doc comment). Each target's own /db-keepalive
// handler (lumo-movies-svc, lumo-series-svc, lumo-user-svc,
// lumo-payments-svc) runs a trivial `SELECT 1` against its pool when hit.
// Runs until ctx is canceled.
func StartFleetDBKeepAlive(ctx context.Context, urls map[string]string) {
	client := &http.Client{Timeout: 30 * time.Second}

	ticker := time.NewTicker(dbPingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for name, base := range urls {
				pingOne(ctx, client, "fleet db keepalive", name, base+"/db-keepalive")
			}
		}
	}
}

// pingOne fires one fire-and-forget GET to url — same detached-context,
// panic-recovering shape as fireAndForget above, so one slow or dead target
// can never delay or fail the ping to any other, and a failure here just
// gets logged, never surfaced anywhere else. label distinguishes the two
// callers (StartFleetHealthPing vs StartFleetDBKeepAlive) in the log line.
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

// StartSelfPing mirrors SelfHeartBeatScheduler.heartBeatSelf: pings this
// gateway's own public /heartbeat endpoint every 4 minutes so Render's free
// tier never lets it go idle long enough to sleep. Runs until ctx is canceled.
func StartSelfPing(ctx context.Context) {
	const selfURL = "https://lumo-api-gateway.onrender.com/heartbeat"
	client := &http.Client{Timeout: 30 * time.Second}

	ticker := time.NewTicker(240 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Println("=====>>>>>>> Receive SELF-HEARTBEAT...!!!")
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, selfURL, nil)
			if err != nil {
				continue
			}
			resp, err := client.Do(req)
			if err != nil {
				log.Printf("Ping failed: %v", err)
				continue
			}
			resp.Body.Close()
		}
	}
}
