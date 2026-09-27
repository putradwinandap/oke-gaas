package http

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestEventProcessingMetricsRecordLowCardinalityMeasurements(t *testing.T) {
	ctx := context.Background()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(ctx)) })

	metrics := newEventProcessingMetrics(provider.Meter(instrumentationName))
	metrics.record(ctx, time.Now().Add(-25*time.Millisecond), nil, true, 3)

	var data metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &data))
	require.Len(t, data.ScopeMetrics, 1)
	require.Len(t, data.ScopeMetrics[0].Metrics, 3)

	seen := make(map[string]bool)
	for _, item := range data.ScopeMetrics[0].Metrics {
		seen[item.Name] = true
		checkAttributes := func(attrs attribute.Set) {
			require.ElementsMatch(t, []attribute.KeyValue{
				attribute.String("outcome", "success"),
				attribute.Bool("duplicate", true),
			}, attrs.ToSlice())
		}
		switch points := item.Data.(type) {
		case metricdata.Sum[int64]:
			require.Len(t, points.DataPoints, 1)
			require.EqualValues(t, 1, points.DataPoints[0].Value)
			checkAttributes(points.DataPoints[0].Attributes)
		case metricdata.Histogram[float64]:
			require.Len(t, points.DataPoints, 1)
			checkAttributes(points.DataPoints[0].Attributes)
			switch item.Name {
			case "oke_gaas.event.processing.duration":
				require.Greater(t, points.DataPoints[0].Sum, 0.0)
			case "oke_gaas.event.processing.reward_grants":
				require.Equal(t, 3.0, points.DataPoints[0].Sum)
			}
		default:
			t.Fatalf("unexpected metric aggregation type %T", item.Data)
		}
	}
	require.True(t, seen["oke_gaas.event.processing.count"])
	require.True(t, seen["oke_gaas.event.processing.duration"])
	require.True(t, seen["oke_gaas.event.processing.reward_grants"])
}
