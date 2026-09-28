package telemetry

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Runtime owns the OpenTelemetry providers and their shutdown lifecycle.
type Runtime struct {
	traces  *sdktrace.TracerProvider
	metrics *sdkmetric.MeterProvider
}

// Configure installs process-wide OpenTelemetry providers. Export is enabled when
// OTEL_EXPORTER_OTLP_ENDPOINT or a signal-specific endpoint is configured.
func Configure(ctx context.Context, serviceName string) (*Runtime, error) {
	res := resource.NewWithAttributes("", attribute.String("service.name", serviceName))
	var traces *sdktrace.TracerProvider
	if traceExporterConfigured() {
		traceExporter, err := otlptracehttp.New(ctx)
		if err != nil {
			return nil, err
		}
		traces = sdktrace.NewTracerProvider(sdktrace.WithResource(res), sdktrace.WithBatcher(traceExporter))
	} else {
		traces = sdktrace.NewTracerProvider(sdktrace.WithResource(res))
	}
	runtime := &Runtime{traces: traces}
	var metrics *sdkmetric.MeterProvider
	if metricExporterConfigured() {
		metricExporter, err := otlpmetrichttp.New(ctx)
		if err != nil {
			return nil, errors.Join(err, runtime.Shutdown(ctx))
		}
		metrics = sdkmetric.NewMeterProvider(sdkmetric.WithResource(res), sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(30*time.Second)),
		))
	} else {
		metrics = sdkmetric.NewMeterProvider(sdkmetric.WithResource(res))
	}
	runtime.metrics = metrics

	otel.SetTracerProvider(traces)
	otel.SetMeterProvider(metrics)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return runtime, nil
}

func traceExporterConfigured() bool {
	return nonEmptyEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")
}

func metricExporterConfigured() bool {
	return nonEmptyEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT")
}

func nonEmptyEnv(names ...string) bool {
	for _, name := range names {
		if strings.TrimSpace(os.Getenv(name)) != "" {
			return true
		}
	}
	return false
}

// Shutdown flushes pending telemetry and releases provider resources.
func (r *Runtime) Shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	var errs []error
	if r.traces != nil {
		if err := r.traces.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if r.metrics != nil {
		if err := r.metrics.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
