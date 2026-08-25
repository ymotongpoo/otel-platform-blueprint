// Package otelinit is the internal OpenTelemetry SDK distribution of the
// blueprint organization. It assembles trace, metric and log providers with
// organization defaults while honoring standard OTEL_* environment variables.
//
// Contract:
//   - Setup returns a non-nil shutdown function even on error; calling it is
//     always safe.
//   - On error the SDK is not installed and the application decides whether
//     to continue without telemetry or to abort.
package otelinit

import (
	"context"
	"errors"
	"os"

	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/contrib/propagators/autoprop"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Setup initializes the OpenTelemetry SDK with organization defaults and
// installs the providers globally. Precedence: explicit code options (none in
// this minimal distro), then OTEL_* environment variables, then the defaults
// below.
func Setup(ctx context.Context) (func(context.Context) error, error) {
	noop := func(context.Context) error { return nil }

	// Organization default: talk to the node-local agent collector over
	// OTLP/HTTP unless the standard variable is already set.
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")
	}

	res, err := resource.New(ctx,
		resource.WithFromEnv(),      // OTEL_SERVICE_NAME, OTEL_RESOURCE_ATTRIBUTES
		resource.WithHost(),         // host.name
		resource.WithTelemetrySDK(), // telemetry.sdk.*
	)
	if err != nil {
		return noop, err
	}

	spanExporter, err := autoexport.NewSpanExporter(ctx) // OTEL_TRACES_EXPORTER
	if err != nil {
		return noop, err
	}
	metricReader, err := autoexport.NewMetricReader(ctx) // OTEL_METRICS_EXPORTER
	if err != nil {
		return noop, err
	}
	logExporter, err := autoexport.NewLogExporter(ctx) // OTEL_LOGS_EXPORTER
	if err != nil {
		return noop, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(spanExporter),
		sdktrace.WithResource(res),
	)
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(metricReader),
		sdkmetric.WithResource(res),
	)
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	global.SetLoggerProvider(lp)
	otel.SetTextMapPropagator(autoprop.NewTextMapPropagator()) // OTEL_PROPAGATORS

	shutdown := func(ctx context.Context) error {
		return errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx), lp.Shutdown(ctx))
	}
	return shutdown, nil
}
