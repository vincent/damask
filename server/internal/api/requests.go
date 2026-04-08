package api

import (
	"context"
	"encoding/json"
	"strings"
)

// -- auth.go ------------------------------------------------------------------

type RegisterRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r *RegisterRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.Name = strings.TrimSpace(r.Name)
	r.Email = strings.TrimSpace(r.Email)
	if r.Name == "" {
		p["name"] = "required"
	}
	if r.Email == "" {
		p["email"] = "required"
	}
	if r.Password == "" {
		p["password"] = "required"
	} else if len(r.Password) < 8 {
		p["password"] = "must be at least 8 characters"
	}
	return p
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r *loginRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if r.Email == "" {
		p["email"] = "required"
	}
	if r.Password == "" {
		p["password"] = "required"
	}
	return p
}

// -- workspace.go -------------------------------------------------------------

type createWorkspaceRequest struct {
	Name string `json:"name"`
}

func (r *createWorkspaceRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		p["name"] = "required"
	}
	return p
}

type createInviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"` // "editor" or "viewer"
}

func (r *createInviteRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if r.Email == "" {
		p["email"] = "required"
	}
	if r.Role == "" {
		r.Role = "editor"
	}
	if r.Role != "editor" && r.Role != "viewer" {
		p["role"] = "must be editor or viewer"
	}
	return p
}

type acceptInviteRequest struct {
	Token    string `json:"token"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (r *acceptInviteRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if r.Token == "" {
		p["token"] = "required"
	}
	if r.Name == "" {
		p["name"] = "required"
	}
	if r.Password == "" {
		p["password"] = "required"
	} else if len(r.Password) < 8 {
		p["password"] = "must be at least 8 characters"
	}
	return p
}

type switchWorkspaceRequest struct {
	WorkspaceID string `json:"workspace_id"`
}

func (r *switchWorkspaceRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if r.WorkspaceID == "" {
		p["workspace_id"] = "required"
	}
	return p
}

type updateWorkspaceSettingsRequest struct {
	VersionRetentionCount int64 `json:"version_retention_count"`
}

func (r *updateWorkspaceSettingsRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if r.VersionRetentionCount < 0 {
		p["version_retention_count"] = "must be 0 (keep all) or a positive integer"
	}
	return p
}

// -- projects.go --------------------------------------------------------------

type createProjectRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Color       *string `json:"color"`
}

func (r *createProjectRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		p["name"] = "required"
	}
	return p
}

type updateProjectRequest struct {
	Name         *string `json:"name"`
	Description  *string `json:"description"`
	Color        *string `json:"color"`
	CoverAssetID *string `json:"cover_asset_id"`
}

func (r *updateProjectRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if r.Name != nil {
		trimmed := strings.TrimSpace(*r.Name)
		if trimmed == "" {
			p["name"] = "cannot be empty"
		} else {
			r.Name = &trimmed
		}
	}
	return p
}

// -- folders.go ---------------------------------------------------------------

type createFolderRequest struct {
	Name     string  `json:"name"`
	ParentID *string `json:"parent_id"`
	Position int64   `json:"position"`
}

func (r *createFolderRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		p["name"] = "required"
	}
	return p
}

type updateFolderRequest struct {
	Name     *string `json:"name"`
	Position *int64  `json:"position"`
}

func (r *updateFolderRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if r.Name != nil {
		trimmed := strings.TrimSpace(*r.Name)
		if trimmed == "" {
			p["name"] = "cannot be empty"
		} else {
			r.Name = &trimmed
		}
	}
	return p
}

// -- assets.go ----------------------------------------------------------------

type BulkTagRequest struct {
	AssetIDs []string `json:"asset_ids"`
	TagName  string   `json:"tag_name"`
}

func (r *BulkTagRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.TagName = strings.TrimSpace(strings.ToLower(r.TagName))
	if r.TagName == "" {
		p["tag_name"] = "required"
	}
	if len(r.AssetIDs) == 0 {
		p["asset_ids"] = "required, must contain at least one id"
	}
	return p
}

type bulkProjectRequest struct {
	AssetIDs  []string `json:"asset_ids"`
	ProjectID *string  `json:"project_id"` // null = unassign
}

func (r *bulkProjectRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if len(r.AssetIDs) == 0 {
		p["asset_ids"] = "required"
	}
	return p
}

type updateAssetFolderRequest struct {
	FolderID *string `json:"folder_id"`
}

func (r *updateAssetFolderRequest) Valid(_ context.Context) map[string]string {
	return map[string]string{}
}

type renameAssetRequest struct {
	Name string `json:"name"`
}

func (r *renameAssetRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		p["name"] = "required"
	}
	return p
}

type bulkDeleteRequest struct {
	AssetIDs []string `json:"asset_ids"`
}

func (r *bulkDeleteRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if len(r.AssetIDs) == 0 {
		p["asset_ids"] = "required"
	}
	return p
}

// -- tags.go ------------------------------------------------------------------

type addTagRequest struct {
	Name string `json:"name"`
}

func (r *addTagRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.Name = strings.TrimSpace(strings.ToLower(r.Name))
	if r.Name == "" {
		p["name"] = "required"
	}
	return p
}

// -- shares.go ----------------------------------------------------------------

type createShareRequest struct {
	Label         string  `json:"label"`
	TargetType    string  `json:"target_type"`
	TargetID      string  `json:"target_id"`
	Password      *string `json:"password"`
	ExpiresInDays *int    `json:"expires_in_days"`
	AllowComments bool    `json:"allow_comments"`
	AllowDownload *bool   `json:"allow_download"`
}

func (r *createShareRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	validTargetTypes := map[string]bool{"collection": true, "asset": true, "project": true}
	if !validTargetTypes[r.TargetType] {
		p["target_type"] = "must be one of: collection, asset, project"
	}
	if r.TargetID == "" {
		p["target_id"] = "required"
	}
	return p
}

type updateShareRequest struct {
	Label         *string `json:"label"`
	Password      *string `json:"password"`       // empty string = remove password
	ClearPassword *bool   `json:"clear_password"` // explicit flag to remove password
	ExpiresAt     *string `json:"expires_at"`     // ISO string or null to clear
	ClearExpiry   *bool   `json:"clear_expiry"`
	AllowComments *bool   `json:"allow_comments"`
	AllowDownload *bool   `json:"allow_download"`
}

func (r *updateShareRequest) Valid(_ context.Context) map[string]string {
	return map[string]string{}
}

// -- shares_public.go ---------------------------------------------------------

type shareAccessRequest struct {
	Password string `json:"password"`
}

func (r *shareAccessRequest) Valid(_ context.Context) map[string]string {
	return map[string]string{}
}

type createCommentRequest struct {
	AssetID     string  `json:"asset_id"`
	AuthorName  string  `json:"author_name"`
	AuthorEmail *string `json:"author_email"`
	Body        string  `json:"body"`
}

func (r *createCommentRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.AuthorName = strings.TrimSpace(r.AuthorName)
	r.Body = strings.TrimSpace(r.Body)
	if r.AuthorName == "" {
		p["author_name"] = "required"
	}
	if r.Body == "" {
		p["body"] = "required"
	}
	if r.AssetID == "" {
		p["asset_id"] = "required"
	}
	return p
}

// -- variants.go --------------------------------------------------------------

type createVariantRequest struct {
	Type   string          `json:"type"`
	Params json.RawMessage `json:"params"`
}

func (r *createVariantRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if r.Type == "" {
		p["type"] = "required"
	}
	return p
}

// -- ingress.go ---------------------------------------------------------------

type ingressRuleReq struct {
	Position int64  `json:"position"`
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
	Action   string `json:"action"`
}

func (r *ingressRuleReq) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if r.Field == "" {
		p["field"] = "required"
	}
	if r.Operator == "" {
		p["operator"] = "required"
	}
	if r.Value == "" {
		p["value"] = "required"
	}
	if r.Action == "" {
		p["action"] = "required"
	}
	return p
}

type createIngressSourceReq struct {
	Type            string           `json:"type"`
	Label           string           `json:"label"`
	Config          map[string]any   `json:"config"`
	DestFolderID    *string          `json:"dest_folder_id"`
	DestProjectID   *string          `json:"dest_project_id"`
	Enabled         *bool            `json:"enabled"`
	PollIntervalMin int64            `json:"poll_interval_min"`
	Rules           []ingressRuleReq `json:"rules"`
}

func (r *createIngressSourceReq) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if r.Type == "" {
		p["type"] = "required"
	}
	if r.Label == "" {
		p["label"] = "required"
	}
	return p
}

type updateIngressSourceReq struct {
	Label           string           `json:"label"`
	Config          map[string]any   `json:"config"`
	DestFolderID    *json.RawMessage `json:"dest_folder_id"`
	DestProjectID   *json.RawMessage `json:"dest_project_id"`
	Enabled         *bool            `json:"enabled"`
	PollIntervalMin int64            `json:"poll_interval_min"`
}

func (r *updateIngressSourceReq) Valid(_ context.Context) map[string]string {
	return map[string]string{}
}

type reorderRuleEntry struct {
	ID       string `json:"id"`
	Position int64  `json:"position"`
}

// -- custom_fields_definitions.go (reorder) -----------------------------------

type ReorderFieldEntry struct {
	ID       string `json:"id"`
	Position int64  `json:"position"`
}

// reorderFieldEntry is the unexported alias for backward compatibility
type reorderFieldEntry = ReorderFieldEntry

type ReorderFieldDefinitionsRequest struct {
	Items []ReorderFieldEntry
}

// reorderFieldDefinitionsRequest is the unexported alias for backward compatibility
type reorderFieldDefinitionsRequest = ReorderFieldDefinitionsRequest

// UnmarshalJSON lets the client send a bare JSON array while still satisfying
// the Validator interface required by decodeAndValidate.
func (r *ReorderFieldDefinitionsRequest) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &r.Items)
}

func (r *ReorderFieldDefinitionsRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if len(r.Items) == 0 {
		p["items"] = "at least one item required"
	}
	return p
}

// -- custom_fields_definitions.go ---------------------------------------------

type CreateFieldDefinitionRequest struct {
	Scope              string  `json:"scope"`
	Name               string  `json:"name"`
	Key                string  `json:"key"`
	FieldType          string  `json:"field_type"`
	Options            *string `json:"options"`
	Required           bool    `json:"required"`
	Position           int64   `json:"position"`
	InheritFromProject bool    `json:"inherit_from_project"`
}

// createFieldDefinitionRequest is the unexported alias for backward compatibility
type createFieldDefinitionRequest = CreateFieldDefinitionRequest

func (r *CreateFieldDefinitionRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		p["name"] = "required"
	}
	if r.Scope != "asset" && r.Scope != "project" {
		p["scope"] = "must be 'asset' or 'project'"
	}
	validTypes := map[string]bool{"text": true, "number": true, "date": true, "boolean": true, "select": true, "url": true}
	if !validTypes[r.FieldType] {
		p["field_type"] = "must be one of: text, number, date, boolean, select, url"
	}
	if !keyRegexp.MatchString(r.Key) {
		p["key"] = "must match /^[a-z0-9_]+$/"
	}
	if r.FieldType == "select" {
		if r.Options == nil || *r.Options == "" {
			p["options"] = "required for select fields"
		} else {
			var opts []string
			if err := json.Unmarshal([]byte(*r.Options), &opts); err != nil || len(opts) == 0 {
				p["options"] = "must be a non-empty JSON array of strings"
			}
		}
	}
	return p
}

type UpdateFieldDefinitionRequest struct {
	Name               *string `json:"name"`
	Key                *string `json:"key"`
	FieldType          *string `json:"field_type"`
	Options            *string `json:"options"`
	Required           *bool   `json:"required"`
	Position           *int64  `json:"position"`
	InheritFromProject *bool   `json:"inherit_from_project"`
}

// updateFieldDefinitionRequest is the unexported alias for backward compatibility
type updateFieldDefinitionRequest = UpdateFieldDefinitionRequest

func (r *UpdateFieldDefinitionRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if r.Name != nil {
		trimmed := strings.TrimSpace(*r.Name)
		if trimmed == "" {
			p["name"] = "cannot be empty"
		} else {
			r.Name = &trimmed
		}
	}
	return p
}

// -- custom_fields_values.go --------------------------------------------------

type PatchAssetFieldsRequest struct {
	Values []FieldValueInput `json:"values"`
}

// patchAssetFieldsRequest is the unexported alias for backward compatibility
type patchAssetFieldsRequest = PatchAssetFieldsRequest

func (r *PatchAssetFieldsRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if len(r.Values) == 0 {
		p["values"] = "required"
	}
	return p
}

type BulkPatchAssetFieldsRequest struct {
	AssetIDs []string          `json:"asset_ids"`
	Values   []FieldValueInput `json:"values"`
}

// bulkPatchAssetFieldsRequest is the unexported alias for backward compatibility
type bulkPatchAssetFieldsRequest = BulkPatchAssetFieldsRequest

func (r *BulkPatchAssetFieldsRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if len(r.AssetIDs) == 0 {
		p["asset_ids"] = "required"
	}
	if len(r.Values) == 0 {
		p["values"] = "required"
	}
	return p
}

// -- custom_fields_project_values.go ------------------------------------------

type PatchProjectFieldsRequest struct {
	Values []FieldValueInput `json:"values"`
}

// patchProjectFieldsRequest is the unexported alias for backward compatibility
type patchProjectFieldsRequest = PatchProjectFieldsRequest

func (r *PatchProjectFieldsRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if len(r.Values) == 0 {
		p["values"] = "required"
	}
	return p
}
