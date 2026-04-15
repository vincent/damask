package services

import (
	"context"
	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"

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
		Role:        string(auth.Owner),
		InvitedBy:   nil,
	}); err != nil {
		return nil, err
	}

	return &w, nil
}
