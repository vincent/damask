package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

// createIngressSource is a test helper that POSTs to /api/v1/ingress/sources
// and returns the parsed response. Fails the test if status != 201.
func createIngressSource(t *testing.T, env *testEnv, cookie *http.Cookie, body string) map[string]any {
	t.Helper()
	req := authRequest(http.MethodPost, "/api/v1/ingress/sources", jsonStr(body), cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("createIngressSource request: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("createIngressSource: expected 201, got %d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode createIngressSource response: %v", err)
	}
	return out
}

// --- Source CRUD

func TestCreateIngressSource_Success(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	body := `{"type":"imap","label":"Test Inbox","config":{"host":"imap.example.com","password":"s3cr3t"},"poll_interval_min":30}`
	src := createIngressSource(t, env, user.Cookie, body)

	if src["id"] == nil || src["id"] == "" {
		t.Fatal("expected non-empty id in response")
	}
	if src["type"] != "imap" {
		t.Fatalf("type: got %v, want imap", src["type"])
	}
	if src["label"] != "Test Inbox" {
		t.Fatalf("label: got %v, want 'Test Inbox'", src["label"])
	}
	// Sensitive config values must be redacted
	config, ok := src["config"].(map[string]any)
	if !ok {
		t.Fatal("config should be an object")
	}
	if config["password"] != "***" {
		t.Fatalf("password should be redacted, got %v", config["password"])
	}
	if config["host"] != "imap.example.com" {
		t.Fatalf("non-sensitive key should not be redacted, got %v", config["host"])
	}
}

func TestCreateIngressSource_DefaultInterval(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	// omit poll_interval_min — should default to 15
	body := `{"type":"sftp","label":"My SFTP","config":{}}`
	src := createIngressSource(t, env, user.Cookie, body)

	if src["poll_interval_min"] != float64(15) {
		t.Fatalf("expected default poll_interval_min=15, got %v", src["poll_interval_min"])
	}
}

func TestCreateIngressSource_MissingType(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	req := authRequest(http.MethodPost, "/api/v1/ingress/sources",
		jsonStr(`{"label":"No type","config":{}}`), user.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing type, got %d", resp.StatusCode)
	}
}

func TestCreateIngressSource_MissingLabel(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	req := authRequest(http.MethodPost, "/api/v1/ingress/sources",
		jsonStr(`{"type":"imap","config":{}}`), user.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing label, got %d", resp.StatusCode)
	}
}

func TestCreateIngressSource_Unauthenticated(t *testing.T) {
	env := setupTestApp(t)

	req := authRequest(http.MethodPost, "/api/v1/ingress/sources",
		jsonStr(`{"type":"imap","label":"x","config":{}}`), nil)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestListIngressSources_Empty(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	req := authRequest(http.MethodGet, "/api/v1/ingress/sources", nil, user.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var list []any
	json.NewDecoder(resp.Body).Decode(&list)
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %d items", len(list))
	}
}

func TestListIngressSources_ReturnsOwnerSources(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	createIngressSource(t, env, user.Cookie,
		`{"type":"imap","label":"Source A","config":{}}`)
	createIngressSource(t, env, user.Cookie,
		`{"type":"sftp","label":"Source B","config":{}}`)

	req := authRequest(http.MethodGet, "/api/v1/ingress/sources", nil, user.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var list []any
	json.NewDecoder(resp.Body).Decode(&list)
	if len(list) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(list))
	}
}

func TestListIngressSources_WorkspaceIsolation(t *testing.T) {
	env := setupTestApp(t)
	alice := register(t, env, "Alice", "alice@example.com", "password123")
	bob := register(t, env, "Bob", "bob@example.com", "password456")

	createIngressSource(t, env, alice.Cookie, `{"type":"imap","label":"Alice source","config":{}}`)

	req := authRequest(http.MethodGet, "/api/v1/ingress/sources", nil, bob.Cookie)
	resp, _ := env.app.Test(req)
	var list []any
	json.NewDecoder(resp.Body).Decode(&list)
	if len(list) != 0 {
		t.Fatalf("Bob should not see Alice's sources, got %d", len(list))
	}
}

func TestGetIngressSource_Success(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	created := createIngressSource(t, env, user.Cookie,
		`{"type":"imap","label":"My Inbox","config":{"host":"imap.example.com"}}`)
	id := created["id"].(string)

	req := authRequest(http.MethodGet, "/api/v1/ingress/sources/"+id, nil, user.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var src map[string]any
	json.NewDecoder(resp.Body).Decode(&src)
	if src["id"] != id {
		t.Fatalf("id mismatch: got %v, want %v", src["id"], id)
	}
}

func TestGetIngressSource_NotFound(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	req := authRequest(http.MethodGet, "/api/v1/ingress/sources/does-not-exist", nil, user.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetIngressSource_WrongWorkspace(t *testing.T) {
	env := setupTestApp(t)
	alice := register(t, env, "Alice", "alice@example.com", "password123")
	bob := register(t, env, "Bob", "bob@example.com", "password456")

	created := createIngressSource(t, env, alice.Cookie,
		`{"type":"imap","label":"Alice","config":{}}`)
	id := created["id"].(string)

	req := authRequest(http.MethodGet, "/api/v1/ingress/sources/"+id, nil, bob.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 when accessing another workspace's source, got %d", resp.StatusCode)
	}
}

func TestUpdateIngressSource_Label(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	created := createIngressSource(t, env, user.Cookie,
		`{"type":"imap","label":"Old Label","config":{}}`)
	id := created["id"].(string)

	req := authRequest(http.MethodPut, "/api/v1/ingress/sources/"+id,
		jsonStr(`{"label":"New Label"}`), user.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var src map[string]any
	json.NewDecoder(resp.Body).Decode(&src)
	if src["label"] != "New Label" {
		t.Fatalf("expected 'New Label', got %v", src["label"])
	}
}

func TestUpdateIngressSource_EnabledFlag(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	created := createIngressSource(t, env, user.Cookie,
		`{"type":"imap","label":"Inbox","config":{}}`)
	id := created["id"].(string)

	// Disable it
	req := authRequest(http.MethodPut, "/api/v1/ingress/sources/"+id,
		jsonStr(`{"enabled":false}`), user.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var src map[string]any
	json.NewDecoder(resp.Body).Decode(&src)
	if src["enabled"] != false {
		t.Fatalf("expected enabled=false, got %v", src["enabled"])
	}
}

func TestUpdateIngressSource_ConfigUpdated(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	created := createIngressSource(t, env, user.Cookie,
		`{"type":"imap","label":"Inbox","config":{"host":"old.example.com","password":"old"}}`)
	id := created["id"].(string)

	req := authRequest(http.MethodPut, "/api/v1/ingress/sources/"+id,
		jsonStr(`{"config":{"host":"new.example.com","password":"newpass"}}`), user.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var src map[string]any
	json.NewDecoder(resp.Body).Decode(&src)
	config := src["config"].(map[string]any)
	if config["host"] != "new.example.com" {
		t.Fatalf("expected host updated, got %v", config["host"])
	}
	if config["password"] != "***" {
		t.Fatalf("password should be redacted, got %v", config["password"])
	}
}

func TestUpdateIngressSource_NotFound(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	req := authRequest(http.MethodPut, "/api/v1/ingress/sources/nonexistent",
		jsonStr(`{"label":"x"}`), user.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDeleteIngressSource_Success(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	created := createIngressSource(t, env, user.Cookie,
		`{"type":"imap","label":"To Delete","config":{}}`)
	id := created["id"].(string)

	req := authRequest(http.MethodDelete, "/api/v1/ingress/sources/"+id, nil, user.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Verify it's gone
	req2 := authRequest(http.MethodGet, "/api/v1/ingress/sources/"+id, nil, user.Cookie)
	resp2, _ := env.app.Test(req2)
	if resp2.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", resp2.StatusCode)
	}
}

func TestDeleteIngressSource_RequiresOwner(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")

	created := createIngressSource(t, env, owner.Cookie,
		`{"type":"imap","label":"Src","config":{}}`)
	id := created["id"].(string)

	editorToken := mintEditorToken(t, env, owner.WorkspaceID, "editor")
	req := bearerRequest(http.MethodDelete, "/api/v1/ingress/sources/"+id, nil, editorToken)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for editor deleting source, got %d", resp.StatusCode)
	}
}

// --- Poll endpoint

func TestPollIngressSource_EnqueuesJob(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	created := createIngressSource(t, env, user.Cookie,
		`{"type":"imap","label":"Inbox","config":{}}`)
	id := created["id"].(string)

	req := authRequest(http.MethodPost, "/api/v1/ingress/sources/"+id+"/poll", nil, user.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if body["job_id"] == nil || body["job_id"] == "" {
		t.Fatal("expected job_id in response")
	}
}

func TestPollIngressSource_NotFound(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	req := authRequest(http.MethodPost, "/api/v1/ingress/sources/nonexistent/poll", nil, user.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// --- Log API

func TestListIngressSourceLog_Empty(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	created := createIngressSource(t, env, user.Cookie,
		`{"type":"imap","label":"Inbox","config":{}}`)
	id := created["id"].(string)

	req := authRequest(http.MethodGet, "/api/v1/ingress/sources/"+id+"/log", nil, user.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var list []any
	json.NewDecoder(resp.Body).Decode(&list)
	if len(list) != 0 {
		t.Fatalf("expected empty log, got %d entries", len(list))
	}
}

func TestListIngressSourceLog_SourceNotFound(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	req := authRequest(http.MethodGet, "/api/v1/ingress/sources/nonexistent/log", nil, user.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestListWorkspaceIngressLog_Empty(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	req := authRequest(http.MethodGet, "/api/v1/ingress/log", nil, user.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var list []any
	json.NewDecoder(resp.Body).Decode(&list)
	if len(list) != 0 {
		t.Fatalf("expected empty log, got %d", len(list))
	}
}

func TestDeleteIngressLogEntry_NotFound(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	req := authRequest(http.MethodDelete, "/api/v1/ingress/log/nonexistent", nil, user.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDeleteIngressLogEntry_WrongWorkspace(t *testing.T) {
	env := setupTestApp(t)
	alice := register(t, env, "Alice", "alice@example.com", "password123")
	bob := register(t, env, "Bob", "bob@example.com", "password456")

	created := createIngressSource(t, env, alice.Cookie,
		`{"type":"imap","label":"Inbox","config":{}}`)
	sourceID := created["id"].(string)

	// Insert a log entry directly for Alice's source
	entryID := "test-entry-id"
	_, err := env.sqlDB.Exec(
		`INSERT INTO ingress_log (id, source_id, remote_id, filename, status)
		 VALUES (?, ?, ?, ?, 'imported')`,
		entryID, sourceID, "msg-001", "photo.jpg",
	)
	if err != nil {
		t.Fatalf("insert log entry: %v", err)
	}

	// Bob tries to delete Alice's log entry — should get 403
	req := authRequest(http.MethodDelete, "/api/v1/ingress/log/"+entryID, nil, bob.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestRetryIngressLogEntry_NotFound(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	req := authRequest(http.MethodPost, "/api/v1/ingress/log/nonexistent/retry", nil, user.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestRetryIngressLogEntry_ImportedEntryNotRetryable(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	created := createIngressSource(t, env, user.Cookie,
		`{"type":"imap","label":"Inbox","config":{}}`)
	sourceID := created["id"].(string)

	entryID := "test-entry-imported"
	_, err := env.sqlDB.Exec(
		`INSERT INTO ingress_log (id, source_id, remote_id, filename, status)
		 VALUES (?, ?, ?, ?, 'imported')`,
		entryID, sourceID, "msg-002", "photo.jpg",
	)
	if err != nil {
		t.Fatalf("insert log entry: %v", err)
	}

	req := authRequest(http.MethodPost, "/api/v1/ingress/log/"+entryID+"/retry", nil, user.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for retrying 'imported' entry, got %d", resp.StatusCode)
	}
}

func TestRetryIngressLogEntry_ErrorEntryRequeued(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	created := createIngressSource(t, env, user.Cookie,
		`{"type":"imap","label":"Inbox","config":{}}`)
	sourceID := created["id"].(string)

	entryID := "test-entry-error"
	errMsg := "connection refused"
	_, err := env.sqlDB.Exec(
		`INSERT INTO ingress_log (id, source_id, remote_id, filename, status, error)
		 VALUES (?, ?, ?, ?, 'error', ?)`,
		entryID, sourceID, "msg-003", "photo.jpg", errMsg,
	)
	if err != nil {
		t.Fatalf("insert log entry: %v", err)
	}

	req := authRequest(http.MethodPost, "/api/v1/ingress/log/"+entryID+"/retry", nil, user.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if body["job_id"] == nil || body["job_id"] == "" {
		t.Fatal("expected job_id in retry response")
	}
}

func TestRetryIngressLogEntry_SkippedEntryRequeued(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	created := createIngressSource(t, env, user.Cookie,
		`{"type":"imap","label":"Inbox","config":{}}`)
	sourceID := created["id"].(string)

	entryID := "test-entry-skipped"
	_, err := env.sqlDB.Exec(
		`INSERT INTO ingress_log (id, source_id, remote_id, filename, status)
		 VALUES (?, ?, ?, ?, 'skipped')`,
		entryID, sourceID, "msg-004", "invoice.pdf",
	)
	if err != nil {
		t.Fatalf("insert log entry: %v", err)
	}

	req := authRequest(http.MethodPost, "/api/v1/ingress/log/"+entryID+"/retry", nil, user.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 for skipped retry, got %d", resp.StatusCode)
	}
}

// --- Test endpoint (no side-effects, only validates credentials)

func TestTestIngressSource_UnknownSourceType(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	// Create a source with a type that has no registered constructor.
	created := createIngressSource(t, env, user.Cookie,
		`{"type":"unregistered_type","label":"Mystery","config":{}}`)
	id := created["id"].(string)

	req := authRequest(http.MethodPost, "/api/v1/ingress/sources/"+id+"/test", nil, user.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for unknown source type in /test, got %d", resp.StatusCode)
	}
}

func TestTestIngressSource_NotFound(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	req := authRequest(http.MethodPost, "/api/v1/ingress/sources/nonexistent/test", nil, user.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// --- Source with initial rules

func TestCreateIngressSource_WithRules(t *testing.T) {
	env := setupTestApp(t)
	user := register(t, env, "Alice", "alice@example.com", "password123")

	body := `{
		"type":"imap",
		"label":"Filtered Inbox",
		"config":{},
		"rules":[
			{"position":0,"field":"filename","operator":"ends_with","value":".pdf","action":"allow"},
			{"position":1,"field":"mime_type","operator":"starts_with","value":"image/","action":"allow"}
		]
	}`
	src := createIngressSource(t, env, user.Cookie, body)
	id := src["id"].(string)

	// Verify source was created
	req := authRequest(http.MethodGet, "/api/v1/ingress/sources/"+id, nil, user.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// --- rawToNullableString helper (unit test, no HTTP)

func TestRawToNullableString(t *testing.T) {
	cases := []struct {
		name        string
		input       *json.RawMessage
		wantValue   *string
		wantPresent bool
	}{
		{
			name:        "nil pointer (field absent)",
			input:       nil,
			wantValue:   nil,
			wantPresent: false,
		},
		{
			name:        "explicit JSON null",
			input:       rawMsg("null"),
			wantValue:   nil,
			wantPresent: true,
		},
		{
			name:        "JSON string value",
			input:       rawMsg(`"folder-abc"`),
			wantValue:   strPtr("folder-abc"),
			wantPresent: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotValue, gotPresent := rawToNullableString(tc.input)
			if gotPresent != tc.wantPresent {
				t.Fatalf("present: got %v, want %v", gotPresent, tc.wantPresent)
			}
			if tc.wantValue == nil && gotValue != nil {
				t.Fatalf("value: got %q, want nil", *gotValue)
			}
			if tc.wantValue != nil {
				if gotValue == nil {
					t.Fatal("value: got nil, want non-nil")
				}
				if *gotValue != *tc.wantValue {
					t.Fatalf("value: got %q, want %q", *gotValue, *tc.wantValue)
				}
			}
		})
	}
}

// --- redactConfig helper (unit test, no HTTP)

func TestRedactConfig(t *testing.T) {
	input := map[string]any{
		"host":             "imap.example.com",
		"password":         "s3cr3t",
		"secret_key":       "abc123",
		"access_token":     "tok-xyz",
		"api_key":          "key-999",
		"username":         "alice",
		"private_key_data": "-----BEGIN",
	}
	got := redactConfig(input)

	notRedacted := []string{"host", "username"}
	for _, k := range notRedacted {
		if got[k] != input[k] {
			t.Errorf("key %q should not be redacted, got %v", k, got[k])
		}
	}

	redacted := []string{"password", "secret_key", "access_token", "api_key", "private_key_data"}
	for _, k := range redacted {
		if got[k] != "***" {
			t.Errorf("key %q should be redacted to ***, got %v", k, got[k])
		}
	}
}

// --- helpers

func rawMsg(s string) *json.RawMessage {
	r := json.RawMessage(s)
	return &r
}

func strPtr(s string) *string { return &s }

