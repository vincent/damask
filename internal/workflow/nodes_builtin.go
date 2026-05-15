package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"damask/server/internal/apperr"
)

func init() {
	triggerSchema := func(nodeType, label, desc string) NodeSchema {
		return NodeSchema{
			Type:        nodeType,
			Label:       label,
			Category:    "trigger",
			Description: desc,
			Outputs:     []Port{{ID: "out", Label: "Out"}, {ID: "error", Label: "Error"}},
		}
	}
	filterSchema := func(nodeType, label, desc string) NodeSchema {
		return NodeSchema{
			Type:        nodeType,
			Label:       label,
			Category:    "filter",
			Description: desc,
			Inputs:      []Port{{ID: "in", Label: "In"}},
			Outputs:     []Port{{ID: "match", Label: "Match"}, {ID: "no_match", Label: "No match"}, {ID: "error", Label: "Error"}},
		}
	}
	actionSchema := func(nodeType, label, desc string) NodeSchema {
		return NodeSchema{
			Type:        nodeType,
			Label:       label,
			Category:    "action",
			Description: desc,
			Inputs:      []Port{{ID: "in", Label: "In"}},
			Outputs:     []Port{{ID: "out", Label: "Out"}, {ID: "error", Label: "Error"}},
		}
	}

	registerPassThroughTrigger := func(nodeType, label, desc string) {
		Register(triggerSchema(nodeType, label, desc), func(Deps) Node { return passThroughNode{schema: triggerSchema(nodeType, label, desc)} })
	}
	registerPassThroughTrigger("trigger.manual", "Manual Trigger", "Starts a workflow manually.")
	registerPassThroughTrigger("trigger.asset_created", "Asset Created", "Starts when an asset is uploaded.")
	registerPassThroughTrigger("trigger.version_uploaded", "Version Uploaded", "Starts when a new asset version is uploaded.")
	registerPassThroughTrigger("trigger.tag_added", "Tag Added", "Starts when a tag is added to an asset.")
	registerPassThroughTrigger("trigger.schedule", "Schedule Trigger", "Starts on a scheduler tick.")
	registerPassThroughTrigger("trigger.webhook", "Webhook Trigger", "Starts from an inbound webhook.")

	Register(filterSchema("filter.mime", "Filter MIME Type", "Routes based on MIME type."), func(Deps) Node {
		return filterNode{schema: filterSchema("filter.mime", "Filter MIME Type", "Routes based on MIME type."), matchFn: matchMime}
	})
	Register(filterSchema("filter.filename", "Filter Filename", "Routes based on filename."), func(Deps) Node {
		return filterNode{schema: filterSchema("filter.filename", "Filter Filename", "Routes based on filename."), matchFn: matchFilename}
	})
	Register(filterSchema("filter.size", "Filter Size", "Routes based on file size."), func(Deps) Node {
		return filterNode{schema: filterSchema("filter.size", "Filter Size", "Routes based on file size."), matchFn: matchSize}
	})
	Register(filterSchema("filter.tag", "Filter Tag", "Routes based on tag name."), func(Deps) Node {
		return filterNode{schema: filterSchema("filter.tag", "Filter Tag", "Routes based on tag name."), matchFn: matchTag}
	})
	Register(filterSchema("filter.folder", "Filter Folder", "Routes based on folder id."), func(Deps) Node {
		return filterNode{schema: filterSchema("filter.folder", "Filter Folder", "Routes based on folder id."), matchFn: matchFolder}
	})
	Register(filterSchema("filter.expression", "Filter Expression", "Routes based on a key/value comparison."), func(Deps) Node {
		return filterNode{schema: filterSchema("filter.expression", "Filter Expression", "Routes based on a key/value comparison."), matchFn: matchExpression}
	})

	Register(actionSchema("action.create_variant", "Create Variant", "Queues a new variant job."), func(deps Deps) Node {
		return createVariantNode{deps: deps, schema: actionSchema("action.create_variant", "Create Variant", "Queues a new variant job.")}
	})
	Register(actionSchema("action.share", "Create Share", "Creates a share for the asset."), func(deps Deps) Node {
		return createShareNode{deps: deps, schema: actionSchema("action.share", "Create Share", "Creates a share for the asset.")}
	})
	Register(actionSchema("action.tag", "Tag Asset", "Adds a tag to the asset."), func(deps Deps) Node {
		return tagAssetNode{deps: deps, schema: actionSchema("action.tag", "Tag Asset", "Adds a tag to the asset.")}
	})
	Register(actionSchema("action.move_folder", "Move Asset", "Moves the asset to a folder or project."), func(deps Deps) Node {
		return moveAssetNode{deps: deps, schema: actionSchema("action.move_folder", "Move Asset", "Moves the asset to a folder or project.")}
	})
	Register(actionSchema("action.set_field", "Set Asset Field", "Sets a custom field value on the asset."), func(deps Deps) Node {
		return setFieldNode{deps: deps, schema: actionSchema("action.set_field", "Set Asset Field", "Sets a custom field value on the asset.")}
	})
	Register(actionSchema("control.fan_out", "Fan Out", "Forwards execution to every connected branch."), func(Deps) Node {
		return passThroughNode{schema: actionSchema("control.fan_out", "Fan Out", "Forwards execution to every connected branch.")}
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
	var c struct{ Prefix string `json:"prefix"` }
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
	if c.Extension != "" && strings.ToLower(filepath.Ext(name)) != strings.ToLower(c.Extension) {
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
	var c struct{ Name string `json:"name"` }
	_ = json.Unmarshal(cfg, &c)
	tagName, _ := rcGetString(rc, "tag_name")
	return strings.EqualFold(tagName, c.Name), nil
}

func matchFolder(rc *RunContext, cfg json.RawMessage) (bool, error) {
	var c struct{ FolderID string `json:"folder_id"` }
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
		Type   string          `json:"type"`
		Params json.RawMessage `json:"params"`
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
	})
	if err != nil {
		return "", nil, err
	}
	versionID, _ := rcRequireString(rc, "version_id")
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
	payload, _ := json.Marshal(VariantJobPayload{
		AssetID:     asset.ID,
		WorkspaceID: asset.WorkspaceID,
		VersionID:   versionID,
		VersionNum:  versionNum,
		StorageKey:  storageKey,
		MimeType:    asset.MimeType,
		Type:        prepared.Type,
		Params:      prepared.Params,
	})
	job, err := n.deps.Queue.Enqueue(ctx, workspaceID, prepared.Type, string(payload))
	if err != nil {
		return "", nil, err
	}
	return "out", map[string]any{"variant_job_id": job.ID, "variant_type": prepared.Type}, nil
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
	var nodeCfg struct{ Name string `json:"name"` }
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
