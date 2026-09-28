package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "github.com/putradwinandap/oke-gaas/internal/platform/http"

type eventProcessingMetrics struct {
	count    metric.Int64Counter
	duration metric.Float64Histogram
	grants   metric.Float64Histogram
}

func newEventProcessingMetrics(meter metric.Meter) eventProcessingMetrics {
	count, countErr := meter.Int64Counter("oke_gaas.event.processing.count")
	duration, durationErr := meter.Float64Histogram("oke_gaas.event.processing.duration", metric.WithUnit("s"))
	grants, grantsErr := meter.Float64Histogram("oke_gaas.event.processing.reward_grants", metric.WithUnit("{grant}"))
	if countErr != nil {
		count = nil
	}
	if durationErr != nil {
		duration = nil
	}
	if grantsErr != nil {
		grants = nil
	}
	return eventProcessingMetrics{count: count, duration: duration, grants: grants}
}

func (m eventProcessingMetrics) record(ctx context.Context, started time.Time, err error, duplicate bool, grants int) {
	attrs := []attribute.KeyValue{
		attribute.String("outcome", "success"),
		attribute.Bool("duplicate", duplicate),
	}
	if err != nil {
		attrs[0] = attribute.String("outcome", "error")
	}
	if m.count != nil {
		m.count.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
	if m.duration != nil {
		m.duration.Record(ctx, time.Since(started).Seconds(), metric.WithAttributes(attrs...))
	}
	if m.grants != nil {
		m.grants.Record(ctx, float64(grants), metric.WithAttributes(attrs...))
	}
}

func telemetryMiddleware(tracer trace.Tracer, meter metric.Meter) fiber.Handler {
	requests, requestsErr := meter.Int64Counter("http.server.requests", metric.WithDescription("Completed HTTP server requests."))
	duration, durationErr := meter.Float64Histogram("http.server.request.duration", metric.WithUnit("s"), metric.WithDescription("HTTP server request duration."))
	if requestsErr != nil {
		requests = nil
	}
	if durationErr != nil {
		duration = nil
	}
	return func(c fiber.Ctx) error {
		started := time.Now()
		carrier := propagation.HeaderCarrier(http.Header{
			"Traceparent": []string{c.Get("traceparent")},
			"Tracestate":  []string{c.Get("tracestate")},
		})
		ctx := otel.GetTextMapPropagator().Extract(c.Context(), carrier)
		ctx, span := tracer.Start(ctx, "http.server.request")
		c.SetContext(ctx)
		err := c.Next()

		status := c.Response().StatusCode()
		if err != nil {
			var fiberErr *fiber.Error
			if errors.As(err, &fiberErr) {
				status = fiberErr.Code
			} else {
				status = fiber.StatusInternalServerError
				span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%T", err)))
			}
		}
		route := c.Route().Path
		if route == "" {
			route = "unmatched"
		}
		attrs := []attribute.KeyValue{
			attribute.String("http.request.method", c.Method()),
			attribute.String("http.route", route),
			attribute.Int("http.response.status_code", status),
		}
		span.SetAttributes(attrs...)
		if status >= fiber.StatusInternalServerError {
			span.SetStatus(codes.Error, "HTTP server error")
		}
		span.End()

		if requests != nil {
			requests.Add(ctx, 1, metric.WithAttributes(attrs...))
		}
		if duration != nil {
			duration.Record(ctx, time.Since(started).Seconds(), metric.WithAttributes(attrs...))
		}
		return err
	}
}
