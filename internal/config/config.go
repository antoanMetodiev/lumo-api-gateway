// Package config reads the exact same environment variables the Java service
// used (application.properties + the hardcoded @FeignClient URLs), so this
// drop-in replacement needs zero changes to whatever's already set on Render.
package config

import "os"

type Config struct {
	Port string // server.port=8080

	ClerkJWKSURL string // clerk.jwks-url
	ClerkIssuer  string // clerk.issuer

	// internal.service-secret — stamped on every outgoing downstream call
	// (see internal/downstream) and required on every incoming /internal/**
	// call (see internal/internalauth). Shared with lumo-user-svc and
	// lumo-payments-svc.
	InternalSecret string

	GrafanaEnabled  bool
	GrafanaEndpoint string
	GrafanaAuth     string

	// Downstream base URLs — default to the exact URLs the Java
	// @FeignClient annotations hardcoded, overridable for local dev the
	// same way those interfaces had a commented-out localhost alternative.
	MoviesSvcURL               string
	MoviesGeneratorURL         string
	MoviesGeneratorByActorsURL string
	SeriesSvcURL               string
	UserSvcURL                 string
	PaymentsSvcURL             string
}

func Load() Config {
	return Config{
		Port: envOr("PORT", "8080"),

		ClerkJWKSURL: os.Getenv("CLERK_JWKS_URL"),
		ClerkIssuer:  os.Getenv("CLERK_ISSUER"),

		InternalSecret: os.Getenv("INTERNAL_SERVICE_SECRET"),

		GrafanaEnabled:  os.Getenv("GRAFANA_OTLP_ENABLED") == "true",
		GrafanaEndpoint: os.Getenv("GRAFANA_OTLP_ENDPOINT"),
		GrafanaAuth:     os.Getenv("GRAFANA_OTLP_AUTH"),

		MoviesSvcURL:               envOr("MOVIES_SVC_URL", "https://lumo-movies-svc.onrender.com "), //  http://localhost:8081
		MoviesGeneratorURL:         envOr("MOVIES_GENERATOR_URL", "https://lumo-movies-generator.onrender.com"),
		MoviesGeneratorByActorsURL: envOr("MOVIES_GENERATOR_BY_ACTORS_URL", "https://new-lumo-movies-generator-by-actors.onrender.com"),
		SeriesSvcURL:               envOr("SERIES_SVC_URL", "https://lumo-series-svc.onrender.com"),
		UserSvcURL:                 envOr("USER_SVC_URL", "https://lumo-new-user-svc.onrender.com"),
		PaymentsSvcURL:             envOr("PAYMENTS_SVC_URL", "https://lumo-payments-svc.onrender.com"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
