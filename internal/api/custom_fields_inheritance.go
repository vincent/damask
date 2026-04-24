package api

import (
	"context"
	"log/slog"

	"damask/server/internal/fileproc"
)

// newInheritProjectFieldsFunc returns a fileproc.FieldInheritanceFunc that uses
// s.fields (FieldService) instead of *dbgen.Queries.
func (s *Server) newInheritProjectFieldsFunc() fileproc.FieldInheritanceFunc {
	return func(ctx context.Context, workspaceID, assetID, projectID, userID string) {
		if err := s.fields.InheritProjectFields(ctx, workspaceID, assetID, projectID, userID); err != nil {
			slog.Error("field inheritance", "workspace_id", workspaceID, "asset_id", assetID,
				"project_id", projectID, "error", err)
		}
	}
}
