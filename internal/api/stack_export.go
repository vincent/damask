package api

import (
	"archive/zip"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"strings"

	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"

	"github.com/gofiber/fiber/v3"
)

// @Summary Export stack as ZIP
// @Description Streams a ZIP archive containing the original files for the given asset IDs. All assets must belong to the authenticated workspace. Files are named after their original filenames; duplicates are suffixed with <code>_N</code>. If any file is missing from storage it is skipped and listed in a <code>_missing_files.txt</code> manifest inside the ZIP. The response is streamed — do not buffer the entire body before processing.
// @Tags Stack
// @Accept json
// @Produce application/zip
// @Security BearerAuth
// @Param body body stackExportRequest true "Asset IDs and optional filename"
// @Success 200 {file} binary "ZIP archive"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "One or more assets not in workspace"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/stack/export [post]
// POST /api/v1/stack/export
func (s *Server) handleStackExport(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &stackExportRequest{})
	if !ok {
		return nil
	}

	filename := sanitiseFilename(body.Filename)
	if filename == "" {
		filename = "stack-export"
	}

	// Verify all IDs belong to this workspace in one query before fetching details.
	if ok, err := s.allAssetsInWorkspace(c.RequestCtx(), claims.WorkspaceID, body.AssetIDs); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not verify assets")
	} else if !ok {
		return errRes(c, fiber.StatusForbidden, "one or more assets not found in workspace")
	}

	type entry struct {
		name       string
		storageKey string
	}
	var entries []entry
	var missingNames []string
	usedNames := map[string]int{}

	for _, id := range body.AssetIDs {
		asset, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
			ID:          id,
			WorkspaceID: claims.WorkspaceID,
		})
		if err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not load asset")
		}

		version, err := s.db.GetCurrentVersion(c.RequestCtx(), asset.ID)
		if err != nil {
			missingNames = append(missingNames, asset.OriginalFilename)
			continue
		}

		base := asset.OriginalFilename
		usedNames[base]++
		name := base
		if usedNames[base] > 1 {
			ext := ""
			stem := base
			if dot := strings.LastIndex(base, "."); dot >= 0 {
				stem = base[:dot]
				ext = base[dot:]
			}
			name = fmt.Sprintf("%s_%d%s", stem, usedNames[base], ext)
		}

		entries = append(entries, entry{name: name, storageKey: version.StorageKey})
	}

	c.Set("Content-Type", "application/zip")
	c.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename + ".zip"}))

	// missingNames captured by value via closure over a local copy.
	preMissing := append([]string(nil), missingNames...)

	pr, pw := io.Pipe()
	go func() {
		zw := zip.NewWriter(pw)
		// goroutine-local missing list; starts with pre-missing entries.
		missing := preMissing

		for _, e := range entries {
			rc, err := s.storage.Get(e.storageKey)
			if err != nil {
				missing = append(missing, e.name)
				continue
			}
			fw, err := zw.Create(e.name)
			if err != nil {
				_ = rc.Close()
				missing = append(missing, e.name)
				continue
			}
			if _, err := io.Copy(fw, rc); err != nil {
				slog.Warn("zip copy error", "name", e.name, "err", err)
			}
			_ = rc.Close()
		}

		if len(missing) > 0 {
			fw, err := zw.Create("_missing_files.txt")
			if err == nil {
				for _, n := range missing {
					_, _ = fmt.Fprintln(fw, n)
				}
			}
		}

		if err := zw.Close(); err != nil {
			_ = pw.CloseWithError(err)
		} else {
			_ = pw.Close()
		}
	}()

	return c.SendStream(pr)
}

// sanitiseFilename strips path separators and ASCII control characters from a
// filename so it is safe to use in a Content-Disposition header.
func sanitiseFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '/' || r == '\\' || r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}
