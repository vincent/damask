package service

import (
	"context"
	"database/sql"
	"log/slog"
)

// VerifySizeColumns logs warnings for any workspace that has asset_versions or
// variants rows with size = 0 or NULL, which would skew storage usage figures.
// Best-effort diagnostic — never blocks startup.
func VerifySizeColumns(ctx context.Context, db *sql.DB, log *slog.Logger) {
	rows, err := db.QueryContext(ctx, `
		SELECT workspace_id, COUNT(*) AS cnt
		FROM asset_versions
		WHERE size = 0 AND deleted_at IS NULL
		GROUP BY workspace_id
	`)
	if err != nil {
		log.WarnContext(ctx, "storage verify: could not query asset_versions", "error", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var wsID string
			var cnt int64
			if scanErr := rows.Scan(&wsID, &cnt); scanErr == nil {
				log.WarnContext(ctx, "storage verify: asset_versions with size=0",
					"workspace_id", wsID, "count", cnt)
			}
		}
		if iterErr := rows.Err(); iterErr != nil {
			log.WarnContext(ctx, "storage verify: asset_versions iteration error", "error", iterErr)
		}
	}

	rows2, err := db.QueryContext(ctx, `
		SELECT workspace_id, COUNT(*) AS cnt
		FROM variants
		WHERE size = 0 OR size IS NULL
		GROUP BY workspace_id
	`)
	if err != nil {
		log.WarnContext(ctx, "storage verify: could not query variants", "error", err)
		return
	}
	defer rows2.Close()
	for rows2.Next() {
		var wsID string
		var cnt int64
		if scanErr := rows2.Scan(&wsID, &cnt); scanErr == nil {
			log.WarnContext(ctx, "storage verify: variants with size=0 or NULL",
				"workspace_id", wsID, "count", cnt)
		}
	}
	if iterErr := rows2.Err(); iterErr != nil {
		log.WarnContext(ctx, "storage verify: variants iteration error", "error", iterErr)
	}
}
