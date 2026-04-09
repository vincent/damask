package api

import (
	"context"
	"log"

	dbgen "damask/server/internal/db/gen"

	"github.com/google/uuid"
)

// inheritProjectFields copies project field values to a newly created asset
// for all field definitions where inherit_from_project = 1.
func inheritProjectFields(ctx context.Context, db *dbgen.Queries, workspaceID, assetID, projectID, userID string) {
	defs, err := db.ListInheritableAssetFieldDefinitions(ctx, workspaceID)
	if err != nil {
		log.Printf("field inheritance: list defs: %v", err)
		return
	}
	if len(defs) == 0 {
		return
	}

	for _, def := range defs {
		pv, err := db.GetProjectFieldValue(ctx, dbgen.GetProjectFieldValueParams{
			ProjectID: projectID,
			FieldID:   def.ID,
		})
		if err != nil {
			continue // no value set on project for this field
		}

		if _, err := db.UpsertAssetFieldValue(ctx, dbgen.UpsertAssetFieldValueParams{
			ID:           uuid.NewString(),
			AssetID:      assetID,
			FieldID:      def.ID,
			ValueText:    pv.ValueText,
			ValueNumber:  pv.ValueNumber,
			ValueDate:    pv.ValueDate,
			ValueBoolean: pv.ValueBoolean,
			CreatedBy:    userID,
		}); err != nil {
			log.Printf("field inheritance: upsert asset %s field %s: %v", assetID, def.ID, err)
		}
	}
}
