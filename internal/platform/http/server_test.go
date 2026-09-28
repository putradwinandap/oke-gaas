package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestHealthEndpoint(t *testing.T) {
	app := New()

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, 200, resp.StatusCode)

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Equal(t, "ok", body["status"])
}

func TestTelemetryMiddlewareContinuesIncomingTrace(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	app := New()
	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	spans := recorder.Ended()
	require.Len(t, spans, 1)
	require.Equal(t, "0af7651916cd43dd8448eb211c80319c", spans[0].SpanContext().TraceID().String())
	require.Equal(t, "b7ad6b7169203331", spans[0].Parent().SpanID().String())
}

func TestCentralErrorHandlerReturnsSanitizedJSONEnvelope(t *testing.T) {
	app := New()
	app.Get("/transport-error", func(c fiber.Ctx) error {
		return fiber.ErrRequestEntityTooLarge
	})
	app.Get("/internal-error", func(c fiber.Ctx) error {
		return errors.New("database password=secret")
	})
	app.Get("/panic", func(c fiber.Ctx) error {
		panic("sensitive panic detail")
	})

	for _, tc := range []struct {
		path       string
		statusCode int
		code       string
		message    string
	}{
		{path: "/transport-error", statusCode: fiber.StatusRequestEntityTooLarge, code: "request_too_large", message: fiber.ErrRequestEntityTooLarge.Message},
		{path: "/internal-error", statusCode: fiber.StatusInternalServerError, code: "internal_error", message: "internal server error"},
		{path: "/panic", statusCode: fiber.StatusInternalServerError, code: "internal_error", message: "internal server error"},
	} {
		req := httptest.NewRequest("GET", tc.path, nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		var body struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		require.NoError(t, resp.Body.Close())
		require.Equal(t, tc.statusCode, resp.StatusCode)
		require.Equal(t, tc.code, body.Error.Code)
		require.Equal(t, tc.message, body.Error.Message)
		require.NotContains(t, body.Error.Message, "secret")
	}
}
