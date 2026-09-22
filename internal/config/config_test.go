package config

import "testing"

// clearAllConfigEnvVars ensures each test starts from a clean slate — Load
// reads directly from the process environment, so a var left over from a
// previous test (or the developer's own shell) could otherwise leak in.
func clearAllConfigEnvVars(t *testing.T) {
	t.Helper()
	vars := []string{
		"PORT", "CLERK_JWKS_URL", "CLERK_ISSUER", "INTERNAL_SERVICE_SECRET",
		"GRAFANA_OTLP_ENABLED", "GRAFANA_OTLP_ENDPOINT", "GRAFANA_OTLP_AUTH",
		"MOVIES_SVC_URL", "MOVIES_GENERATOR_URL", "MOVIES_GENERATOR_BY_ACTORS_URL",
		"SERIES_SVC_URL", "USER_SVC_URL", "PAYMENTS_SVC_URL",
	}
	for _, v := range vars {
		t.Setenv(v, "")
	}
}

func TestLoadDefaultsWhenUnset(t *testing.T) {
	clearAllConfigEnvVars(t)

	cfg := Load()

	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want default %q", cfg.Port, "8080")
	}
	if cfg.GrafanaEnabled {
		t.Error("GrafanaEnabled should default to false when GRAFANA_OTLP_ENABLED is unset")
	}
	if cfg.ClerkJWKSURL != "" || cfg.ClerkIssuer != "" || cfg.InternalSecret != "" {
		t.Errorf("expected all no-default env vars to be empty, got %+v", cfg)
	}

	wantDefaults := map[string]string{
		"MoviesSvcURL":               "https://lumo-movies-svc.onrender.com",
		"MoviesGeneratorURL":         "https://lumo-movies-generator.onrender.com",
		"MoviesGeneratorByActorsURL": "https://new-lumo-movies-generator-by-actors.onrender.com",
		"SeriesSvcURL":               "https://lumo-series-svc.onrender.com",
		"UserSvcURL":                 "https://lumo-new-user-svc.onrender.com",
		"PaymentsSvcURL":             "https://lumo-payments-svc.onrender.com",
		"SelfURL":                    "https://lumo-api-gateway.onrender.com",
		"SubtitlesTakerURL":          "https://lumo-movies-subtitles-taker-svc.onrender.com",
	}
	got := map[string]string{
		"MoviesSvcURL":               cfg.MoviesSvcURL,
		"MoviesGeneratorURL":         cfg.MoviesGeneratorURL,
		"MoviesGeneratorByActorsURL": cfg.MoviesGeneratorByActorsURL,
		"SeriesSvcURL":               cfg.SeriesSvcURL,
		"UserSvcURL":                 cfg.UserSvcURL,
		"PaymentsSvcURL":             cfg.PaymentsSvcURL,
		"SelfURL":                    cfg.SelfURL,
		"SubtitlesTakerURL":          cfg.SubtitlesTakerURL,
	}
	for field, want := range wantDefaults {
		if got[field] != want {
			t.Errorf("%s = %q, want default %q", field, got[field], want)
		}
	}
}

func TestLoadReadsPlainEnvVarsThrough(t *testing.T) {
	clearAllConfigEnvVars(t)
	t.Setenv("PORT", "9090")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/.well-known/jwks.json")
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
	t.Setenv("INTERNAL_SERVICE_SECRET", "shhh")
	t.Setenv("MOVIES_SVC_URL", "http://localhost:8081")

	cfg := Load()

	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want the overridden value", cfg.Port)
	}
	if cfg.ClerkJWKSURL != "https://clerk.example.com/.well-known/jwks.json" {
		t.Errorf("ClerkJWKSURL = %q", cfg.ClerkJWKSURL)
	}
	if cfg.ClerkIssuer != "https://clerk.example.com" {
		t.Errorf("ClerkIssuer = %q", cfg.ClerkIssuer)
	}
	if cfg.InternalSecret != "shhh" {
		t.Errorf("InternalSecret = %q", cfg.InternalSecret)
	}
	if cfg.MoviesSvcURL != "http://localhost:8081" {
		t.Errorf("MoviesSvcURL = %q, want the overridden local dev value", cfg.MoviesSvcURL)
	}
}

// TestLoadGrafanaEnabledOnlyOnExactStringTrue checks the strict
// os.Getenv(...) == "true" comparison — anything else must NOT enable it.
func TestLoadGrafanaEnabledOnlyOnExactStringTrue(t *testing.T) {
	cases := []struct {
		value string
		want  bool
	}{
		{"true", true},
		{"TRUE", false},
		{"1", false},
		{"yes", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			clearAllConfigEnvVars(t)
			t.Setenv("GRAFANA_OTLP_ENABLED", tc.value)
			cfg := Load()
			if cfg.GrafanaEnabled != tc.want {
				t.Errorf("GRAFANA_OTLP_ENABLED=%q -> GrafanaEnabled = %v, want %v", tc.value, cfg.GrafanaEnabled, tc.want)
			}
		})
	}
}

func TestEnvOr(t *testing.T) {
	t.Setenv("GATEWAY_CONFIG_TEST_VAR", "")
	if got := envOr("GATEWAY_CONFIG_TEST_VAR", "fallback"); got != "fallback" {
		t.Errorf("envOr with unset var = %q, want fallback", got)
	}

	t.Setenv("GATEWAY_CONFIG_TEST_VAR", "set-value")
	if got := envOr("GATEWAY_CONFIG_TEST_VAR", "fallback"); got != "set-value" {
		t.Errorf("envOr with set var = %q, want the set value", got)
	}
}
