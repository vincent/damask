package queue

import (
	"context"
	"errors"
	"testing"

	dbpkg "damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestWorker_FailedJobRecordsSpanError(t *testing.T) {
	q, recorder := newTelemetryQueue(t)
	q.Register("failing", func(context.Context, dbgen.Job) error {
		return errors.New("boom")
	})
	mustEnqueue(t, q, "failing")

	q.processNext(context.Background())

	span := findQueueSpan(t, recorder, "job.failing")
	if span.Status().Code != codes.Error {
		t.Fatalf("status = %v, want Error", span.Status().Code)
	}
	if len(span.Events()) == 0 {
		t.Fatal("expected recorded error event")
	}
}

func TestWorker_SuccessfulJobSpan(t *testing.T) {
	q, recorder := newTelemetryQueue(t)
	q.Register("successful", func(context.Context, dbgen.Job) error {
		return nil
	})
	mustEnqueue(t, q, "successful")

	q.processNext(context.Background())

	span := findQueueSpan(t, recorder, "job.successful")
	if span.Status().Code != codes.Ok {
		t.Fatalf("status = %v, want Ok", span.Status().Code)
	}
	for _, attr := range span.Attributes() {
		if string(attr.Key) == "job.type" && attr.Value.AsString() == "successful" {
			return
		}
	}
	t.Fatal("job.type attribute not found")
}

func newTelemetryQueue(t *testing.T) (*Queue, *tracetest.SpanRecorder) {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	queries, sqlDB, err := dbpkg.Open(":memory:?_foreign_keys=ON")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if _, err := sqlDB.Exec(`INSERT INTO workspaces (id, name) VALUES ('ws_1', 'Test')`); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	return New(queries, 1), recorder
}

func mustEnqueue(t *testing.T, q *Queue, jobType string) {
	t.Helper()
	if _, err := q.Enqueue(context.Background(), "ws_1", jobType, "{}"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
}

func findQueueSpan(t *testing.T, recorder *tracetest.SpanRecorder, name string) sdktrace.ReadOnlySpan {
	t.Helper()
	for _, span := range recorder.Ended() {
		if span.Name() == name {
			return span
		}
	}
	t.Fatalf("span %q not found; ended=%d", name, len(recorder.Ended()))
	return nil
}
