package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/media/contentmeta"
	"damask/server/internal/storage"
)

type exifFieldDef struct {
	key       string
	name      string
	fieldType string
	gpsOnly   bool
}

const (
	exifSource            = "exif"
	customFieldTypeNumber = "number"
	customFieldTypeText   = "text"
	startExifPosition     = 1000
)

var exifFields = []exifFieldDef{
	{"_exif_make", "Camera maker", customFieldTypeText, false},
	{"_exif_model", "Camera model", customFieldTypeText, false},
	{"_exif_lens", "Lens", customFieldTypeText, false},
	{"_exif_software", "Software", customFieldTypeText, false},
	{"_exif_exposure_time", "Shutter speed", customFieldTypeText, false},
	{"_exif_f_number", "Aperture", customFieldTypeNumber, false},
	{"_exif_iso", "ISO", customFieldTypeNumber, false},
	{"_exif_focal_length", "Focal length (mm)", customFieldTypeNumber, false},
	{"_exif_focal_length_35", "Focal length 35mm equiv.", customFieldTypeNumber, false},
	{"_exif_flash", "Flash", customFieldTypeText, false},
	{"_exif_white_balance", "White balance", customFieldTypeText, false},
	{"_exif_taken_at", "Date taken", "date", false},
	{"_exif_gps_lat", "GPS latitude", customFieldTypeNumber, true},
	{"_exif_gps_lng", "GPS longitude", customFieldTypeNumber, true},
}

// ExifService extracts EXIF metadata from image assets and stores it as field values.
type ExifService struct {
	queries *dbgen.Queries
	storage storage.Storage
}

func NewExifService(queries *dbgen.Queries, stor storage.Storage) *ExifService {
	return &ExifService{queries: queries, storage: stor}
}

func (s *ExifService) ExtractForAsset(ctx context.Context, workspaceID, assetID, userID string) error {
	ws, err := s.queries.GetWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("load workspace: %w", err)
	}
	if ws.ExifKeep == 0 {
		return nil
	}

	asset, err := s.queries.GetAssetByID(ctx, dbgen.GetAssetByIDParams{
		ID:          assetID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("load asset: %w", err)
	}
	if !strings.HasPrefix(asset.MimeType, "image/") {
		return nil
	}

	keepGPS := ws.ExifKeepGps == 1

	// Tombstone check before ensureFields to avoid N inserts on already-processed assets.
	if makeField, mfErr := s.queries.GetFieldDefinitionByKey(ctx, dbgen.GetFieldDefinitionByKeyParams{
		WorkspaceID: workspaceID,
		Key:         "_exif_make",
	}); mfErr == nil {
		_, tombErr := s.queries.GetAssetFieldValueByAssetAndField(ctx, dbgen.GetAssetFieldValueByAssetAndFieldParams{
			AssetID: assetID,
			FieldID: makeField.ID,
		})
		if tombErr == nil {
			return nil
		}
		if !errors.Is(tombErr, sql.ErrNoRows) {
			return fmt.Errorf("check tombstone: %w", tombErr)
		}
	}

	fieldIDs, err := s.ensureFields(ctx, workspaceID, userID, keepGPS)
	if err != nil {
		return fmt.Errorf("ensure exif fields: %w", err)
	}

	makeFieldID, ok := fieldIDs["_exif_make"]
	if !ok {
		return errors.New("_exif_make field not found after ensureFields")
	}

	r, err := s.storage.Get(asset.StorageKey)
	if err != nil {
		return fmt.Errorf("open asset: %w", err)
	}
	defer r.Close()

	result, err := contentmeta.ExtractImageEXIF(ctx, r, keepGPS)
	if err != nil {
		slog.WarnContext(ctx, "exif: extract error — writing tombstone", "asset_id", assetID, "error", err)
	}

	if result == nil {
		empty := ""
		if _, uErr := s.queries.UpsertAssetFieldValue(ctx, dbgen.UpsertAssetFieldValueParams{
			ID:        uuid.New().String(),
			AssetID:   assetID,
			FieldID:   makeFieldID,
			ValueText: &empty,
			CreatedBy: &userID,
		}); uErr != nil {
			return fmt.Errorf("write tombstone: %w", uErr)
		}
		slog.DebugContext(ctx, "exif: no data — tombstone written", "asset_id", assetID)
		return nil
	}

	type textField struct {
		key string
		val *string
	}
	type numField struct {
		key string
		val *float64
	}

	texts := []textField{
		{"_exif_make", result.Make},
		{"_exif_model", result.Model},
		{"_exif_lens", result.LensModel},
		{"_exif_software", result.Software},
		{"_exif_exposure_time", result.ExposureTime},
		{"_exif_flash", result.Flash},
		{"_exif_white_balance", result.WhiteBalance},
	}
	for _, f := range texts {
		fid, found := fieldIDs[f.key]
		if !found {
			continue
		}
		if _, uErr := s.queries.UpsertAssetFieldValue(ctx, dbgen.UpsertAssetFieldValueParams{
			ID:        uuid.New().String(),
			AssetID:   assetID,
			FieldID:   fid,
			ValueText: f.val,
			CreatedBy: &userID,
		}); uErr != nil {
			return fmt.Errorf("upsert %s: %w", f.key, uErr)
		}
	}

	var isoF *float64
	if result.ISO != nil {
		v := float64(*result.ISO)
		isoF = &v
	}
	nums := []numField{
		{"_exif_f_number", result.FNumber},
		{"_exif_iso", isoF},
		{"_exif_focal_length", result.FocalLength},
		{"_exif_focal_length_35", result.FocalLength35},
	}
	if keepGPS && result.GPS != nil {
		lat := result.GPS.Lat
		lng := result.GPS.Lng
		nums = append(nums,
			numField{"_exif_gps_lat", &lat},
			numField{"_exif_gps_lng", &lng},
		)
	}
	for _, f := range nums {
		fid, found := fieldIDs[f.key]
		if !found {
			continue
		}
		if _, uErr := s.queries.UpsertAssetFieldValue(ctx, dbgen.UpsertAssetFieldValueParams{
			ID:          uuid.New().String(),
			AssetID:     assetID,
			FieldID:     fid,
			ValueNumber: f.val,
			CreatedBy:   &userID,
		}); uErr != nil {
			return fmt.Errorf("upsert %s: %w", f.key, uErr)
		}
	}

	if result.TakenAt != nil {
		fid, found := fieldIDs["_exif_taken_at"]
		if found {
			v := result.TakenAt.Format("2006-01-02")
			if _, uErr := s.queries.UpsertAssetFieldValue(ctx, dbgen.UpsertAssetFieldValueParams{
				ID:        uuid.New().String(),
				AssetID:   assetID,
				FieldID:   fid,
				ValueDate: &v,
				CreatedBy: &userID,
			}); uErr != nil {
				return fmt.Errorf("upsert _exif_taken_at: %w", uErr)
			}
		}
	}

	slog.DebugContext(ctx, "exif: extracted",
		"asset_id", assetID,
		"make", ptrStr(result.Make),
		"model", ptrStr(result.Model),
		"gps", result.GPS != nil,
	)
	return nil
}

func (s *ExifService) ensureFields(
	ctx context.Context,
	workspaceID, _ string,
	keepGPS bool,
) (map[string]string, error) {
	for i, fd := range exifFields {
		if fd.gpsOnly && !keepGPS {
			continue
		}
		if err := s.queries.InsertSystemFieldDefinition(ctx, dbgen.InsertSystemFieldDefinitionParams{
			ID:          uuid.NewString(),
			WorkspaceID: workspaceID,
			Source:      exifSource,
			Name:        fd.name,
			Key:         fd.key,
			FieldType:   fd.fieldType,
			Position:    int64(startExifPosition + i),
		}); err != nil {
			return nil, fmt.Errorf("ensure exif field %s: %w", fd.key, err)
		}
	}

	fields, err := s.queries.GetSystemFieldsBySource(ctx, dbgen.GetSystemFieldsBySourceParams{
		WorkspaceID: workspaceID,
		Source:      exifSource,
	})
	if err != nil {
		return nil, fmt.Errorf("load exif fields: %w", err)
	}

	fieldIDs := make(map[string]string, len(fields))
	for _, field := range fields {
		fieldIDs[field.Key] = field.ID
	}
	return fieldIDs, nil
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
