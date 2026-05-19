package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"damask/server/internal/apperr"

	"github.com/google/uuid"
)

func mustConfigSchema(raw string) json.RawMessage {
	return json.RawMessage(raw)
}

func triggerSchema(nodeType, label, desc string, configSchema json.RawMessage) NodeSchema {
	return NodeSchema{
		Type:         nodeType,
		Label:        label,
		Category:     "trigger",
		Description:  desc,
		Outputs:      []Port{{ID: "out", Label: "Out"}, {ID: "error", Label: "Error"}},
		ConfigSchema: configSchema,
	}
}

func filterSchema(nodeType, label, desc string, configSchema json.RawMessage) NodeSchema {
	return NodeSchema{
		Type:         nodeType,
		Label:        label,
		Category:     "filter",
		Description:  desc,
		Inputs:       []Port{{ID: "in", Label: "In"}},
		Outputs:      []Port{{ID: "match", Label: "Match"}, {ID: "no_match", Label: "No match"}, {ID: "error", Label: "Error"}},
		ConfigSchema: configSchema,
	}
}

func actionSchema(nodeType, label, desc string, configSchema json.RawMessage) NodeSchema {
	return NodeSchema{
		Type:         nodeType,
		Label:        label,
		Category:     "action",
		Description:  desc,
		Inputs:       []Port{{ID: "in", Label: "In"}},
		Outputs:      []Port{{ID: "out", Label: "Out"}, {ID: "error", Label: "Error"}},
		ConfigSchema: configSchema,
	}
}

func init() {
	registerPassThroughTrigger := func(nodeType, label, desc string, configSchema json.RawMessage) {
		schema := triggerSchema(nodeType, label, desc, configSchema)
		Register(schema, func(Deps) Node { return passThroughNode{schema: schema} })
	}
	registerPassThroughTrigger("trigger.manual", "Manual Trigger", "Starts a workflow manually.", mustConfigSchema(`{"type":"object","properties":{},"additionalProperties":false}`))
	registerPassThroughTrigger("trigger.asset_created", "Asset Created", "Starts when an asset is uploaded.", mustConfigSchema(`{"type":"object","properties":{"project_id":{"type":"string","title":"Project ID"},"folder_id":{"type":"string","title":"Folder ID","format":"folder"}},"additionalProperties":false}`))
	registerPassThroughTrigger("trigger.version_uploaded", "Version Uploaded", "Starts when a new asset version is uploaded.", mustConfigSchema(`{"type":"object","properties":{"asset_id":{"type":"string","title":"Asset ID"}},"additionalProperties":false}`))
	registerPassThroughTrigger("trigger.tag_added", "Tag Added", "Starts when a tag is added to an asset.", mustConfigSchema(`{"type":"object","properties":{"name":{"type":"string","title":"Tag Name","format":"tag"}},"required":["name"],"additionalProperties":false}`))
	registerPassThroughTrigger("trigger.schedule", "Schedule Trigger", "Starts on a scheduler tick.", mustConfigSchema(`{"type":"object","properties":{"cron":{"type":"string","title":"Cron","format":"cron"}},"required":["cron"],"additionalProperties":false}`))
	registerPassThroughTrigger("trigger.webhook", "Webhook Trigger", "Starts from an inbound webhook.", mustConfigSchema(`{"type":"object","properties":{},"additionalProperties":false}`))

	Register(filterSchema("filter.mime", "Filter MIME Type", "Routes based on MIME type.", mustConfigSchema(`{"type":"object","properties":{"prefix":{"type":"string","title":"MIME Prefix","placeholder":"image/"}},"required":["prefix"],"additionalProperties":false}`)), func(Deps) Node {
		return filterNode{schema: filterSchema("filter.mime", "Filter MIME Type", "Routes based on MIME type.", mustConfigSchema(`{"type":"object","properties":{"prefix":{"type":"string","title":"MIME Prefix","placeholder":"image/"}},"required":["prefix"],"additionalProperties":false}`)), matchFn: matchMime}
	})
	Register(filterSchema("filter.filename", "Filter Filename", "Routes based on filename.", mustConfigSchema(`{"type":"object","properties":{"contains":{"type":"string","title":"Contains"},"extension":{"type":"string","title":"Extension","placeholder":".pdf"}},"additionalProperties":false}`)), func(Deps) Node {
		return filterNode{schema: filterSchema("filter.filename", "Filter Filename", "Routes based on filename.", mustConfigSchema(`{"type":"object","properties":{"contains":{"type":"string","title":"Contains"},"extension":{"type":"string","title":"Extension","placeholder":".pdf"}},"additionalProperties":false}`)), matchFn: matchFilename}
	})
	Register(filterSchema("filter.size", "Filter Size", "Routes based on file size.", mustConfigSchema(`{"type":"object","properties":{"min":{"type":"number","title":"Min Bytes"},"max":{"type":"number","title":"Max Bytes"}},"additionalProperties":false}`)), func(Deps) Node {
		return filterNode{schema: filterSchema("filter.size", "Filter Size", "Routes based on file size.", mustConfigSchema(`{"type":"object","properties":{"min":{"type":"number","title":"Min Bytes"},"max":{"type":"number","title":"Max Bytes"}},"additionalProperties":false}`)), matchFn: matchSize}
	})
	Register(filterSchema("filter.tag", "Filter Tag", "Routes based on tag name.", mustConfigSchema(`{"type":"object","properties":{"name":{"type":"string","title":"Tag Name","format":"tag"}},"required":["name"],"additionalProperties":false}`)), func(Deps) Node {
		return filterNode{schema: filterSchema("filter.tag", "Filter Tag", "Routes based on tag name.", mustConfigSchema(`{"type":"object","properties":{"name":{"type":"string","title":"Tag Name","format":"tag"}},"required":["name"],"additionalProperties":false}`)), matchFn: matchTag}
	})
	Register(filterSchema("filter.folder", "Filter Folder", "Routes based on folder id.", mustConfigSchema(`{"type":"object","properties":{"folder_id":{"type":"string","title":"Folder ID","format":"folder"}},"required":["folder_id"],"additionalProperties":false}`)), func(Deps) Node {
		return filterNode{schema: filterSchema("filter.folder", "Filter Folder", "Routes based on folder id.", mustConfigSchema(`{"type":"object","properties":{"folder_id":{"type":"string","title":"Folder ID","format":"folder"}},"required":["folder_id"],"additionalProperties":false}`)), matchFn: matchFolder}
	})
	Register(filterSchema("filter.expression", "Filter Expression", "Routes based on a key/value comparison.", mustConfigSchema(`{"type":"object","properties":{"key":{"type":"string","title":"Context Key"},"value":{"type":"string","title":"Expected Value"}},"required":["key","value"],"additionalProperties":false}`)), func(Deps) Node {
		return filterNode{schema: filterSchema("filter.expression", "Filter Expression", "Routes based on a key/value comparison.", mustConfigSchema(`{"type":"object","properties":{"key":{"type":"string","title":"Context Key"},"value":{"type":"string","title":"Expected Value"}},"required":["key","value"],"additionalProperties":false}`)), matchFn: matchExpression}
	})

	Register(actionSchema("action.create_variant", "Create Variant", "Queues a new variant job.", mustConfigSchema(`{"type":"object","properties":{"type":{"type":"string","title":"Variant Type","format":"variant"},"params":{"type":"object","title":"Params","format":"json"},"title":{"type":"string","title":"Title"},"is_shared":{"type":"boolean","title":"Shared"}},"required":["type"],"additionalProperties":false}`)), func(deps Deps) Node {
		return createVariantNode{deps: deps, schema: actionSchema("action.create_variant", "Create Variant", "Queues a new variant job.", mustConfigSchema(`{"type":"object","properties":{"type":{"type":"string","title":"Variant Type","format":"variant"},"params":{"type":"object","title":"Params","format":"json"},"title":{"type":"string","title":"Title"},"is_shared":{"type":"boolean","title":"Shared"}},"required":["type"],"additionalProperties":false}`))}
	})
	Register(actionSchema("action.share", "Create Share", "Creates a share for the asset.", mustConfigSchema(`{"type":"object","properties":{"label":{"type":"string","title":"Label"},"allow_comments":{"type":"boolean","title":"Allow Comments"},"allow_download":{"type":"boolean","title":"Allow Download"},"expires_in_days":{"type":"number","title":"Expires In Days"}},"additionalProperties":false}`)), func(deps Deps) Node {
		return createShareNode{deps: deps, schema: actionSchema("action.share", "Create Share", "Creates a share for the asset.", mustConfigSchema(`{"type":"object","properties":{"label":{"type":"string","title":"Label"},"allow_comments":{"type":"boolean","title":"Allow Comments"},"allow_download":{"type":"boolean","title":"Allow Download"},"expires_in_days":{"type":"number","title":"Expires In Days"}},"additionalProperties":false}`))}
	})
	Register(actionSchema("action.tag", "Tag Asset", "Adds a tag to the asset.", mustConfigSchema(`{"type":"object","properties":{"name":{"type":"string","title":"Tag Name","format":"tag"}},"required":["name"],"additionalProperties":false}`)), func(deps Deps) Node {
		return tagAssetNode{deps: deps, schema: actionSchema("action.tag", "Tag Asset", "Adds a tag to the asset.", mustConfigSchema(`{"type":"object","properties":{"name":{"type":"string","title":"Tag Name","format":"tag"}},"required":["name"],"additionalProperties":false}`))}
	})
	Register(actionSchema("action.move_folder", "Move Asset", "Moves the asset to a folder or project.", mustConfigSchema(`{"type":"object","properties":{"folder_id":{"type":"string","title":"Folder ID","format":"folder"},"project_id":{"type":"string","title":"Project ID"}},"additionalProperties":false}`)), func(deps Deps) Node {
		return moveAssetNode{deps: deps, schema: actionSchema("action.move_folder", "Move Asset", "Moves the asset to a folder or project.", mustConfigSchema(`{"type":"object","properties":{"folder_id":{"type":"string","title":"Folder ID","format":"folder"},"project_id":{"type":"string","title":"Project ID"}},"additionalProperties":false}`))}
	})
	Register(actionSchema("action.set_field", "Set Asset Field", "Sets a custom field value on the asset.", mustConfigSchema(`{"type":"object","properties":{"field_id":{"type":"string","title":"Field ID"},"value":{"title":"Value","format":"json"}},"required":["field_id"],"additionalProperties":false}`)), func(deps Deps) Node {
		return setFieldNode{deps: deps, schema: actionSchema("action.set_field", "Set Asset Field", "Sets a custom field value on the asset.", mustConfigSchema(`{"type":"object","properties":{"field_id":{"type":"string","title":"Field ID"},"value":{"title":"Value","format":"json"}},"required":["field_id"],"additionalProperties":false}`))}
	})
	Register(actionSchema("control.fan_out", "Fan Out", "Forwards execution to every connected branch.", mustConfigSchema(`{"type":"object","properties":{},"additionalProperties":false}`)), func(Deps) Node {
		return passThroughNode{schema: actionSchema("control.fan_out", "Fan Out", "Forwards execution to every connected branch.", mustConfigSchema(`{"type":"object","properties":{},"additionalProperties":false}`))}
	})
}

type passThroughNode struct{ schema NodeSchema }

func (n passThroughNode) Schema() NodeSchema { return n.schema }
func (n passThroughNode) Execute(_ context.Context, _ *RunContext, _ json.RawMessage) (string, map[string]any, error) {
	return "out", nil, nil
}

type filterNode struct {
	schema  NodeSchema
	matchFn func(*RunContext, json.RawMessage) (bool, error)
}

func (n filterNode) Schema() NodeSchema { return n.schema }
func (n filterNode) Execute(_ context.Context, rc *RunContext, cfg json.RawMessage) (string, map[string]any, error) {
	match, err := n.matchFn(rc, cfg)
	if err != nil {
		return "", nil, err
	}
	if match {
		return "match", nil, nil
	}
	return "no_match", nil, nil
}

func matchMime(rc *RunContext, cfg json.RawMessage) (bool, error) {
	var c struct {
		Prefix string `json:"prefix"`
	}
	_ = json.Unmarshal(cfg, &c)
	mimeType, _ := rcGetString(rc, "mime_type")
	return strings.HasPrefix(mimeType, c.Prefix), nil
}

func matchFilename(rc *RunContext, cfg json.RawMessage) (bool, error) {
	var c struct {
		Contains  string `json:"contains"`
		Extension string `json:"extension"`
	}
	_ = json.Unmarshal(cfg, &c)
	name, _ := rcGetString(rc, "filename")
	if name == "" {
		name, _ = rcGetString(rc, "original_filename")
	}
	if c.Contains != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(c.Contains)) {
		return false, nil
	}
	if c.Extension != "" && !strings.EqualFold(filepath.Ext(name), c.Extension) {
		return false, nil
	}
	return true, nil
}

func matchSize(rc *RunContext, cfg json.RawMessage) (bool, error) {
	var c struct {
		Min *float64 `json:"min"`
		Max *float64 `json:"max"`
	}
	_ = json.Unmarshal(cfg, &c)
	val, ok := rc.Get("size")
	if !ok {
		return false, nil
	}
	size, ok := val.(float64)
	if !ok {
		return false, nil
	}
	if c.Min != nil && size < *c.Min {
		return false, nil
	}
	if c.Max != nil && size > *c.Max {
		return false, nil
	}
	return true, nil
}

func matchTag(rc *RunContext, cfg json.RawMessage) (bool, error) {
	var c struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(cfg, &c)
	tagName, _ := rcGetString(rc, "tag_name")
	return strings.EqualFold(tagName, c.Name), nil
}

func matchFolder(rc *RunContext, cfg json.RawMessage) (bool, error) {
	var c struct {
		FolderID string `json:"folder_id"`
	}
	_ = json.Unmarshal(cfg, &c)
	folderID, _ := rcGetString(rc, "folder_id")
	return folderID == c.FolderID, nil
}

func matchExpression(rc *RunContext, cfg json.RawMessage) (bool, error) {
	var c struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	_ = json.Unmarshal(cfg, &c)
	val, _ := rcGetString(rc, c.Key)
	return val == c.Value, nil
}

type createVariantNode struct {
	deps   Deps
	schema NodeSchema
}

func (n createVariantNode) Schema() NodeSchema { return n.schema }
func (n createVariantNode) Execute(ctx context.Context, rc *RunContext, cfg json.RawMessage) (string, map[string]any, error) {
	assetID, err := rcRequireString(rc, "asset_id")
	if err != nil {
		return "", nil, err
	}
	workspaceID, err := rcRequireString(rc, "workspace_id")
	if err != nil {
		return "", nil, err
	}
	if n.deps.Assets == nil || n.deps.Variants == nil || n.deps.Workspace == nil || n.deps.Queue == nil || n.deps.Config == nil {
		return "", nil, fmt.Errorf("workflow create_variant dependencies not configured")
	}
	asset, err := n.deps.Assets.Get(ctx, workspaceID, assetID)
	if err != nil {
		return "", nil, err
	}
	if asset.CurrentVersionID == nil {
		return "", nil, fmt.Errorf("asset has no current version: %w", apperr.ErrInvalidInput)
	}
	currentVer, err := n.deps.Workspace.GetImageRouterKeyStatus(ctx, workspaceID)
	if err != nil {
		return "", nil, err
	}
	var nodeCfg struct {
		Type     string          `json:"type"`
		Params   json.RawMessage `json:"params"`
		Title    *string         `json:"title,omitempty"`
		IsShared bool            `json:"is_shared"`
	}
	if err := json.Unmarshal(cfg, &nodeCfg); err != nil {
		return "", nil, fmt.Errorf("invalid node config: %w", apperr.ErrInvalidInput)
	}
	prepared, err := n.deps.Variants.PrepareCreate(ctx, VariantPrepareRequest{
		WorkspaceID:           workspaceID,
		AssetID:               assetID,
		Type:                  nodeCfg.Type,
		Params:                nodeCfg.Params,
		AssetMimeType:         asset.MimeType,
		ImageRouterConfigured: currentVer,
		DefaultImageModel:     n.deps.Config.ImageRouter.DefaultModel,
		DefaultBgRemoveModel:  n.deps.Config.ImageRouter.DefaultBgRemoveModel,
		Title:                 nodeCfg.Title,
		IsShared:              nodeCfg.IsShared,
	})
	if err != nil {
		return "", nil, err
	}
	versionID, err := rcRequireString(rc, "version_id")
	if err != nil {
		return "", nil, err
	}
	versionNum := int64(0)
	if v, ok := rc.Get("version_num"); ok {
		switch x := v.(type) {
		case float64:
			versionNum = int64(x)
		case int64:
			versionNum = x
		}
	}
	storageKey, _ := rcGetString(rc, "storage_key")
	variantID := uuid.NewString()
	payload, _ := json.Marshal(VariantJobPayload{
		AssetID:     asset.ID,
		WorkspaceID: asset.WorkspaceID,
		VersionID:   versionID,
		VersionNum:  versionNum,
		VariantID:   variantID,
		StorageKey:  storageKey,
		MimeType:    asset.MimeType,
		Type:        prepared.Type,
		Params:      prepared.Params,
		Title:       prepared.Title,
		IsShared:    prepared.IsShared,
	})
	job, err := n.deps.Queue.Enqueue(ctx, workspaceID, prepared.Type, string(payload))
	if err != nil {
		return "", nil, err
	}
	return "out", map[string]any{"variant_id": variantID, "variant_job_id": job.ID, "variant_type": prepared.Type}, nil
}

type createShareNode struct {
	deps   Deps
	schema NodeSchema
}

func (n createShareNode) Schema() NodeSchema { return n.schema }
func (n createShareNode) Execute(ctx context.Context, rc *RunContext, cfg json.RawMessage) (string, map[string]any, error) {
	assetID, err := rcRequireString(rc, "asset_id")
	if err != nil {
		return "", nil, err
	}
	workspaceID, err := rcRequireString(rc, "workspace_id")
	if err != nil {
		return "", nil, err
	}
	var nodeCfg struct {
		Label         string `json:"label"`
		AllowComments bool   `json:"allow_comments"`
		AllowDownload bool   `json:"allow_download"`
		ExpiresInDays *int   `json:"expires_in_days"`
	}
	if err := json.Unmarshal(cfg, &nodeCfg); err != nil {
		return "", nil, fmt.Errorf("invalid node config: %w", apperr.ErrInvalidInput)
	}
	createdBy := actorUserID(ctx, rc)
	if createdBy == "" {
		return "", nil, fmt.Errorf("workflow_created_by is required for share creation: %w", apperr.ErrInvalidInput)
	}
	shareID, err := n.deps.Shares.Create(ctx, workspaceID, ShareCreateParams{
		CreatedBy:     createdBy,
		Label:         nodeCfg.Label,
		TargetType:    "asset",
		TargetID:      assetID,
		ExpiresInDays: nodeCfg.ExpiresInDays,
		AllowComments: nodeCfg.AllowComments,
		AllowDownload: nodeCfg.AllowDownload,
	})
	if err != nil {
		return "", nil, err
	}
	return "out", map[string]any{"share_id": shareID}, nil
}

type tagAssetNode struct {
	deps   Deps
	schema NodeSchema
}

func (n tagAssetNode) Schema() NodeSchema { return n.schema }
func (n tagAssetNode) Execute(ctx context.Context, rc *RunContext, cfg json.RawMessage) (string, map[string]any, error) {
	workspaceID, err := rcRequireString(rc, "workspace_id")
	if err != nil {
		return "", nil, err
	}
	assetID, err := rcRequireString(rc, "asset_id")
	if err != nil {
		return "", nil, err
	}
	var nodeCfg struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(cfg, &nodeCfg); err != nil {
		return "", nil, fmt.Errorf("invalid node config: %w", apperr.ErrInvalidInput)
	}
	tagName, err := n.deps.Tags.AddToAsset(ctx, workspaceID, assetID, nodeCfg.Name)
	if err != nil {
		return "", nil, err
	}
	return "out", map[string]any{"tag_name": tagName}, nil
}

type moveAssetNode struct {
	deps   Deps
	schema NodeSchema
}

func (n moveAssetNode) Schema() NodeSchema { return n.schema }
func (n moveAssetNode) Execute(ctx context.Context, rc *RunContext, cfg json.RawMessage) (string, map[string]any, error) {
	workspaceID, err := rcRequireString(rc, "workspace_id")
	if err != nil {
		return "", nil, err
	}
	assetID, err := rcRequireString(rc, "asset_id")
	if err != nil {
		return "", nil, err
	}
	var nodeCfg struct {
		FolderID  *string `json:"folder_id"`
		ProjectID *string `json:"project_id"`
	}
	if err := json.Unmarshal(cfg, &nodeCfg); err != nil {
		return "", nil, fmt.Errorf("invalid node config: %w", apperr.ErrInvalidInput)
	}
	asset, err := n.deps.Assets.Move(ctx, workspaceID, assetID, AssetMoveParams{
		FolderID:  nodeCfg.FolderID,
		ProjectID: nodeCfg.ProjectID,
	})
	if err != nil {
		return "", nil, err
	}
	return "out", map[string]any{"folder_id": asset.FolderID, "project_id": asset.ProjectID}, nil
}

type setFieldNode struct {
	deps   Deps
	schema NodeSchema
}

func (n setFieldNode) Schema() NodeSchema { return n.schema }
func (n setFieldNode) Execute(ctx context.Context, rc *RunContext, cfg json.RawMessage) (string, map[string]any, error) {
	workspaceID, err := rcRequireString(rc, "workspace_id")
	if err != nil {
		return "", nil, err
	}
	assetID, err := rcRequireString(rc, "asset_id")
	if err != nil {
		return "", nil, err
	}
	if n.deps.AssetFields == nil {
		return "", nil, fmt.Errorf("asset field service unavailable")
	}
	var nodeCfg struct {
		FieldID string `json:"field_id"`
		Value   any    `json:"value"`
	}
	if err := json.Unmarshal(cfg, &nodeCfg); err != nil {
		return "", nil, fmt.Errorf("invalid node config: %w", apperr.ErrInvalidInput)
	}
	userID := actorUserID(ctx, rc)
	if userID == "" {
		return "", nil, fmt.Errorf("workflow_created_by is required for field updates: %w", apperr.ErrInvalidInput)
	}
	err = n.deps.AssetFields.SetValues(ctx, workspaceID, assetID, userID, []FieldValueInput{{
		FieldID: nodeCfg.FieldID,
		Value:   nodeCfg.Value,
	}})
	if err != nil {
		return "", nil, err
	}
	return "out", map[string]any{"field_id": nodeCfg.FieldID, "field_value": nodeCfg.Value}, nil
}
