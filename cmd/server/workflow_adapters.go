package main

import (
	"context"

	"damask/server/internal/repository"
	"damask/server/internal/service"
	"damask/server/internal/workflow"
)

type assetAdapter struct{ svc service.AssetService }

func newAssetManager(svc service.AssetService) workflow.AssetManager {
	return assetAdapter{svc: svc}
}

func (a assetAdapter) Get(ctx context.Context, workspaceID, assetID string) (*workflow.Asset, error) {
	asset, err := a.svc.Get(ctx, workspaceID, assetID)
	if err != nil {
		return nil, err
	}
	return &workflow.Asset{
		ID:               asset.ID,
		WorkspaceID:      asset.WorkspaceID,
		MimeType:         asset.MimeType,
		CurrentVersionID: asset.CurrentVersionID,
		FolderID:         asset.FolderID,
		ProjectID:        asset.ProjectID,
	}, nil
}

func (a assetAdapter) Move(
	ctx context.Context,
	workspaceID, assetID string,
	p workflow.AssetMoveParams,
) (*workflow.Asset, error) {
	asset, err := a.svc.Move(ctx, workspaceID, assetID, service.MoveAssetParams{
		FolderID:  p.FolderID,
		ProjectID: p.ProjectID,
	})
	if err != nil {
		return nil, err
	}
	return &workflow.Asset{
		ID:               asset.ID,
		WorkspaceID:      asset.WorkspaceID,
		MimeType:         asset.MimeType,
		CurrentVersionID: asset.CurrentVersionID,
		FolderID:         asset.FolderID,
		ProjectID:        asset.ProjectID,
	}, nil
}

type variantAdapter struct{ svc service.VariantService }

func newVariantManager(svc service.VariantService) workflow.VariantManager {
	return variantAdapter{svc: svc}
}

func (a variantAdapter) GetVariantByID(ctx context.Context, workspaceID, id string) (repository.Variant, error) {
	dto, err := a.svc.Get(ctx, workspaceID, id)
	if err != nil {
		return repository.Variant{}, err
	}
	return repository.Variant{
		ID:         dto.ID,
		StorageKey: dto.StorageKey,
		Size:       dto.Size,
		Status:     dto.Status,
	}, nil
}

func (a variantAdapter) PrepareCreate(
	ctx context.Context,
	p workflow.VariantPrepareRequest,
) (workflow.VariantPrepareResult, error) {
	prepared, err := a.svc.PrepareCreate(ctx, service.PrepareCreateVariantParams{
		WorkspaceID:           p.WorkspaceID,
		AssetID:               p.AssetID,
		Type:                  p.Type,
		Params:                p.Params,
		AssetMimeType:         p.AssetMimeType,
		ImageRouterConfigured: p.ImageRouterConfigured,
		DefaultImageModel:     p.DefaultImageModel,
		DefaultBgRemoveModel:  p.DefaultBgRemoveModel,
		Title:                 p.Title,
		IsShared:              p.IsShared,
	})
	if err != nil {
		return workflow.VariantPrepareResult{}, err
	}
	return workflow.VariantPrepareResult{
		Type:     prepared.Type,
		Params:   prepared.Params,
		Title:    prepared.Title,
		IsShared: prepared.IsShared,
	}, nil
}

type shareAdapter struct{ svc service.ShareService }

func newShareManager(svc service.ShareService) workflow.ShareManager {
	return shareAdapter{svc: svc}
}

func (a shareAdapter) Create(ctx context.Context, workspaceID string, p workflow.ShareCreateParams) (string, error) {
	share, err := a.svc.Create(ctx, workspaceID, service.CreateShareParams{
		CreatedBy:     p.CreatedBy,
		Label:         p.Label,
		TargetType:    p.TargetType,
		TargetID:      p.TargetID,
		ExpiresInDays: p.ExpiresInDays,
		AllowComments: p.AllowComments,
		AllowDownload: p.AllowDownload,
	})
	if err != nil {
		return "", err
	}
	return share.ID, nil
}

type tagAdapter struct{ svc service.TagService }

func newTagManager(svc service.TagService) workflow.TagManager {
	return tagAdapter{svc: svc}
}

func (a tagAdapter) AddToAsset(ctx context.Context, workspaceID, assetID, tagName string) (string, error) {
	tag, err := a.svc.AddToAsset(ctx, workspaceID, assetID, tagName)
	if err != nil {
		return "", err
	}
	return tag.Name, nil
}

type assetFieldAdapter struct{ svc service.AssetFieldService }

func newAssetFieldManager(svc service.AssetFieldService) workflow.AssetFieldManager {
	return assetFieldAdapter{svc: svc}
}

func (a assetFieldAdapter) SetValues(
	ctx context.Context,
	workspaceID, assetID, userID string,
	inputs []workflow.FieldValueInput,
) error {
	serviceInputs := make([]service.SetFieldValueInput, len(inputs))
	for i, input := range inputs {
		serviceInputs[i] = service.SetFieldValueInput{FieldID: input.FieldID, Value: input.Value}
	}
	_, err := a.svc.SetValues(ctx, workspaceID, assetID, userID, serviceInputs)
	return err
}

type workspaceAdapter struct{ svc service.WorkspaceService }

func newWorkspaceManager(svc service.WorkspaceService) workflow.WorkspaceManager {
	return workspaceAdapter{svc: svc}
}

func (a workspaceAdapter) GetImageRouterKeyStatus(ctx context.Context, workspaceID string) (bool, error) {
	status, err := a.svc.GetImageRouterKeyStatus(ctx, workspaceID)
	if err != nil {
		return false, err
	}
	return status.KeySet, nil
}

type versionManagerAdapter struct {
	versions repository.VersionRepository
}

func newVersionManager(versions repository.VersionRepository) workflow.VersionManager {
	return versionManagerAdapter{versions: versions}
}

func (a versionManagerAdapter) GetByID(ctx context.Context, id string) (repository.AssetVersion, error) {
	return a.versions.GetByID(ctx, id)
}

func (a versionManagerAdapter) NextVersionNum(ctx context.Context, assetID string) (int64, error) {
	return a.versions.NextVersionNum(ctx, assetID)
}

func (a versionManagerAdapter) Create(ctx context.Context, v repository.AssetVersion) (repository.AssetVersion, error) {
	return a.versions.Create(ctx, v)
}

func (a versionManagerAdapter) SetCurrent(ctx context.Context, assetID, versionID string) error {
	return a.versions.SetCurrent(ctx, assetID, versionID)
}

func (a versionManagerAdapter) SetAssetThumbnail(ctx context.Context, assetID string, key *string) error {
	return a.versions.SetAssetThumbnail(ctx, assetID, key)
}

