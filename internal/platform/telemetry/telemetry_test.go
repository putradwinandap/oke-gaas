package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
)

func TestExporterConfigurationUsesGlobalAndSignalSpecificEndpoints(t *testing.T) {
	for _, name := range []string{
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
	} {
		t.Setenv(name, "")
	}
	if traceExporterConfigured() || metricExporterConfigured() {
		t.Fatal("exporters should be disabled when no endpoint is configured")
	}

	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "http://traces:4318")
	if !traceExporterConfigured() || metricExporterConfigured() {
		t.Fatal("trace-specific endpoint should enable only trace export")
	}

	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "http://metrics:4318")
	if traceExporterConfigured() || !metricExporterConfigured() {
		t.Fatal("metric-specific endpoint should enable only metric export")
	}

	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://collector:4318")
	if !traceExporterConfigured() || !metricExporterConfigured() {
		t.Fatal("global endpoint should enable both exporters")
	}
}

func TestConfigureWithoutEndpointSupportsCleanShutdownAndIgnoresBaggage(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")

	runtime, err := Configure(context.Background(), "test-service")
	if err != nil {
		t.Fatalf("configure telemetry: %v", err)
	}
	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown telemetry: %v", err)
	}

	ctx := otel.GetTextMapPropagator().Extract(context.Background(), propagation.MapCarrier{
		"baggage": "player.id=private-value",
	})
	if baggage.FromContext(ctx).Len() != 0 {
		t.Fatal("untrusted baggage must not be propagated")
	}
}
