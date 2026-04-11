package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/exifextract"
	"damask/server/internal/queue"
)

// ExtractExifPayload is the payload for the extract_exif job.
type ExtractExifPayload struct {
	AssetID     string `json:"asset_id"`
	WorkspaceID string `json:"workspace_id"`
	UserID      string `json:"user_id"` // required: field_definitions.created_by and asset_field_values.created_by are NOT NULL
}

// EnqueueExtractExifJob enqueues an extract_exif job for an image asset.
func EnqueueExtractExifJob(ctx context.Context, q queue.JobQueue, workspaceID, assetID, userID string) error {
	payload, _ := json.Marshal(ExtractExifPayload{
		AssetID:     assetID,
		WorkspaceID: workspaceID,
		UserID:      userID,
	})
	_, err := q.Enqueue(ctx, workspaceID, queue.JobTypeExtractExif, string(payload))
	return err
}

// jobExtractExif is the handler for extract_exif jobs.
func (s *JobServer) jobExtractExif(ctx context.Context, job dbgen.Job) error {
	var p ExtractExifPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	// Load workspace and check exif_keep.
	ws, err := s.db.GetWorkspaceByID(ctx, p.WorkspaceID)
	if err != nil {
		return fmt.Errorf("load workspace: %w", err)
	}
	if ws.ExifKeep == 0 {
		return nil // EXIF extraction disabled for this workspace
	}

	// Load asset and verify it's an image.
	asset, err := s.db.GetAssetByID(ctx, dbgen.GetAssetByIDParams{
		ID:          p.AssetID,
		WorkspaceID: p.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil // asset deleted, nothing to do
		}
		return fmt.Errorf("load asset: %w", err)
	}
	if !strings.HasPrefix(asset.MimeType, "image/") {
		return nil
	}

	keepGPS := ws.ExifKeepGps == 1

	// Ensure system EXIF field definitions exist.
	fieldIDs, err := s.ensureExifFields(ctx, p.WorkspaceID, p.UserID, keepGPS)
	if err != nil {
		return fmt.Errorf("ensure exif fields: %w", err)
	}

	// Check tombstone: if the _exif_make field value exists for this asset, skip.
	makeFieldID, ok := fieldIDs["_exif_make"]
	if !ok {
		return fmt.Errorf("_exif_make field not found after ensureExifFields")
	}
	_, tombErr := s.db.GetAssetFieldValueByAssetAndField(ctx, dbgen.GetAssetFieldValueByAssetAndFieldParams{
		AssetID: p.AssetID,
		FieldID: makeFieldID,
	})
	if tombErr == nil {
		return nil // already extracted (tombstone present)
	}
	if !errors.Is(tombErr, sql.ErrNoRows) {
		return fmt.Errorf("check tombstone: %w", tombErr)
	}

	// Open asset from storage.
	r, err := s.storage.Get(asset.StorageKey)
	if err != nil {
		return fmt.Errorf("open asset: %w", err)
	}
	defer r.Close()

	// Extract EXIF.
	result, err := exifextract.Extract(r, keepGPS)
	if err != nil {
		log.Printf("exif: extract error asset=%s: %v — writing tombstone", p.AssetID, err)
	}

	if result == nil {
		// No EXIF or extraction error — write tombstone on _exif_make with empty value.
		empty := ""
		if _, uErr := s.db.UpsertAssetFieldValue(ctx, dbgen.UpsertAssetFieldValueParams{
			ID:        uuid.New().String(),
			AssetID:   p.AssetID,
			FieldID:   makeFieldID,
			ValueText: &empty,
			CreatedBy: p.UserID,
		}); uErr != nil {
			return fmt.Errorf("write tombstone: %w", uErr)
		}
		log.Printf("exif: no data asset=%s — tombstone written", p.AssetID)
		return nil
	}

	// Write field values.
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
		fid, ok := fieldIDs[f.key]
		if !ok {
			continue
		}
		if _, uErr := s.db.UpsertAssetFieldValue(ctx, dbgen.UpsertAssetFieldValueParams{
			ID:        uuid.New().String(),
			AssetID:   p.AssetID,
			FieldID:   fid,
			ValueText: f.val,
			CreatedBy: p.UserID,
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
		fid, ok := fieldIDs[f.key]
		if !ok {
			continue
		}
		if _, uErr := s.db.UpsertAssetFieldValue(ctx, dbgen.UpsertAssetFieldValueParams{
			ID:          uuid.New().String(),
			AssetID:     p.AssetID,
			FieldID:     fid,
			ValueNumber: f.val,
			CreatedBy:   p.UserID,
		}); uErr != nil {
			return fmt.Errorf("upsert %s: %w", f.key, uErr)
		}
	}

	if result.TakenAt != nil {
		fid, ok := fieldIDs["_exif_taken_at"]
		if ok {
			v := result.TakenAt.Format("2006-01-02")
			if _, uErr := s.db.UpsertAssetFieldValue(ctx, dbgen.UpsertAssetFieldValueParams{
				ID:        uuid.New().String(),
				AssetID:   p.AssetID,
				FieldID:   fid,
				ValueDate: &v,
				CreatedBy: p.UserID,
			}); uErr != nil {
				return fmt.Errorf("upsert _exif_taken_at: %w", uErr)
			}
		}
	}

	log.Printf("exif: extracted asset=%s make=%v model=%v gps=%v",
		p.AssetID, ptrStr(result.Make), ptrStr(result.Model), result.GPS != nil)
	return nil
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// exifFieldDef describes a system EXIF field definition.
type exifFieldDef struct {
	key       string
	name      string
	fieldType string
	gpsOnly   bool
}

var exifFields = []exifFieldDef{
	{"_exif_make", "Camera maker", "text", false},
	{"_exif_model", "Camera model", "text", false},
	{"_exif_lens", "Lens", "text", false},
	{"_exif_software", "Software", "text", false},
	{"_exif_exposure_time", "Shutter speed", "text", false},
	{"_exif_f_number", "Aperture", "number", false},
	{"_exif_iso", "ISO", "number", false},
	{"_exif_focal_length", "Focal length (mm)", "number", false},
	{"_exif_focal_length_35", "Focal length 35mm equiv.", "number", false},
	{"_exif_flash", "Flash", "text", false},
	{"_exif_white_balance", "White balance", "text", false},
	{"_exif_taken_at", "Date taken", "date", false},
	{"_exif_gps_lat", "GPS latitude", "number", true},
	{"_exif_gps_lng", "GPS longitude", "number", true},
}

// ensureExifFields creates missing system EXIF field definitions for the workspace
// and returns a map of key → field ID. Idempotent — safe to call on every job run.
func (s *JobServer) ensureExifFields(ctx context.Context, workspaceID, userID string, keepGPS bool) (map[string]string, error) {
	fieldIDs := make(map[string]string)

	for i, fd := range exifFields {
		if fd.gpsOnly && !keepGPS {
			continue
		}

		existing, err := s.db.GetFieldDefinitionByKey(ctx, dbgen.GetFieldDefinitionByKeyParams{
			WorkspaceID: workspaceID,
			Key:         fd.key,
		})
		if err == nil {
			fieldIDs[fd.key] = existing.ID
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("get field %s: %w", fd.key, err)
		}

		// Create it.
		created, err := s.db.CreateFieldDefinition(ctx, dbgen.CreateFieldDefinitionParams{
			ID:          uuid.New().String(),
			WorkspaceID: workspaceID,
			CreatedBy:   userID,
			Scope:       "asset",
			Name:        fd.name,
			Key:         fd.key,
			FieldType:   fd.fieldType,
			Position:    int64(1000 + i), // push below user fields
		})
		if err != nil {
			return nil, fmt.Errorf("create field %s: %w", fd.key, err)
		}
		fieldIDs[fd.key] = created.ID
	}

	return fieldIDs, nil
}
