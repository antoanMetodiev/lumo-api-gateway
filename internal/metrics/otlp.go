// Package metrics pushes Go runtime memory/GC/goroutine metrics to Grafana
// Cloud via OTLP — the direct Go analog of the Java service's
// management.otlp.metrics.export.* setup, reusing the exact same
// GRAFANA_OTLP_ENDPOINT/GRAFANA_OTLP_AUTH env vars (endpoint already
// includes the full /v1/metrics path, same value as configured for the Java
// service) so this shows up in the same Grafana dashboard,
// service_name="lumo-api-gateway", for a direct side-by-side RAM comparison.
package metrics

import (
	"context"
	"log"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

// Start wires up an OTLP/HTTP metrics exporter pushing every 60s (matching
// the Java service's management.otlp.metrics.export.step=60s), returning a
// shutdown func to call on graceful exit. A no-op (nil shutdown, nil error)
// when enabled is false — same as the Java side's export.enabled=false
// default — but unlike a silent no-op, this logs exactly why, so "nothing
// showed up in Grafana" is diagnosable straight from Render's log tab
// instead of guessing.
func Start(ctx context.Context, enabled bool, endpointURL, authHeader string) (func(context.Context) error, error) {
	if !enabled {
		log.Println("metrics: GRAFANA_OTLP_ENABLED is not \"true\" — OTLP export disabled")
		return func(context.Context) error { return nil }, nil
	}
	if endpointURL == "" {
		log.Println("metrics: GRAFANA_OTLP_ENABLED=true but GRAFANA_OTLP_ENDPOINT is empty — OTLP export disabled")
		return func(context.Context) error { return nil }, nil
	}

	// The OTel SDK reports its own internal problems (a failed export, a
	// bad response from the collector, ...) through this handler rather
	// than returning an error anyone here would see — route it to the
	// standard logger so those show up in Render's logs like everything else.
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		log.Printf("metrics: otel error: %v", err)
	}))

	exporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpointURL(endpointURL),
		otlpmetrichttp.WithHeaders(map[string]string{"Authorization": authHeader}),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.Merge(resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName("lumo-api-gateway")))
	if err != nil {
		return nil, err
	}

	provider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(exporter, metric.WithInterval(60*time.Second))),
	)

	if err := runtime.Start(runtime.WithMeterProvider(provider), runtime.WithMinimumReadMemStatsInterval(15*time.Second)); err != nil {
		return nil, err
	}

	log.Printf("metrics: pushing to %s every 60s as service_name=lumo-api-gateway", endpointURL)
	return provider.Shutdown, nil
}
