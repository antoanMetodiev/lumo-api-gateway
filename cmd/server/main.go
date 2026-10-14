// Command server is the Go rewrite of lumo-api-gateway (originally Spring
// Boot). Endpoint paths, params, status codes, and downstream call shapes
// all mirror the Java service exactly — see each internal/ package for the
// per-package notes on where behavior was intentionally preserved
// (including a couple of the original's own quirks, like the
// movies/series asymmetry in blank-input handling) vs. where a Java-only
// implementation detail (Feign's typed exceptions, Spring's default
// unhandled-exception response) was replaced with a plain, equivalent
// status-code check.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lumo-api-gateway/internal/actor"
	"lumo-api-gateway/internal/aiassistant"
	"lumo-api-gateway/internal/clerkauth"
	"lumo-api-gateway/internal/config"
	"lumo-api-gateway/internal/cors"
	"lumo-api-gateway/internal/downstream"
	"lumo-api-gateway/internal/friends"
	"lumo-api-gateway/internal/heartbeat"
	"lumo-api-gateway/internal/internalauth"
	"lumo-api-gateway/internal/lastviewed"
	"lumo-api-gateway/internal/likes"
	"lumo-api-gateway/internal/metrics"
	"lumo-api-gateway/internal/movies"
	"lumo-api-gateway/internal/payments"
	"lumo-api-gateway/internal/playlists"
	"lumo-api-gateway/internal/reqlog"
	"lumo-api-gateway/internal/series"
	"lumo-api-gateway/internal/users"
	"lumo-api-gateway/internal/watching"
	"lumo-api-gateway/internal/watchlist"
	"lumo-api-gateway/internal/wsnotify"
)

// clerkProtectedPaths mirrors SecurityConfig.PROTECTED_PATHS exactly, plus
// "/watching" — a new-feature addition with no Java original (see
// lumo-user-svc's watching package doc comment).
var clerkProtectedPaths = []string{
	"/user", "/watchlist", "/last-viewed", "/payments", "/friends", "/likes", "/playlists", "/watching", "/ai-assistant",
	"/post-movie-comment", "/update-movie-comment", "/delete-movie-comment", "/react-movie-comment",
	"/post-series-comment", "/update-series-comment", "/delete-series-comment", "/react-series-comment",
}

// internalProtectedPaths mirrors SecurityConfig.INTERNAL_PATHS exactly.
var internalProtectedPaths = []string{"/internal"}

func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownMetrics, err := metrics.Start(ctx, cfg.GrafanaEnabled, cfg.GrafanaEndpoint, cfg.GrafanaAuth)
	if err != nil {
		log.Printf("metrics exporter did not start: %v", err)
		shutdownMetrics = func(context.Context) error { return nil }
	}
	defer shutdownMetrics(context.Background())

	ds := downstream.New(cfg.InternalSecret)

	moviesClient := movies.NewClient(ds, cfg.MoviesSvcURL, cfg.MoviesGeneratorURL, cfg.MoviesGeneratorByActorsURL)
	seriesClient := series.NewClient(ds, cfg.SeriesSvcURL)
	usersClient := users.NewClient(ds, cfg.UserSvcURL)
	watchlistClient := watchlist.NewClient(ds, cfg.UserSvcURL)
	lastViewedClient := lastviewed.NewClient(ds, cfg.UserSvcURL)
	likesClient := likes.NewClient(ds, cfg.UserSvcURL)
	friendsClient := friends.NewClient(ds, cfg.UserSvcURL)
	playlistsClient := playlists.NewClient(ds, cfg.UserSvcURL)
	watchingClient := watching.NewClient(ds, cfg.UserSvcURL)
	aiAssistantClient := aiassistant.NewClient(ds, cfg.UserSvcURL)
	paymentsClient := payments.NewClient(ds, cfg.PaymentsSvcURL)

	verifier := clerkauth.New(cfg.ClerkJWKSURL, cfg.ClerkIssuer)
	wsHub := wsnotify.NewHub(verifier)

	mux := http.NewServeMux()
	movies.NewHandlers(moviesClient, usersClient).Register(mux)
	series.NewHandlers(seriesClient, usersClient).Register(mux)
	actor.NewHandlers(moviesClient, seriesClient).Register(mux)
	users.NewHandlers(usersClient).Register(mux)
	watchlist.NewHandlers(watchlistClient).Register(mux)
	lastviewed.NewHandlers(lastViewedClient).Register(mux)
	likes.NewHandlers(likesClient).Register(mux)
	friends.NewHandlers(friendsClient).Register(mux)
	playlists.NewHandlers(playlistsClient).Register(mux)
	watching.NewHandlers(watchingClient).Register(mux)
	aiassistant.NewHandlers(aiAssistantClient).Register(mux)
	wsHub.Register(mux)
	payments.NewHandlers(paymentsClient).Register(mux)
	heartbeat.NewHandlers(
		func(ctx context.Context) error { _, err := moviesClient.Heartbeat(ctx); return err },
		func(ctx context.Context) error { _, err := seriesClient.Heartbeat(ctx); return err },
		func(ctx context.Context) error { _, err := usersClient.Heartbeat(ctx); return err },
		func(ctx context.Context) error { _, err := paymentsClient.Heartbeat(ctx); return err },
	).Register(mux)

	// management.endpoints.web.exposure.include=health — Render's own health
	// check and any uptime monitor hit this.
	mux.HandleFunc("GET /actuator/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"UP"}`))
	})
	// Plain alias of the above, same as every downstream service exposes —
	// this is what StartKeepAlive's self-ping hits.
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"UP"}`))
	})

	handler := reqlog.Middleware(
		cors.Middleware(
			clerkauth.Middleware(verifier, clerkProtectedPaths)(
				internalauth.Middleware(cfg.InternalSecret, internalProtectedPaths)(mux),
			),
		),
	)

	// The one scheduled keep-alive job for the whole fleet — every service
	// (this gateway included, via its own public URL) every 2 minutes, and
	// every distinct database every 10 hours. Independent of the /heartbeat
	// fan-out above, which only reaches four services and only when
	// something external hits it. See StartKeepAlive's doc comment for why
	// the database list is per-database rather than per-service.
	go heartbeat.StartKeepAlive(ctx,
		map[string]string{
			"api-gateway":                cfg.SelfURL + "/health",
			"movies-svc":                 cfg.MoviesSvcURL + "/health",
			"movies-generator":           cfg.MoviesGeneratorURL + "/health",
			"movies-generator-by-actors": cfg.MoviesGeneratorByActorsURL + "/health",
			"series-svc":                 cfg.SeriesSvcURL + "/health",
			"user-svc":                   cfg.UserSvcURL + "/health",
			"payments-svc":               cfg.PaymentsSvcURL + "/health",
			"subtitles-taker-svc":        cfg.SubtitlesTakerURL + "/health",
			"series-subtitles-taker-svc": cfg.SeriesSubtitlesTakerURL + "/health",
		},
		map[string]string{
			"movies-db":           cfg.MoviesSvcURL + "/db-keepalive",
			"series-db":           cfg.SeriesSvcURL + "/db-keepalive",
			"user-db":             cfg.UserSvcURL + "/db-keepalive",
			"payments-db":         cfg.PaymentsSvcURL + "/db-keepalive",
			"subtitles-db":        cfg.SubtitlesTakerURL + "/db-keepalive",
			"series-subtitles-db": cfg.SeriesSubtitlesTakerURL + "/db-keepalive",
		},
		// The homepage hero's two lists. Called straight on the owning
		// service rather than through this gateway's own routes: only the
		// read side is proxied here, and both regenerate endpoints are
		// public on their service (outside its protectedPaths), same as the
		// /health and /db-keepalive above.
		[]heartbeat.TrendingTarget{
			{
				Name:       "trending-movies",
				ListURL:    cfg.MoviesSvcURL + "/get-trending-movies",
				RefreshURL: cfg.MoviesSvcURL + "/find-new-trending-movies",
			},
			{
				Name:       "trending-series",
				ListURL:    cfg.SeriesSvcURL + "/get-trending-series",
				RefreshURL: cfg.SeriesSvcURL + "/find-new-trending-series",
			},
		},
	)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("lumo-api-gateway (Go) listening on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}
