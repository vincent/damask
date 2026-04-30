package telemetry

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	logglobal "go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

type Config struct {
	Enabled     bool
	Endpoint    string
	Token       string
	ServiceName string
	Env         string
}

func Setup(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	if !cfg.Enabled {
		otel.SetTracerProvider(trace.NewNoopTracerProvider())
		return func(context.Context) error { return nil }, nil
	}

	spanExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(cfg.Endpoint+"/traces"),
		otlptracehttp.WithHeaders(map[string]string{
			"Authorization": "Bearer " + cfg.Token,
		}),
	)
	if err != nil {
		otel.SetTracerProvider(trace.NewNoopTracerProvider())
		return func(context.Context) error { return nil }, err
	}

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(cfg.ServiceName),
		semconv.DeploymentEnvironment(cfg.Env),
	)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(spanExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

func InitLogs(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	if !cfg.Enabled {
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlploghttp.New(ctx,
		otlploghttp.WithEndpointURL(cfg.Endpoint+"/logs"),
		otlploghttp.WithHeaders(map[string]string{
			"Authorization": "Bearer " + cfg.Token,
		}),
	)
	if err != nil {
		return func(context.Context) error { return nil }, err
	}

	res, _ := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(cfg.ServiceName),
	))

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter,
			sdklog.WithExportInterval(2*time.Second))),
		sdklog.WithResource(res),
	)

	logglobal.SetLoggerProvider(lp)
	slog.SetDefault(otelslog.NewLogger(cfg.ServiceName, otelslog.WithLoggerProvider(lp)))

	return lp.Shutdown, nil
}

func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}
