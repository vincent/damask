package export

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
	"damask/server/internal/storage"
)

// BuildProgress reports incremental progress during Build.
type BuildProgress struct {
	AssetsExported int
	AssetsSkipped  int
	BytesWritten   int64
}

// BuildResult is returned by Build on success.
type BuildResult struct {
	AssetsTotal    int
	AssetsExported int
	AssetsSkipped  int
	BytesWritten   int64
	ManifestJSON   []byte
}

// BuildParams holds all dependencies needed by Build.
type BuildParams struct {
	Config     repository.ExportConfig
	Run        repository.ExportRun
	Dest       Destination
	Storage    storage.Storage
	Queries    *dbgen.Queries
	SQLite     *sql.DB
	AppSecret  string
	Project    dbgen.Project
	OnProgress func(p BuildProgress) // called every 10 assets, safe to nil
}

// remoteKey builds the incremental dedup key for an item.
func remoteKey(assetID string, versionNum int64, variantID string) string {
	return fmt.Sprintf("%s:%d:%s", assetID, versionNum, variantID)
}

// Build assembles the ZIP archive and writes it to the destination.
// The temp file is cleaned up automatically.
func Build(ctx context.Context, p BuildParams) (BuildResult, error) {
	// 1. Fetch remote sidecar manifest → remoteHashes
	sidecarPath := slugify(p.Project.Name) + "__manifest.json"
	remoteHashes := map[string]string{}
	manifestBytes, err := p.Dest.ReadManifest(ctx, sidecarPath)
	if err != nil {
		slog.WarnContext(ctx, "export: read remote manifest failed", "error", err)
	} else if manifestBytes != nil {
		var prev Manifest
		if jsonErr := json.Unmarshal(manifestBytes, &prev); jsonErr == nil {
			for _, a := range prev.Assets {
				for _, v := range a.Versions {
					remoteHashes[remoteKey(a.ID, v.VersionNum, "")] = v.ContentHash
					for _, vr := range v.Variants {
						remoteHashes[remoteKey(a.ID, v.VersionNum, vr.ID)] = vr.ContentHash
					}
				}
			}
		}
	}

	// 2. Decrypt dest_config (already done by caller for validation; builder just needs it for destination).
	// Destination is already constructed and passed in via BuildParams.Dest.

	// 3. Query assets based on versions setting.
	projectID := p.Config.ProjectID
	workspaceID := p.Config.WorkspaceID
	includeVariants := p.Config.IncludeVariants

	type assetVersionRow struct {
		assetID          string
		originalFilename string
		folderName       *string
		versionID        string
		versionNum       int64
		storageKey       string
		contentHash      string
		mimeType         string
		size             int64
		comment          *string
		versionCreatedAt string
		isCurrent        bool
	}

	var avRows []assetVersionRow

	if p.Config.Versions == "all" {
		rows, qErr := p.Queries.GetProjectAllVersionsForExport(ctx, dbgen.GetProjectAllVersionsForExportParams{
			ProjectID:   &projectID,
			WorkspaceID: workspaceID,
		})
		if qErr != nil {
			return BuildResult{}, fmt.Errorf("export: query all versions: %w", qErr)
		}
		for _, r := range rows {
			avRows = append(avRows, assetVersionRow{
				assetID:          r.AssetID,
				originalFilename: r.OriginalFilename,
				folderName:       r.FolderName,
				versionID:        r.VersionID,
				versionNum:       r.VersionNum,
				storageKey:       r.StorageKey,
				contentHash:      r.ContentHash,
				mimeType:         r.MimeType,
				size:             r.Size,
				comment:          r.Comment,
				versionCreatedAt: r.VersionCreatedAt,
				isCurrent:        r.IsCurrent == 1,
			})
		}
	} else {
		rows, qErr := p.Queries.GetProjectAssetsForExport(ctx, dbgen.GetProjectAssetsForExportParams{
			ProjectID:   &projectID,
			WorkspaceID: workspaceID,
		})
		if qErr != nil {
			return BuildResult{}, fmt.Errorf("export: query current versions: %w", qErr)
		}
		for _, r := range rows {
			avRows = append(avRows, assetVersionRow{
				assetID:          r.ID,
				originalFilename: r.OriginalFilename,
				folderName:       r.FolderName,
				versionID:        r.VersionID,
				versionNum:       r.VersionNum,
				storageKey:       r.StorageKey,
				contentHash:      r.ContentHash,
				mimeType:         r.MimeType,
				size:             r.Size,
				comment:          r.Comment,
				versionCreatedAt: r.VersionCreatedAt,
				isCurrent:        true,
			})
		}
	}

	// Tags and field values.
	tagRows, err := p.Queries.GetAssetTagsForProject(ctx, dbgen.GetAssetTagsForProjectParams{
		ProjectID:   &projectID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return BuildResult{}, fmt.Errorf("export: query tags: %w", err)
	}
	tagsMap := map[string][]string{}
	for _, tr := range tagRows {
		tagsMap[tr.AssetID] = append(tagsMap[tr.AssetID], tr.TagName)
	}

	fieldRows, err := p.Queries.GetFieldValuesForProject(ctx, dbgen.GetFieldValuesForProjectParams{
		ProjectID:   &projectID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return BuildResult{}, fmt.Errorf("export: query fields: %w", err)
	}
	fieldsMap := map[string][]ManifestFieldVal{}
	for _, fr := range fieldRows {
		var val any
		switch fr.FieldType {
		case "boolean":
			val = fr.ValueBoolean != nil && *fr.ValueBoolean == 1
		case "number":
			val = fr.ValueNumber
		case "date":
			val = fr.ValueDate
		default:
			val = fr.ValueText
		}
		fieldsMap[fr.AssetID] = append(fieldsMap[fr.AssetID], ManifestFieldVal{
			Name:  fr.FieldName,
			Type:  fr.FieldType,
			Value: val,
		})
	}

	// Variants per version.
	variantsMap := map[string][]dbgen.Variant{} // versionID → []Variant
	if includeVariants && len(avRows) > 0 {
		versionIDs := make([]string, 0, len(avRows))
		for _, r := range avRows {
			versionIDs = append(versionIDs, r.versionID)
		}
		variants, qErr := queryVariantsForVersionIDs(ctx, p.SQLite, versionIDs, workspaceID)
		if qErr != nil {
			return BuildResult{}, fmt.Errorf("export: query variants: %w", qErr)
		}
		for _, v := range variants {
			variantsMap[v.AssetVersionID] = append(variantsMap[v.AssetVersionID], v)
		}
	}

	// 4. Build PathRegistry and resolve all paths up front.
	reg := NewPathRegistry()
	type itemInfo struct {
		row          assetVersionRow
		variant      *dbgen.Variant // nil for originals
		resolvedPath string
	}

	includeVersionSuffix := p.Config.Versions == "all"
	var items []itemInfo

	for _, r := range avRows {
		stem, ext := splitFilename(r.originalFilename)
		var versionSuffix string
		if includeVersionSuffix {
			versionSuffix = "__v" + itoa(int(r.versionNum))
		}
		folderName := ""
		if r.folderName != nil {
			folderName = *r.folderName
		}
		rp := reg.Resolve(p.Project.Name, folderName, stem, ext, versionSuffix, "")
		items = append(items, itemInfo{row: r, resolvedPath: rp})

		if includeVariants {
			for i := range variantsMap[r.versionID] {
				v := variantsMap[r.versionID][i]
				title := ""
				if v.Title != nil {
					title = *v.Title
				}
				if title == "" {
					title = v.Type
				}
				variantSlug := "__" + slugify(title)
				variantExt := ext
				vrp := reg.Resolve(p.Project.Name, folderName, stem, variantExt, versionSuffix, variantSlug)
				items = append(items, itemInfo{row: r, variant: &variantsMap[r.versionID][i], resolvedPath: vrp})
			}
		}
	}

	// 5. Create temp file for ZIP.
	f, err := os.CreateTemp("", "damask-export-*.zip")
	if err != nil {
		return BuildResult{}, fmt.Errorf("export: create temp file: %w", err)
	}
	defer os.Remove(f.Name())

	// 6. Open zip writer.
	zw := zip.NewWriter(f)

	// 7. Build in-memory manifest.
	manifest := Manifest{
		DamaskExportVersion: "1",
		ExportedAt:          time.Now().UTC(),
		ExportConfigID:      p.Config.ID,
		ExportRunID:         p.Run.ID,
		Project: ManifestProject{
			ID:          p.Project.ID,
			Name:        p.Project.Name,
			Description: p.Project.Description,
			Color:       p.Project.Color,
		},
		Assets: []ManifestAsset{},
	}

	// Track assets for manifest building.
	assetManifestMap := map[string]*ManifestAsset{}

	var result BuildResult

	// 8. Write each item into the ZIP.
	for i, item := range items {
		r := item.row

		var storageKey string
		var contentHash string
		var size int64
		var rkey string

		if item.variant != nil {
			storageKey = item.variant.StorageKey
			if item.variant.Size != nil {
				size = *item.variant.Size
			}
			rkey = remoteKey(r.assetID, r.versionNum, item.variant.ID)
			contentHash = ""
		} else {
			storageKey = r.storageKey
			contentHash = r.contentHash
			size = r.size
			rkey = remoteKey(r.assetID, r.versionNum, "")
		}

		// Incremental skip check.
		if prevHash, ok := remoteHashes[rkey]; ok && contentHash != "" && prevHash == contentHash {
			result.AssetsSkipped++
		} else {
			rc, storageErr := p.Storage.Get(storageKey)
			if storageErr != nil {
				slog.WarnContext(ctx, "export: storage get failed, skipping asset",
					"storage_key", storageKey,
					"error", storageErr)
				result.AssetsSkipped++
			} else {
				w, zipErr := zw.Create(item.resolvedPath)
				if zipErr != nil {
					_ = rc.Close()
					_ = zw.Close()
					_ = f.Close()
					return BuildResult{}, fmt.Errorf("export: zip create entry: %w", zipErr)
				}
				written, copyErr := io.Copy(w, rc)
				_ = rc.Close()
				if copyErr != nil {
					slog.WarnContext(ctx, "export: copy to zip failed", "error", copyErr)
					result.AssetsSkipped++
				} else {
					result.AssetsExported++
					result.BytesWritten += written
					if size == 0 {
						size = written
					}
				}
			}
		}

		// Build manifest entry.
		if item.variant == nil {
			ma, exists := assetManifestMap[r.assetID]
			if !exists {
				folderName := (*string)(nil)
				if r.folderName != nil {
					fn := *r.folderName
					folderName = &fn
				}
				newMA := ManifestAsset{
					ID:               r.assetID,
					OriginalFilename: r.originalFilename,
					Folder:           folderName,
					Tags:             tagsMap[r.assetID],
					CustomFields:     fieldsMap[r.assetID],
					Versions:         []ManifestVersion{},
				}
				manifest.Assets = append(manifest.Assets, newMA)
				assetManifestMap[r.assetID] = &manifest.Assets[len(manifest.Assets)-1]
				ma = assetManifestMap[r.assetID]
			}
			versionCreatedAt, _ := time.Parse("2006-01-02 15:04:05", r.versionCreatedAt)
			ma.Versions = append(ma.Versions, ManifestVersion{
				VersionNum:  r.versionNum,
				IsCurrent:   r.isCurrent,
				ContentHash: r.contentHash,
				Size:        size,
				MimeType:    r.mimeType,
				Comment:     r.comment,
				CreatedAt:   versionCreatedAt,
				Path:        item.resolvedPath,
				Variants:    []ManifestVariant{},
			})
		} else if item.variant != nil {
			if ma, ok := assetManifestMap[r.assetID]; ok {
				for idx := range ma.Versions {
					if ma.Versions[idx].VersionNum == r.versionNum {
						title := ""
						if item.variant.Title != nil {
							title = *item.variant.Title
						}
						ma.Versions[idx].Variants = append(ma.Versions[idx].Variants, ManifestVariant{
							ID:    item.variant.ID,
							Type:  item.variant.Type,
							Title: title,
							Path:  item.resolvedPath,
							Size:  size,
						})
						break
					}
				}
			}
		}

		// Progress callback every 10 items.
		if p.OnProgress != nil && (i+1)%10 == 0 {
			p.OnProgress(BuildProgress{
				AssetsExported: result.AssetsExported,
				AssetsSkipped:  result.AssetsSkipped,
				BytesWritten:   result.BytesWritten,
			})
		}
	}

	// 9. Write manifest.json into ZIP.
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return BuildResult{}, fmt.Errorf("export: marshal manifest: %w", err)
	}
	mw, err := zw.Create("manifest.json")
	if err != nil {
		return BuildResult{}, fmt.Errorf("export: zip manifest entry: %w", err)
	}
	if _, err := mw.Write(manifestJSON); err != nil {
		return BuildResult{}, fmt.Errorf("export: write manifest to zip: %w", err)
	}

	// 10. Close zip, rewind.
	if err := zw.Close(); err != nil {
		return BuildResult{}, fmt.Errorf("export: close zip: %w", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return BuildResult{}, fmt.Errorf("export: seek temp file: %w", err)
	}

	// 11. Get ZIP size.
	fi, err := f.Stat()
	if err != nil {
		return BuildResult{}, fmt.Errorf("export: stat temp file: %w", err)
	}
	zipSize := fi.Size()

	// 12. Write ZIP to destination.
	zipRemotePath := slugify(p.Project.Name) + "__export.zip"
	if err := p.Dest.Write(ctx, zipRemotePath, f, zipSize, ""); err != nil {
		return BuildResult{}, fmt.Errorf("export: write zip to destination: %w", err)
	}

	// 13. Write sidecar manifest.
	if err := p.Dest.WriteManifest(ctx, sidecarPath, manifestJSON); err != nil {
		return BuildResult{}, fmt.Errorf("export: write manifest to destination: %w", err)
	}

	result.AssetsTotal = result.AssetsExported + result.AssetsSkipped
	result.ManifestJSON = manifestJSON
	return result, nil
}

// splitFilename splits a filename into stem and extension.
// "hero-shot.jpg" → ("hero-shot", ".jpg")
// "archive.tar.gz" → ("archive.tar", ".gz").
func splitFilename(name string) (stem, ext string) {
	ext = filepath.Ext(name)
	stem = strings.TrimSuffix(name, ext)
	return stem, ext
}

// queryVariantsForVersionIDs fetches ready variants for a set of version IDs.
// Uses raw SQL with IN clause since sqlc doesn't support slices properly.
func queryVariantsForVersionIDs(
	ctx context.Context,
	db *sql.DB,
	versionIDs []string,
	workspaceID string,
) ([]dbgen.Variant, error) {
	if len(versionIDs) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(versionIDs))
	placeholders = placeholders[:len(placeholders)-1]
	query := fmt.Sprintf(
		`SELECT id, workspace_id, asset_version_id, type, storage_key, transform_params, size, status,
		        thumbnail_key, thumbnail_content_type, title, is_shared, created_at
		 FROM variants
		 WHERE asset_version_id IN (%s)
		   AND workspace_id = ?
		   AND status = 'ready'
		 ORDER BY asset_version_id, type, title`,
		placeholders,
	)
	args := make([]any, 0, len(versionIDs)+1)
	for _, id := range versionIDs {
		args = append(args, id)
	}
	args = append(args, workspaceID)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []dbgen.Variant
	for rows.Next() {
		var v dbgen.Variant
		if err := rows.Scan(
			&v.ID,
			&v.WorkspaceID,
			&v.AssetVersionID,
			&v.Type,
			&v.StorageKey,
			&v.TransformParams,
			&v.Size,
			&v.Status,
			&v.ThumbnailKey,
			&v.ThumbnailContentType,
			&v.Title,
			&v.IsShared,
			&v.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
