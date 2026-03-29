package services

import (
	dbgen "badam/server/internal/db/gen"
	"context"
	"database/sql"

	"github.com/google/uuid"
)

func CreateWorkspaceForUser(ctx context.Context, qtx *dbgen.Queries, name, ownerID string) (workspace *dbgen.Workspace, err error) {

	workspaceID := uuid.New().String()

	w, err := qtx.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{
		ID:   workspaceID,
		Name: name,
	})
	if err != nil {
		return nil, err
	}

	if err := qtx.CreateMember(ctx, dbgen.CreateMemberParams{
		WorkspaceID: workspaceID,
		UserID:      ownerID,
		Role:        "owner",
		InvitedBy:   sql.NullString{},
	}); err != nil {
		return nil, err
	}

	return &w, nil
}
