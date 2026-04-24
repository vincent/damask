package reposqlc

import (
	"context"
	"database/sql"
	"errors"

	"damask/server/internal/apperr"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
)

type userRepo struct {
	q     *dbgen.Queries
	sqlDB *sql.DB
}

// NewUserRepo returns a repository.UserRepository backed by sqlc-generated queries.
func NewUserRepo(q *dbgen.Queries, sqlDB *sql.DB) repository.UserRepository {
	return &userRepo{q: q, sqlDB: sqlDB}
}

func (r *userRepo) GetByID(ctx context.Context, id string) (repository.User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.User{}, apperr.ErrNotFound
		}
		return repository.User{}, err
	}
	return toUser(row), nil
}

func (r *userRepo) GetByEmail(ctx context.Context, email string) (repository.User, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.User{}, apperr.ErrNotFound
		}
		return repository.User{}, err
	}
	return toUser(row), nil
}

func (r *userRepo) Create(ctx context.Context, u repository.User) (repository.User, error) {
	row, err := r.q.CreateUser(ctx, dbgen.CreateUserParams{
		ID:           u.ID,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Name:         u.Name,
	})
	if err != nil {
		return repository.User{}, err
	}
	return toUser(row), nil
}

func (r *userRepo) Update(ctx context.Context, u repository.User) (repository.User, error) {
	return r.GetByID(ctx, u.ID)
}

func (r *userRepo) GetByGoogleID(ctx context.Context, googleUserID string) (repository.User, error) {
	row, err := r.q.GetUserByGoogleID(ctx, &googleUserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.User{}, apperr.ErrNotFound
		}
		return repository.User{}, err
	}
	return toUser(row), nil
}

func (r *userRepo) GetByCanvaID(ctx context.Context, canvaUserID string) (repository.User, error) {
	row, err := r.q.GetUserByCanvaID(ctx, &canvaUserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.User{}, apperr.ErrNotFound
		}
		return repository.User{}, err
	}
	return toUser(row), nil
}

func (r *userRepo) GetByOIDC(ctx context.Context, issuer, sub string) (repository.User, error) {
	row, err := r.q.GetUserByOIDC(ctx, dbgen.GetUserByOIDCParams{
		OidcIssuer: &issuer,
		OidcSub:    &sub,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.User{}, apperr.ErrNotFound
		}
		return repository.User{}, err
	}
	return toUser(row), nil
}

func (r *userRepo) CreateWithGoogle(ctx context.Context, u repository.User) (repository.User, error) {
	row, err := r.q.CreateUserWithGoogle(ctx, dbgen.CreateUserWithGoogleParams{
		ID:           u.ID,
		Email:        u.Email,
		Name:         u.Name,
		GoogleUserID: u.GoogleUserID,
		AvatarUrl:    u.AvatarUrl,
		AuthMethods:  u.AuthMethods,
	})
	if err != nil {
		return repository.User{}, err
	}
	return toUser(row), nil
}

func (r *userRepo) CreateWithOIDC(ctx context.Context, u repository.User) (repository.User, error) {
	row, err := r.q.CreateUserWithOIDC(ctx, dbgen.CreateUserWithOIDCParams{
		ID:         u.ID,
		Email:      u.Email,
		Name:       u.Name,
		OidcIssuer: u.OidcIssuer,
		OidcSub:    u.OidcSub,
		AvatarUrl:  u.AvatarUrl,
		AuthMethods: u.AuthMethods,
	})
	if err != nil {
		return repository.User{}, err
	}
	return toUser(row), nil
}

func (r *userRepo) CreateWithCanva(ctx context.Context, u repository.User) (repository.User, error) {
	row, err := r.q.CreateUserWithCanva(ctx, dbgen.CreateUserWithCanvaParams{
		ID:          u.ID,
		Email:       u.Email,
		Name:        u.Name,
		CanvaUserID: u.CanvaUserID,
		AvatarUrl:   u.AvatarUrl,
		AuthMethods: u.AuthMethods,
	})
	if err != nil {
		return repository.User{}, err
	}
	return toUser(row), nil
}

func (r *userRepo) LinkGoogle(ctx context.Context, u repository.User) (repository.User, error) {
	row, err := r.q.LinkGoogle(ctx, dbgen.LinkGoogleParams{
		ID:           u.ID,
		GoogleUserID: u.GoogleUserID,
		AvatarUrl:    u.AvatarUrl,
		AuthMethods:  u.AuthMethods,
	})
	if err != nil {
		return repository.User{}, err
	}
	return toUser(row), nil
}

func (r *userRepo) LinkOIDC(ctx context.Context, u repository.User) (repository.User, error) {
	row, err := r.q.LinkOIDC(ctx, dbgen.LinkOIDCParams{
		ID:          u.ID,
		OidcIssuer:  u.OidcIssuer,
		OidcSub:     u.OidcSub,
		AvatarUrl:   u.AvatarUrl,
		AuthMethods: u.AuthMethods,
	})
	if err != nil {
		return repository.User{}, err
	}
	return toUser(row), nil
}

func (r *userRepo) LinkCanva(ctx context.Context, u repository.User) (repository.User, error) {
	row, err := r.q.LinkCanva(ctx, dbgen.LinkCanvaParams{
		ID:          u.ID,
		CanvaUserID: u.CanvaUserID,
		AvatarUrl:   u.AvatarUrl,
		AuthMethods: u.AuthMethods,
	})
	if err != nil {
		return repository.User{}, err
	}
	return toUser(row), nil
}

func (r *userRepo) UnlinkGoogle(ctx context.Context, u repository.User) (repository.User, error) {
	row, err := r.q.UnlinkGoogle(ctx, dbgen.UnlinkGoogleParams{
		ID:          u.ID,
		AuthMethods: u.AuthMethods,
	})
	if err != nil {
		return repository.User{}, err
	}
	return toUser(row), nil
}

func (r *userRepo) UnlinkOIDC(ctx context.Context, u repository.User) (repository.User, error) {
	row, err := r.q.UnlinkOIDC(ctx, dbgen.UnlinkOIDCParams{
		ID:          u.ID,
		AuthMethods: u.AuthMethods,
	})
	if err != nil {
		return repository.User{}, err
	}
	return toUser(row), nil
}

func (r *userRepo) UnlinkCanva(ctx context.Context, u repository.User) (repository.User, error) {
	row, err := r.q.UnlinkCanva(ctx, dbgen.UnlinkCanvaParams{
		ID:          u.ID,
		AuthMethods: u.AuthMethods,
	})
	if err != nil {
		return repository.User{}, err
	}
	return toUser(row), nil
}

func (r *userRepo) ListWorkspaceIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.q.ListWorkspacesByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(rows))
	for i, w := range rows {
		ids[i] = w.ID
	}
	return ids, nil
}

func (r *userRepo) RunInTx(ctx context.Context, fn func(repository.UserRepository) error) error {
	tx, err := r.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(&userRepo{q: r.q.WithTx(tx), sqlDB: r.sqlDB}); err != nil {
		return err
	}
	return tx.Commit()
}

func toUser(u dbgen.User) repository.User {
	return repository.User{
		ID:           u.ID,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Name:         u.Name,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
		OidcSub:      u.OidcSub,
		OidcIssuer:   u.OidcIssuer,
		GoogleUserID: u.GoogleUserID,
		CanvaUserID:  u.CanvaUserID,
		AvatarUrl:    u.AvatarUrl,
		AuthMethods:  u.AuthMethods,
	}
}
