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
	// No generic UpdateUser query exists in sqlc today; targeted link/unlink queries
	// handle OIDC/OAuth updates. This method is a placeholder for when auth migration
	// reaches the workspace service.
	return r.GetByID(ctx, u.ID)
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
		AvatarUrl:    u.AvatarUrl,
		AuthMethods:  u.AuthMethods,
	}
}
