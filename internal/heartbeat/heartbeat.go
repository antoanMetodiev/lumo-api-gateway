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
