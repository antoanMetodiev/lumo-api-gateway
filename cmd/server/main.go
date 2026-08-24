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
	"lumo-api-gateway/internal/clerkauth"
	"lumo-api-gateway/internal/config"
	"lumo-api-gateway/internal/cors"
	"lumo-api-gateway/internal/downstream"
	"lumo-api-gateway/internal/heartbeat"
	"lumo-api-gateway/internal/internalauth"
	"lumo-api-gateway/internal/lastviewed"
	"lumo-api-gateway/internal/metrics"
	"lumo-api-gateway/internal/movies"
	"lumo-api-gateway/internal/payments"
	"lumo-api-gateway/internal/reqlog"
	"lumo-api-gateway/internal/series"
	"lumo-api-gateway/internal/users"
	"lumo-api-gateway/internal/watchlist"
)

// clerkProtectedPaths mirrors SecurityConfig.PROTECTED_PATHS exactly.
var clerkProtectedPaths = []string{
	"/user", "/watchlist", "/last-viewed", "/payments",
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
	paymentsClient := payments.NewClient(ds, cfg.PaymentsSvcURL)

	verifier := clerkauth.New(cfg.ClerkJWKSURL, cfg.ClerkIssuer)

	mux := http.NewServeMux()
	movies.NewHandlers(moviesClient, usersClient).Register(mux)
	series.NewHandlers(seriesClient, usersClient).Register(mux)
	actor.NewHandlers(moviesClient, seriesClient).Register(mux)
	users.NewHandlers(usersClient).Register(mux)
	watchlist.NewHandlers(watchlistClient).Register(mux)
	lastviewed.NewHandlers(lastViewedClient).Register(mux)
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

	handler := reqlog.Middleware(
		cors.Middleware(
			clerkauth.Middleware(verifier, clerkProtectedPaths)(
				internalauth.Middleware(cfg.InternalSecret, internalProtectedPaths)(mux),
			),
		),
	)

	go heartbeat.StartSelfPing(ctx)

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
