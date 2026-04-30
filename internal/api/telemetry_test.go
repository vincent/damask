package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/config"
	"damask/server/internal/service"
	"damask/server/internal/testutil"
	"damask/server/internal/testutil/fixtures"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestAssetHandler_SpanAttributes(t *testing.T) {
	recorder := installSpanRecorder(t)
	env := testutil.NewTestEnv(t)
	env.Assets.GetFn = func(_ context.Context, workspaceID, assetID string) (*service.AssetDTO, error) {
		return fixtures.Asset(func(a *service.AssetDTO) {
			a.ID = assetID
			a.WorkspaceID = workspaceID
		}), nil
	}

	token := env.MintToken(t, "user_1", "ws_1")
	resp, err := env.App.Test(testutil.BearerRequest(http.MethodGet, "/api/v1/assets/asset_1", nil, token))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	span := findSpan(t, recorder, "GET /api/v1/assets/:id")
	assertAttr(t, span.Attributes(), "damask.workspace_id", "ws_1")
	assertAttr(t, span.Attributes(), "damask.user_id", "user_1")
}

func TestAssetHandler_Span404(t *testing.T) {
	recorder := installSpanRecorder(t)
	env := testutil.NewTestEnv(t)
	env.Assets.GetFn = func(_ context.Context, _, _ string) (*service.AssetDTO, error) {
		return nil, apperr.ErrNotFound
	}

	token := env.MintToken(t, "user_1", "ws_1")
	resp, err := env.App.Test(testutil.BearerRequest(http.MethodGet, "/api/v1/assets/missing", nil, token))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusNotFound)

	span := findSpan(t, recorder, "GET /api/v1/assets/:id")
	if span.Status().Code != codes.Error {
		t.Fatalf("status code = %v, want Error", span.Status().Code)
	}
}

func TestGetTelemetryStatus_Disabled(t *testing.T) {
	env := testutil.NewTestEnv(t)
	token := env.MintToken(t, "user_1", "ws_1")

	resp, err := env.App.Test(testutil.BearerRequest(http.MethodGet, "/api/admin/telemetry", nil, token))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Enabled {
		t.Fatalf("unexpected telemetry status: %+v", body)
	}
}

func TestGetTelemetryStatus_Enabled(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Server.SetConfigForTest(&config.Config{
		Telemetry: config.TelemetryConfig{Enabled: true, ServiceName: "damask", Env: "test"},
	})
	token := env.MintToken(t, "user_1", "ws_1")

	resp, err := env.App.Test(testutil.BearerRequest(http.MethodGet, "/api/admin/telemetry", nil, token))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Enabled {
		t.Fatal("expected enabled")
	}
}

func TestGetTelemetryStatus_Unauthorized(t *testing.T) {
	env := testutil.NewTestEnv(t)
	resp, err := env.App.Test(testutil.BearerRequest(http.MethodGet, "/api/admin/telemetry", nil, ""))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func installSpanRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { _ = tp.Shutdown(t.Context()) })
	return recorder
}

func findSpan(t *testing.T, recorder *tracetest.SpanRecorder, name string) sdktrace.ReadOnlySpan {
	t.Helper()
	for _, span := range recorder.Ended() {
		if span.Name() == name {
			return span
		}
	}
	t.Fatalf("span %q not found; ended=%d", name, len(recorder.Ended()))
	return nil
}

func assertAttr(t *testing.T, attrs []attribute.KeyValue, key, want string) {
	t.Helper()
	for _, attr := range attrs {
		if string(attr.Key) == key {
			if attr.Value.AsString() != want {
				t.Fatalf("%s = %q, want %q", key, attr.Value.AsString(), want)
			}
			return
		}
	}
	t.Fatalf("attribute %s not found", key)
}
