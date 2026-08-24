# lumo-api-gateway

API gateway for AuroraFlix — rewritten from Java/Spring Boot to Go,
endpoint-for-endpoint and behavior-for-behavior compatible, to compare
real-world RAM usage on Render's free tier (same reason as lumo-movies-svc).

Unlike the other Lumo services, this one has no database of its own — it's a
pure gateway that fans out to six downstream services (lumo-movies-svc,
lumo-series-svc, lumo-movies-generator, lumo-movies-generator-by-actors,
lumo-user-svc, lumo-payments-svc) and is the *only* service that verifies a
Clerk session JWT directly against Clerk's JWKS; every downstream service
instead trusts an `X-Clerk-User-Id` + `X-Internal-Api-Key` header pair this
gateway forwards after doing that verification itself.

## Run locally

```bash
go run ./cmd/server
```

Needs the same environment variables the Java service used: `CLERK_JWKS_URL`,
`CLERK_ISSUER`, `INTERNAL_SERVICE_SECRET`, and optionally `GRAFANA_OTLP_ENABLED`
/ `GRAFANA_OTLP_ENDPOINT` / `GRAFANA_OTLP_AUTH`. The six downstream base URLs
default to the exact production URLs the old `@FeignClient` annotations
hardcoded, but can be overridden for local dev: `MOVIES_SVC_URL`,
`MOVIES_GENERATOR_URL`, `MOVIES_GENERATOR_BY_ACTORS_URL`, `SERIES_SVC_URL`,
`USER_SVC_URL`, `PAYMENTS_SVC_URL`.

## Deploy

`Dockerfile` builds a static binary on a distroless base image — no JDK, no
Gradle. Render picks it up the same way it did the Java Dockerfile.

## Structure

- `cmd/server` — entrypoint, wiring, middleware chain
- `internal/clerkauth` — Clerk JWKS fetch/cache + JWT verification (mirrors
  ClerkJwtService/ClerkAuthenticationFilter)
- `internal/internalauth` — incoming `/internal/**` service-to-service auth
  (mirrors InternalServiceAuthenticationFilter)
- `internal/downstream` — shared outbound HTTP client used by every domain
  package (mirrors FeignConfig's RequestInterceptor)
- `internal/cors` — mirrors SecurityConfig's CORS policy
- `internal/models` — the handful of request DTOs shared between
  internal/movies and internal/series
- `internal/movies`, `internal/series` — the movie/series catalog proxy,
  comments, and reactions (mirrors MovieController/MovieCommentsController/
  TrendingMoviesController and SeriesController)
- `internal/actor` — combines movies + series actor filmography (mirrors
  ActorController)
- `internal/users`, `internal/watchlist`, `internal/lastviewed`,
  `internal/payments` — proxy lumo-user-svc/lumo-payments-svc (mirrors
  UserController/InternalUserController, WatchlistController,
  LastViewedController, PaymentsController/InternalSubscriptionController)
- `internal/heartbeat` — the `/heartbeat` fan-out plus the self-ping ticker
  that keeps Render's free tier from sleeping (mirrors Heartbeat +
  SelfHeartBeatScheduler)
- `internal/reqlog`, `internal/metrics`, `internal/config` — supporting
  pieces, same shape as lumo-movies-svc's own
