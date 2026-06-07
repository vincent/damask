package workflow

import (
	"context"
	"encoding/json"
	"testing"
)

// --- helpers ---

func rc(kv ...any) *RunContext {
	data := map[string]any{}
	for i := 0; i < len(kv)-1; i += 2 {
		data[kv[i].(string)] = kv[i+1]
	}
	return NewRunContext(data)
}

func cfg(raw string) json.RawMessage { return json.RawMessage(raw) }

// --- passThroughNode (triggers + control.fan_out) ---

func TestTriggerNodes_SchemaAndExecute(t *testing.T) {
	t.Parallel()
	types := []string{
		"trigger.manual",
		"trigger.asset_created",
		"trigger.version_uploaded",
		"trigger.tag_added",
		"trigger.schedule",
		"trigger.webhook",
		"control.fan_out",
	}
	for _, nodeType := range types {
		t.Run(nodeType, func(t *testing.T) {
			t.Parallel()
			node, err := Build(Deps{}, nodeType)
			if err != nil {
				t.Fatalf("Build(%q): %v", nodeType, err)
			}
			schema := node.Schema()
			if schema.Type != nodeType {
				t.Errorf("Schema.Type: got %q, want %q", schema.Type, nodeType)
			}
			if len(schema.Outputs) < 2 {
				t.Errorf("expected at least 2 outputs, got %d", len(schema.Outputs))
			}
			port, updates, err := node.Execute(context.Background(), rc(), cfg(`{}`))
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if port != portOut {
				t.Errorf("port: got %q, want %q", port, portOut)
			}
			if updates != nil {
				t.Errorf("expected nil updates, got %v", updates)
			}
		})
	}
}

// --- filterNode ---

func buildFilter(t *testing.T, nodeType string) Node {
	t.Helper()
	node, err := Build(Deps{}, nodeType)
	if err != nil {
		t.Fatalf("Build(%q): %v", nodeType, err)
	}
	return node
}

func TestFilterMime_Match(t *testing.T) {
	t.Parallel()
	node := buildFilter(t, "filter.mime")
	port, _, err := node.Execute(context.Background(),
		rc("mime_type", "image/jpeg"),
		cfg(`{"prefix":"image/"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != portMatch {
		t.Errorf("expected %q, got %q", portMatch, port)
	}
}

func TestFilterMime_NoMatch(t *testing.T) {
	t.Parallel()
	node := buildFilter(t, "filter.mime")
	port, _, err := node.Execute(context.Background(),
		rc("mime_type", "video/mp4"),
		cfg(`{"prefix":"image/"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != "no_match" {
		t.Errorf("expected no_match, got %q", port)
	}
}

func TestFilterFilename_Contains_Match(t *testing.T) {
	t.Parallel()
	node := buildFilter(t, "filter.filename")
	port, _, err := node.Execute(context.Background(),
		rc("filename", "product_hero.jpg"),
		cfg(`{"contains":"hero"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != portMatch {
		t.Errorf("expected match, got %q", port)
	}
}

func TestFilterFilename_Extension_NoMatch(t *testing.T) {
	t.Parallel()
	node := buildFilter(t, "filter.filename")
	port, _, err := node.Execute(context.Background(),
		rc("filename", "document.pdf"),
		cfg(`{"extension":".jpg"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != "no_match" {
		t.Errorf("expected no_match, got %q", port)
	}
}

func TestFilterSize_InRange(t *testing.T) {
	t.Parallel()
	node := buildFilter(t, "filter.size")
	port, _, err := node.Execute(context.Background(),
		rc("size", float64(5000)),
		cfg(`{"min":1000,"max":10000}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != portMatch {
		t.Errorf("expected match, got %q", port)
	}
}

func TestFilterSize_OutOfRange(t *testing.T) {
	t.Parallel()
	node := buildFilter(t, "filter.size")
	port, _, err := node.Execute(context.Background(),
		rc("size", float64(500)),
		cfg(`{"min":1000}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != "no_match" {
		t.Errorf("expected no_match, got %q", port)
	}
}

func TestFilterTag_Match(t *testing.T) {
	t.Parallel()
	node := buildFilter(t, "filter.tag")
	port, _, err := node.Execute(context.Background(),
		rc("tag_name", "Hero"),
		cfg(`{"name":"hero"}`)) // case-insensitive
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != portMatch {
		t.Errorf("expected match, got %q", port)
	}
}

func TestFilterTag_NoMatch(t *testing.T) {
	t.Parallel()
	node := buildFilter(t, "filter.tag")
	port, _, err := node.Execute(context.Background(),
		rc("tag_name", "draft"),
		cfg(`{"name":"hero"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != "no_match" {
		t.Errorf("expected no_match, got %q", port)
	}
}

func TestFilterFolder_Match(t *testing.T) {
	t.Parallel()
	node := buildFilter(t, "filter.folder")
	port, _, err := node.Execute(context.Background(),
		rc("folder_id", "fld_1"),
		cfg(`{"folder_id":"fld_1"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != portMatch {
		t.Errorf("expected match, got %q", port)
	}
}

func TestFilterExpression_Match(t *testing.T) {
	t.Parallel()
	node := buildFilter(t, "filter.expression")
	port, _, err := node.Execute(context.Background(),
		rc("status", "ready"),
		cfg(`{"key":"status","value":"ready"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != portMatch {
		t.Errorf("expected match, got %q", port)
	}
}

func TestFilterExpression_NoMatch(t *testing.T) {
	t.Parallel()
	node := buildFilter(t, "filter.expression")
	port, _, err := node.Execute(context.Background(),
		rc("status", "pending"),
		cfg(`{"key":"status","value":"ready"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != "no_match" {
		t.Errorf("expected no_match, got %q", port)
	}
}

// --- rcGetString / rcRequireString ---

func TestRcGetString_String(t *testing.T) {
	t.Parallel()
	r := rc("key", "value")
	v, ok := rcGetString(r, "key")
	if !ok || v != "value" {
		t.Fatalf("expected (value, true), got (%q, %v)", v, ok)
	}
}

func TestRcGetString_Float(t *testing.T) {
	t.Parallel()
	r := rc("size", float64(42))
	v, ok := rcGetString(r, "size")
	if !ok || v == "" {
		t.Fatalf("expected non-empty string, got (%q, %v)", v, ok)
	}
}

func TestRcGetString_Missing(t *testing.T) {
	t.Parallel()
	r := rc()
	v, ok := rcGetString(r, "missing")
	if ok || v != "" {
		t.Fatalf("expected (\"\", false), got (%q, %v)", v, ok)
	}
}

func TestRcRequireString_Missing(t *testing.T) {
	t.Parallel()
	r := rc()
	_, err := rcRequireString(r, "required_key")
	if err == nil {
		t.Fatal("expected error for missing required key")
	}
}

// --- stub managers for action node tests ---

type stubTagManager struct {
	addedTag string
}

func (s *stubTagManager) AddToAsset(_ context.Context, _, _, tagName string) (string, error) {
	s.addedTag = tagName
	return tagName, nil
}

type stubAssetManager struct {
	movedFolderID  *string
	movedProjectID *string
}

func (s *stubAssetManager) Get(_ context.Context, _, _ string) (*Asset, error) {
	return &Asset{ID: "ast_1", WorkspaceID: "ws_1", MimeType: "image/jpeg"}, nil
}

func (s *stubAssetManager) Move(_ context.Context, _, _ string, p AssetMoveParams) (*Asset, error) {
	s.movedFolderID = p.FolderID
	s.movedProjectID = p.ProjectID
	return &Asset{ID: "ast_1", WorkspaceID: "ws_1", FolderID: p.FolderID, ProjectID: p.ProjectID}, nil
}

type stubShareManager struct {
	lastShareID string
}

func (s *stubShareManager) Create(_ context.Context, _ string, _ ShareCreateParams) (string, error) {
	s.lastShareID = "sh_new"
	return s.lastShareID, nil
}

// --- action node tests ---

func TestActionTag_Execute_OK(t *testing.T) {
	t.Parallel()
	tags := &stubTagManager{}
	node := tagAssetNode{
		deps: Deps{Tags: tags},
		schema: actionSchema("action.tag", "Tag Asset", ".",
			mustConfigSchema(`{"type":"object","properties":{"name":{"type":"string"}}}`)),
	}
	triggerRC := rc("workspace_id", "ws_1", "asset_id", "ast_1", "workflow_created_by", "usr_1")
	port, updates, err := node.Execute(context.Background(), triggerRC, cfg(`{"name":"hero"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != portOut {
		t.Errorf("expected portOut, got %q", port)
	}
	if updates["tag_name"] != "hero" {
		t.Errorf("expected tag_name=hero in updates, got %v", updates)
	}
	if tags.addedTag != "hero" {
		t.Errorf("expected stub to record tag hero, got %q", tags.addedTag)
	}
}

func TestActionMoveFolder_Execute_OK(t *testing.T) {
	t.Parallel()
	assets := &stubAssetManager{}
	folder := "fld_42"
	node := moveAssetNode{
		deps: Deps{Assets: assets},
		schema: actionSchema("action.move_folder", "Move Asset", ".",
			mustConfigSchema(`{"type":"object","properties":{}}`)),
	}
	triggerRC := rc("workspace_id", "ws_1", "asset_id", "ast_1")
	port, updates, err := node.Execute(context.Background(), triggerRC, cfg(`{"folder_id":"`+folder+`"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != portOut {
		t.Errorf("expected portOut, got %q", port)
	}
	if assets.movedFolderID == nil || *assets.movedFolderID != folder {
		t.Errorf("expected folder_id=%q to be moved, got %v", folder, assets.movedFolderID)
	}
	_ = updates
}

func TestActionShare_Execute_OK(t *testing.T) {
	t.Parallel()
	shares := &stubShareManager{}
	node := createShareNode{
		deps: Deps{Shares: shares},
		schema: actionSchema("action.share", "Create Share", ".",
			mustConfigSchema(`{"type":"object","properties":{}}`)),
	}
	triggerRC := rc("workspace_id", "ws_1", "asset_id", "ast_1", "workflow_created_by", "usr_1")
	port, updates, err := node.Execute(
		context.Background(),
		triggerRC,
		cfg(`{"label":"My Share","allow_download":true}`),
	)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != portOut {
		t.Errorf("expected portOut, got %q", port)
	}
	if updates["share_id"] == "" {
		t.Errorf("expected non-empty share_id in updates, got %v", updates)
	}
}
