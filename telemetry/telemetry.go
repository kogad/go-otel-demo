package telemetry

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// setupOTelSDK bootstraps stdout exporters for trace, metric, log,
// and sets up W3C TraceContext+Baggage propagator.
// Returns a shutdown func that you must call on exit.
func SetupOTelSDK(ctx context.Context, serviceName string) (shutdown func(context.Context) error, err error) {
	var shutdownFuncs []func(context.Context) error

	shutdown = func(ctx context.Context) error {
		var merr error
		for _, fn := range shutdownFuncs {
			merr = errors.Join(merr, fn(ctx))
		}
		shutdownFuncs = nil
		return merr
	}

	// Propagator
	prop := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	)
	otel.SetTextMapPropagator(prop)

	// Trace
	otlpClient := otlptracegrpc.NewClient(
		otlptracegrpc.WithEndpoint("localhost:4317"),
		otlptracegrpc.WithInsecure(),
		// otlptracegrpc.WithDialOption(grpc.WithBlock()),
	)

	trExp, err := otlptrace.New(ctx, otlpClient)
	if err != nil {
		return nil, err
	}

	tp, err := newTracerProvider(serviceName, trExp)
	if err != nil {
		shutdown(ctx)
		return nil, err
	}
	shutdownFuncs = append(shutdownFuncs, tp.Shutdown)
	otel.SetTracerProvider(tp)

	// Metric
	mp, err := newMeterProvider()
	if err != nil {
		shutdown(ctx)
		return nil, err
	}
	shutdownFuncs = append(shutdownFuncs, mp.Shutdown)
	otel.SetMeterProvider(mp)

	// Log
	lp, err := newLoggerProvider()
	if err != nil {
		shutdown(ctx)
		return nil, err
	}
	shutdownFuncs = append(shutdownFuncs, lp.Shutdown)
	global.SetLoggerProvider(lp)

	return shutdown, nil
}

func newTracerProvider(serviceName string, trExp *otlptrace.Exporter) (*sdktrace.TracerProvider, error) {
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(trExp, sdktrace.WithBatchTimeout(5*time.Second)),
		sdktrace.WithResource(
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String(serviceName),
			),
		),
	)
	return tp, nil
}

func newMeterProvider() (*sdkmetric.MeterProvider, error) {
	exporter, err := stdoutmetric.New()
	if err != nil {
		return nil, err
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter,
			sdkmetric.WithInterval(3*time.Second))),
	)
	return mp, nil
}

func newLoggerProvider() (*sdklog.LoggerProvider, error) {
	exporter, err := stdoutlog.New()
	if err != nil {
		return nil, err
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
	)
	return lp, nil
}
