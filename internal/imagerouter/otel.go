package imagerouter

import (
	"context"
	"encoding/json"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "damask/imagerouter"

func startGenAISpan(ctx context.Context, operation, model, prompt string) (context.Context, trace.Span) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "imagerouter "+operation,
		trace.WithSpanKind(trace.SpanKindClient),
	)
	span.SetAttributes(
		attribute.String("gen_ai.operation.name", operation),
		attribute.String("gen_ai.system", "openai"),
		attribute.String("gen_ai.request.model", model),
	)
	if prompt != "" {
		promptJSON, _ := json.Marshal(map[string]any{
			"messages": []map[string]any{
				{"role": "user", "content": prompt},
			},
		})
		span.SetAttributes(attribute.String("gen_ai.prompt", string(promptJSON)))
	}
	return ctx, span
}

func endGenAISpan(span trace.Span, responseModel string, envelope *imageResponseEnvelope, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.End()
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("gen_ai.response.model", responseModel),
		attribute.String("gen_ai.response.finish_reason", "stop"),
	}
	if envelope != nil {
		attrs = append(attrs, attribute.Float64("gen_ai.usage.total_cost", envelope.Cost))
	}
	span.SetAttributes(attrs...)
	span.End()
}
