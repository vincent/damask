package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

// -- helpers -----------------------------------------------------------------

func createFieldDef(t *testing.T, env *testEnv, cookie *http.Cookie, scope, name, key, fieldType string, options *string) map[string]interface{} {
	t.Helper()
	body := fmt.Sprintf(`{"scope":%q,"name":%q,"key":%q,"field_type":%q}`, scope, name, key, fieldType)
	if options != nil {
		// options must be sent as a JSON string (the DB stores it as TEXT containing a JSON array)
		optJSON, _ := json.Marshal(*options)
		body = fmt.Sprintf(`{"scope":%q,"name":%q,"key":%q,"field_type":%q,"options":%s}`, scope, name, key, fieldType, optJSON)
	}
	resp, err := env.app.Test(authRequest(http.MethodPost, "/api/v1/field-definitions", strings.NewReader(body), cookie))
	if err != nil {
		t.Fatalf("create field def: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create field def: expected 201, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode field def: %v", err)
	}
	return result
}

// -- CF-1: Field definitions CRUD --------------------------------------------

func TestFieldDefinitions_CRUD(t *testing.T) {
	env := setupTestApp(t)
	u := register(t, env, "Alice", "alice@example.com", "password123")

	// Create
	def := createFieldDef(t, env, u.Cookie, "asset", "Client Name", "client_name", "text", nil)
	if def["key"] != "client_name" {
		t.Fatalf("expected key=client_name, got %v", def["key"])
	}
	if def["field_type"] != "text" {
		t.Fatalf("expected field_type=text, got %v", def["field_type"])
	}
	id := def["id"].(string)

	// List
	resp, _ := env.app.Test(authRequest(http.MethodGet, "/api/v1/field-definitions?scope=asset", nil, u.Cookie))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", resp.StatusCode)
	}
	var list []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&list)
	if len(list) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(list))
	}

	// Get single
	resp, _ = env.app.Test(authRequest(http.MethodGet, "/api/v1/field-definitions/"+id, nil, u.Cookie))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", resp.StatusCode)
	}

	// Update name (allowed)
	resp, _ = env.app.Test(authRequest(http.MethodPut, "/api/v1/field-definitions/"+id,
		jsonStr(`{"name":"Client"}`), u.Cookie))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update: expected 200, got %d", resp.StatusCode)
	}
	var updated map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&updated)
	if updated["name"] != "Client" {
		t.Fatalf("expected name=Client, got %v", updated["name"])
	}

	// Update key (forbidden)
	resp, _ = env.app.Test(authRequest(http.MethodPut, "/api/v1/field-definitions/"+id,
		jsonStr(`{"key":"new_key"}`), u.Cookie))
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("update key: expected 422, got %d", resp.StatusCode)
	}

	// Update field_type (forbidden)
	resp, _ = env.app.Test(authRequest(http.MethodPut, "/api/v1/field-definitions/"+id,
		jsonStr(`{"field_type":"number"}`), u.Cookie))
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("update field_type: expected 422, got %d", resp.StatusCode)
	}

	// Soft delete
	resp, _ = env.app.Test(authRequest(http.MethodDelete, "/api/v1/field-definitions/"+id, nil, u.Cookie))
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", resp.StatusCode)
	}

	// List after delete — should be empty
	resp, _ = env.app.Test(authRequest(http.MethodGet, "/api/v1/field-definitions?scope=asset", nil, u.Cookie))
	json.NewDecoder(resp.Body).Decode(&list)
	if len(list) != 0 {
		t.Fatalf("expected 0 active definitions after delete, got %d", len(list))
	}
}

func TestFieldDefinitions_SelectValidation(t *testing.T) {
	env := setupTestApp(t)
	u := register(t, env, "Alice", "alice@example.com", "password123")

	// select without options → 400
	resp, _ := env.app.Test(authRequest(http.MethodPost, "/api/v1/field-definitions",
		jsonStr(`{"scope":"asset","name":"Status","key":"status","field_type":"select"}`), u.Cookie))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("select no options: expected 400, got %d", resp.StatusCode)
	}

	// select with valid options → 201
	opts := `["Draft","Review","Approved"]`
	def := createFieldDef(t, env, u.Cookie, "asset", "Status", "status", "select", &opts)
	if def["options"] == nil {
		t.Fatal("expected options to be set")
	}
}

func TestFieldDefinitions_InvalidKey(t *testing.T) {
	env := setupTestApp(t)
	u := register(t, env, "Alice", "alice@example.com", "password123")

	resp, _ := env.app.Test(authRequest(http.MethodPost, "/api/v1/field-definitions",
		jsonStr(`{"scope":"asset","name":"Bad Key","key":"Bad Key!","field_type":"text"}`), u.Cookie))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad key: expected 400, got %d", resp.StatusCode)
	}
}

func TestFieldDefinitions_DuplicateKey(t *testing.T) {
	env := setupTestApp(t)
	u := register(t, env, "Alice", "alice@example.com", "password123")

	createFieldDef(t, env, u.Cookie, "asset", "Client", "client", "text", nil)

	resp, _ := env.app.Test(authRequest(http.MethodPost, "/api/v1/field-definitions",
		jsonStr(`{"scope":"asset","name":"Client2","key":"client","field_type":"text"}`), u.Cookie))
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate key: expected 409, got %d", resp.StatusCode)
	}
}

func TestFieldDefinitions_ScopeIsolation(t *testing.T) {
	env := setupTestApp(t)
	u := register(t, env, "Alice", "alice@example.com", "password123")

	// Same key allowed in different scopes
	createFieldDef(t, env, u.Cookie, "asset", "Budget", "budget", "number", nil)
	createFieldDef(t, env, u.Cookie, "project", "Budget", "budget", "number", nil)

	resp, _ := env.app.Test(authRequest(http.MethodGet, "/api/v1/field-definitions?scope=asset", nil, u.Cookie))
	var list []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&list)
	if len(list) != 1 {
		t.Fatalf("asset scope: expected 1, got %d", len(list))
	}

	resp, _ = env.app.Test(authRequest(http.MethodGet, "/api/v1/field-definitions?scope=project", nil, u.Cookie))
	json.NewDecoder(resp.Body).Decode(&list)
	if len(list) != 1 {
		t.Fatalf("project scope: expected 1, got %d", len(list))
	}
}

func TestFieldDefinitions_Reorder(t *testing.T) {
	env := setupTestApp(t)
	u := register(t, env, "Alice", "alice@example.com", "password123")

	d1 := createFieldDef(t, env, u.Cookie, "asset", "Alpha", "alpha", "text", nil)
	d2 := createFieldDef(t, env, u.Cookie, "asset", "Beta", "beta", "text", nil)

	body := fmt.Sprintf(`[{"id":%q,"position":10},{"id":%q,"position":5}]`, d1["id"], d2["id"])
	resp, _ := env.app.Test(authRequest(http.MethodPut, "/api/v1/field-definitions/reorder",
		strings.NewReader(body), u.Cookie))
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("reorder: expected 204, got %d", resp.StatusCode)
	}

	// After reorder, beta (pos=5) should come before alpha (pos=10)
	resp, _ = env.app.Test(authRequest(http.MethodGet, "/api/v1/field-definitions?scope=asset", nil, u.Cookie))
	var list []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&list)
	if len(list) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(list))
	}
	if list[0]["key"] != "beta" {
		t.Fatalf("expected beta first after reorder, got %v", list[0]["key"])
	}
}

func TestFieldDefinitions_Stats(t *testing.T) {
	env := setupTestApp(t)
	u := register(t, env, "Alice", "alice@example.com", "password123")

	def := createFieldDef(t, env, u.Cookie, "asset", "Client", "client", "text", nil)
	id := def["id"].(string)

	resp, _ := env.app.Test(authRequest(http.MethodGet, "/api/v1/field-definitions/"+id+"/stats", nil, u.Cookie))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stats: expected 200, got %d", resp.StatusCode)
	}
	var stats map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&stats)
	if stats["asset_count"] == nil {
		t.Fatal("expected asset_count in stats")
	}
}

func TestFieldDefinitions_Auth(t *testing.T) {
	env := setupTestApp(t)
	u := register(t, env, "Alice", "alice@example.com", "password123")

	// Unauthenticated
	resp, _ := env.app.Test(authRequest(http.MethodGet, "/api/v1/field-definitions", nil, nil))
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauth list: expected 401, got %d", resp.StatusCode)
	}

	// Viewer cannot create
	viewerToken := mintEditorToken(t, env, u.WorkspaceID, "viewer")
	resp, _ = env.app.Test(bearerRequest(http.MethodPost, "/api/v1/field-definitions",
		jsonStr(`{"scope":"asset","name":"X","key":"x","field_type":"text"}`), viewerToken))
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer create: expected 403, got %d", resp.StatusCode)
	}
}

// -- CF-2: Asset field values ------------------------------------------------

func TestAssetFieldValues_GetPatch(t *testing.T) {
	env := setupTestApp(t)
	u := register(t, env, "Alice", "alice@example.com", "password123")

	def := createFieldDef(t, env, u.Cookie, "asset", "Client", "client", "text", nil)
	fieldID := def["id"].(string)

	// Create an asset via direct SQL (no file upload needed)
	assetID := "test-asset-fields-01"
	_, err := env.sqlDB.Exec(`INSERT INTO assets (id, workspace_id, original_filename, storage_key, mime_type, size)
		VALUES (?, ?, ?, ?, ?, ?)`, assetID, u.WorkspaceID, "test.jpg", "storage/test.jpg", "image/jpeg", 100)
	if err != nil {
		t.Fatalf("insert asset: %v", err)
	}

	// GET — no values yet
	resp, _ := env.app.Test(authRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/fields", nil, u.Cookie))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get fields: expected 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	fields := result["fields"].([]interface{})
	if len(fields) != 0 {
		t.Fatalf("expected 0 fields, got %d", len(fields))
	}

	// PATCH — set value
	body := fmt.Sprintf(`{"values":[{"field_id":%q,"value":"Nike"}]}`, fieldID)
	resp, _ = env.app.Test(authRequest(http.MethodPatch, "/api/v1/assets/"+assetID+"/fields",
		strings.NewReader(body), u.Cookie))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch fields: expected 200, got %d", resp.StatusCode)
	}
	json.NewDecoder(resp.Body).Decode(&result)
	fields = result["fields"].([]interface{})
	if len(fields) != 1 {
		t.Fatalf("expected 1 field after patch, got %d", len(fields))
	}
	field := fields[0].(map[string]interface{})
	if field["value"] != "Nike" {
		t.Fatalf("expected value=Nike, got %v", field["value"])
	}

	// PATCH — clear value (null)
	body = fmt.Sprintf(`{"values":[{"field_id":%q,"value":null}]}`, fieldID)
	resp, _ = env.app.Test(authRequest(http.MethodPatch, "/api/v1/assets/"+assetID+"/fields",
		strings.NewReader(body), u.Cookie))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("clear field: expected 200, got %d", resp.StatusCode)
	}
	json.NewDecoder(resp.Body).Decode(&result)
	fields = result["fields"].([]interface{})
	if len(fields) != 0 {
		t.Fatalf("expected 0 fields after clear, got %d", len(fields))
	}
}

func TestAssetFieldValues_TypeValidation(t *testing.T) {
	env := setupTestApp(t)
	u := register(t, env, "Alice", "alice@example.com", "password123")

	numDef := createFieldDef(t, env, u.Cookie, "asset", "Price", "price", "number", nil)
	dateDef := createFieldDef(t, env, u.Cookie, "asset", "Expiry", "expiry", "date", nil)
	boolDef := createFieldDef(t, env, u.Cookie, "asset", "Published", "published", "boolean", nil)
	opts := `["Draft","Approved"]`
	selDef := createFieldDef(t, env, u.Cookie, "asset", "Status", "status", "select", &opts)

	assetID := "test-asset-validation"
	env.sqlDB.Exec(`INSERT INTO assets (id, workspace_id, original_filename, storage_key, mime_type, size)
		VALUES (?, ?, ?, ?, ?, ?)`, assetID, u.WorkspaceID, "t.jpg", "s/t.jpg", "image/jpeg", 100)

	cases := []struct {
		name     string
		fieldID  string
		value    string
		wantCode int
	}{
		{"number valid", numDef["id"].(string), `42.5`, http.StatusOK},
		{"number string", numDef["id"].(string), `"not a number"`, http.StatusUnprocessableEntity},
		{"date valid", dateDef["id"].(string), `"2026-12-31"`, http.StatusOK},
		{"date bad format", dateDef["id"].(string), `"31-12-2026"`, http.StatusUnprocessableEntity},
		{"boolean valid", boolDef["id"].(string), `true`, http.StatusOK},
		{"boolean string", boolDef["id"].(string), `"yes"`, http.StatusUnprocessableEntity},
		{"select valid", selDef["id"].(string), `"Draft"`, http.StatusOK},
		{"select invalid", selDef["id"].(string), `"Unknown"`, http.StatusUnprocessableEntity},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"values":[{"field_id":%q,"value":%s}]}`, tc.fieldID, tc.value)
			resp, _ := env.app.Test(authRequest(http.MethodPatch, "/api/v1/assets/"+assetID+"/fields",
				strings.NewReader(body), u.Cookie))
			if resp.StatusCode != tc.wantCode {
				t.Fatalf("%s: expected %d, got %d", tc.name, tc.wantCode, resp.StatusCode)
			}
		})
	}
}

func TestAssetFieldValues_SoftDeletedField(t *testing.T) {
	env := setupTestApp(t)
	u := register(t, env, "Alice", "alice@example.com", "password123")

	def := createFieldDef(t, env, u.Cookie, "asset", "Old Field", "old_field", "text", nil)
	fieldID := def["id"].(string)

	assetID := "test-asset-deleted-field"
	env.sqlDB.Exec(`INSERT INTO assets (id, workspace_id, original_filename, storage_key, mime_type, size)
		VALUES (?, ?, ?, ?, ?, ?)`, assetID, u.WorkspaceID, "t.jpg", "s/t.jpg", "image/jpeg", 100)

	// Set a value
	body := fmt.Sprintf(`{"values":[{"field_id":%q,"value":"some value"}]}`, fieldID)
	env.app.Test(authRequest(http.MethodPatch, "/api/v1/assets/"+assetID+"/fields",
		strings.NewReader(body), u.Cookie))

	// Soft delete the definition
	env.app.Test(authRequest(http.MethodDelete, "/api/v1/field-definitions/"+fieldID, nil, u.Cookie))

	// Cannot write to soft-deleted field
	resp, _ := env.app.Test(authRequest(http.MethodPatch, "/api/v1/assets/"+assetID+"/fields",
		strings.NewReader(body), u.Cookie))
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("write to deleted field: expected 422, got %d", resp.StatusCode)
	}

	// But GET still shows the orphaned value with definition_deleted=true
	resp, _ = env.app.Test(authRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/fields", nil, u.Cookie))
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	fields := result["fields"].([]interface{})
	if len(fields) != 1 {
		t.Fatalf("expected orphaned value to still appear, got %d fields", len(fields))
	}
	field := fields[0].(map[string]interface{})
	if field["definition_deleted"] != true {
		t.Fatalf("expected definition_deleted=true, got %v", field["definition_deleted"])
	}
}

func TestAssetFieldValues_BulkPatch(t *testing.T) {
	env := setupTestApp(t)
	u := register(t, env, "Alice", "alice@example.com", "password123")

	def := createFieldDef(t, env, u.Cookie, "asset", "Client", "client", "text", nil)
	fieldID := def["id"].(string)

	// Create two assets
	for _, id := range []string{"bulk-asset-1", "bulk-asset-2"} {
		env.sqlDB.Exec(`INSERT INTO assets (id, workspace_id, original_filename, storage_key, mime_type, size)
			VALUES (?, ?, ?, ?, ?, ?)`, id, u.WorkspaceID, id+".jpg", "s/"+id, "image/jpeg", 100)
	}

	body := fmt.Sprintf(`{"asset_ids":["bulk-asset-1","bulk-asset-2"],"values":[{"field_id":%q,"value":"Nike"}]}`, fieldID)
	resp, err2 := env.app.Test(authRequest(http.MethodPatch, "/api/v1/assets/bulk/fields",
		strings.NewReader(body), u.Cookie), fiber.TestConfig{Timeout: 5 * time.Second})
	if err2 != nil {
		t.Fatalf("bulk patch request: %v", err2)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("bulk patch: expected 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	if err3 := json.NewDecoder(resp.Body).Decode(&result); err3 != nil {
		t.Fatalf("decode bulk patch response: %v", err3)
	}
	if result["updated"].(float64) != 2 {
		t.Fatalf("expected updated=2, got %v", result["updated"])
	}

	// Verify values were set
	for _, id := range []string{"bulk-asset-1", "bulk-asset-2"} {
		resp, _ := env.app.Test(authRequest(http.MethodGet, "/api/v1/assets/"+id+"/fields", nil, u.Cookie))
		var r map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&r)
		fields := r["fields"].([]interface{})
		if len(fields) != 1 || fields[0].(map[string]interface{})["value"] != "Nike" {
			t.Fatalf("asset %s: expected value=Nike", id)
		}
	}
}

// -- CF-2.4: Asset field filters ---------------------------------------------

func TestAssetFieldFilter(t *testing.T) {
	env := setupTestApp(t)
	u := register(t, env, "Alice", "alice@example.com", "password123")

	def := createFieldDef(t, env, u.Cookie, "asset", "Client", "client", "text", nil)
	fieldID := def["id"].(string)

	// Create assets
	for _, id := range []string{"filter-asset-nike", "filter-asset-puma"} {
		env.sqlDB.Exec(`INSERT INTO assets (id, workspace_id, original_filename, storage_key, mime_type, size)
			VALUES (?, ?, ?, ?, ?, ?)`, id, u.WorkspaceID, id+".jpg", "s/"+id, "image/jpeg", 100)
	}

	// Set client=Nike on first asset
	body := fmt.Sprintf(`{"values":[{"field_id":%q,"value":"Nike"}]}`, fieldID)
	env.app.Test(authRequest(http.MethodPatch, "/api/v1/assets/filter-asset-nike/fields",
		strings.NewReader(body), u.Cookie))

	body = fmt.Sprintf(`{"values":[{"field_id":%q,"value":"Puma"}]}`, fieldID)
	env.app.Test(authRequest(http.MethodPatch, "/api/v1/assets/filter-asset-puma/fields",
		strings.NewReader(body), u.Cookie))

	// Filter by exact match
	resp, _ := env.app.Test(authRequest(http.MethodGet, "/api/v1/assets?field[client]=Nike", nil, u.Cookie))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("field filter: expected 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	assets := result["assets"].([]interface{})
	if len(assets) != 1 {
		t.Fatalf("filter by client=Nike: expected 1, got %d", len(assets))
	}

	// Filter contains
	resp, _ = env.app.Test(authRequest(http.MethodGet, "/api/v1/assets?field[client][contains]=ik", nil, u.Cookie))
	json.NewDecoder(resp.Body).Decode(&result)
	assets = result["assets"].([]interface{})
	if len(assets) != 1 {
		t.Fatalf("filter contains ik: expected 1, got %d", len(assets))
	}
}

func TestAssetFieldFilter_TooManyFilters(t *testing.T) {
	env := setupTestApp(t)
	u := register(t, env, "Alice", "alice@example.com", "password123")

	resp, _ := env.app.Test(authRequest(http.MethodGet,
		"/api/v1/assets?field[a]=1&field[b]=2&field[c]=3&field[d]=4&field[e]=5&field[f]=6",
		nil, u.Cookie))
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("too many filters: expected 422, got %d", resp.StatusCode)
	}
}

// -- CF-3: Project field values ----------------------------------------------

func TestProjectFieldValues(t *testing.T) {
	env := setupTestApp(t)
	u := register(t, env, "Alice", "alice@example.com", "password123")

	// Create a project field definition
	def := createFieldDef(t, env, u.Cookie, "project", "Budget", "budget", "number", nil)
	fieldID := def["id"].(string)

	// Create a project
	resp, _ := env.app.Test(authRequest(http.MethodPost, "/api/v1/projects",
		jsonStr(`{"name":"Test Project"}`), u.Cookie))
	var proj map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&proj)
	projectID := proj["id"].(string)

	// GET — empty
	resp, _ = env.app.Test(authRequest(http.MethodGet, "/api/v1/projects/"+projectID+"/fields", nil, u.Cookie))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get project fields: expected 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	fields := result["fields"].([]interface{})
	if len(fields) != 0 {
		t.Fatalf("expected 0 fields, got %d", len(fields))
	}

	// PATCH — set value
	body := fmt.Sprintf(`{"values":[{"field_id":%q,"value":50000}]}`, fieldID)
	resp, _ = env.app.Test(authRequest(http.MethodPatch, "/api/v1/projects/"+projectID+"/fields",
		strings.NewReader(body), u.Cookie))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch project fields: expected 200, got %d", resp.StatusCode)
	}
	json.NewDecoder(resp.Body).Decode(&result)
	fields = result["fields"].([]interface{})
	if len(fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(fields))
	}
	if fields[0].(map[string]interface{})["value"] != float64(50000) {
		t.Fatalf("expected value=50000, got %v", fields[0].(map[string]interface{})["value"])
	}
}

func TestProjectFieldValues_WrongScope(t *testing.T) {
	env := setupTestApp(t)
	u := register(t, env, "Alice", "alice@example.com", "password123")

	// Create an ASSET-scoped field
	def := createFieldDef(t, env, u.Cookie, "asset", "Client", "client", "text", nil)
	fieldID := def["id"].(string)

	// Create a project
	resp, _ := env.app.Test(authRequest(http.MethodPost, "/api/v1/projects",
		jsonStr(`{"name":"Test"}`), u.Cookie))
	var proj map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&proj)
	projectID := proj["id"].(string)

	// Try to set an asset-scoped field on a project → 422
	body := fmt.Sprintf(`{"values":[{"field_id":%q,"value":"Nike"}]}`, fieldID)
	resp, _ = env.app.Test(authRequest(http.MethodPatch, "/api/v1/projects/"+projectID+"/fields",
		strings.NewReader(body), u.Cookie))
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("wrong scope: expected 422, got %d", resp.StatusCode)
	}
}

// -- Workspace isolation -----------------------------------------------------

func TestFieldDefinitions_WorkspaceIsolation(t *testing.T) {
	env := setupTestApp(t)
	u1 := register(t, env, "Alice", "alice@example.com", "password123")
	u2 := register(t, env, "Bob", "bob@example.com", "password123")

	// u1 creates a field definition
	def := createFieldDef(t, env, u1.Cookie, "asset", "Secret", "secret", "text", nil)
	id := def["id"].(string)

	// u2 cannot see it
	resp, _ := env.app.Test(authRequest(http.MethodGet, "/api/v1/field-definitions?scope=asset", nil, u2.Cookie))
	var list []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&list)
	if len(list) != 0 {
		t.Fatalf("isolation: u2 should not see u1's field definitions, got %d", len(list))
	}

	// u2 cannot delete it
	resp, _ = env.app.Test(authRequest(http.MethodDelete, "/api/v1/field-definitions/"+id, nil, u2.Cookie))
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("isolation: u2 delete u1's def: expected 404, got %d", resp.StatusCode)
	}
}
