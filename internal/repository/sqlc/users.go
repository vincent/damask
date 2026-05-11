package reposqlc

import (
	"context"
	"database/sql"
	"errors"

	"damask/server/internal/apperr"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
)

type userDB interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type userRepo struct {
	q     *dbgen.Queries
	sqlDB *sql.DB
	db    userDB
}

// NewUserRepo returns a repository.UserRepository backed by sqlc-generated queries.
func NewUserRepo(q *dbgen.Queries, sqlDB *sql.DB) repository.UserRepository {
	return &userRepo{q: q, sqlDB: sqlDB, db: sqlDB}
}

func (r *userRepo) GetByID(ctx context.Context, id string) (repository.User, error) {
	return r.getOne(ctx, `SELECT id, email, password_hash, name, created_at, updated_at, oidc_sub, oidc_issuer, canva_user_id, google_user_id, avatar_url, auth_methods, avatar_storage_key, pending_email, display_name, deleted_at FROM users WHERE id = ? AND deleted_at IS NULL LIMIT 1`, id)
}

func (r *userRepo) GetByEmail(ctx context.Context, email string) (repository.User, error) {
	return r.getOne(ctx, `SELECT id, email, password_hash, name, created_at, updated_at, oidc_sub, oidc_issuer, canva_user_id, google_user_id, avatar_url, auth_methods, avatar_storage_key, pending_email, display_name, deleted_at FROM users WHERE email = ? AND deleted_at IS NULL LIMIT 1`, email)
}

func (r *userRepo) Create(ctx context.Context, u repository.User) (repository.User, error) {
	_, err := r.q.CreateUser(ctx, dbgen.CreateUserParams{
		ID:           u.ID,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Name:         u.Name,
	})
	if err != nil {
		return repository.User{}, err
	}
	return r.GetByID(ctx, u.ID)
}

func (r *userRepo) Update(ctx context.Context, u repository.User) (repository.User, error) {
	return r.GetByID(ctx, u.ID)
}

func (r *userRepo) UpdateProfile(ctx context.Context, id, displayName string) (repository.User, error) {
	if _, err := r.db.ExecContext(ctx, `UPDATE users SET display_name = ?, updated_at = datetime('now') WHERE id = ? AND deleted_at IS NULL`, displayName, id); err != nil {
		return repository.User{}, err
	}
	return r.GetByID(ctx, id)
}

func (r *userRepo) UpdateAvatarKey(ctx context.Context, id, storageKey string) (repository.User, error) {
	if _, err := r.db.ExecContext(ctx, `UPDATE users SET avatar_storage_key = ?, updated_at = datetime('now') WHERE id = ? AND deleted_at IS NULL`, storageKey, id); err != nil {
		return repository.User{}, err
	}
	return r.GetByID(ctx, id)
}

func (r *userRepo) ClearAvatarKey(ctx context.Context, id string) (repository.User, error) {
	if _, err := r.db.ExecContext(ctx, `UPDATE users SET avatar_storage_key = NULL, updated_at = datetime('now') WHERE id = ? AND deleted_at IS NULL`, id); err != nil {
		return repository.User{}, err
	}
	return r.GetByID(ctx, id)
}

func (r *userRepo) SetPassword(ctx context.Context, id, passwordHash string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET password_hash = ?, updated_at = datetime('now') WHERE id = ? AND deleted_at IS NULL`, passwordHash, id)
	return err
}

func (r *userRepo) SetAuthMethods(ctx context.Context, id, authMethods string) (repository.User, error) {
	if _, err := r.db.ExecContext(ctx, `UPDATE users SET auth_methods = ?, updated_at = datetime('now') WHERE id = ? AND deleted_at IS NULL`, authMethods, id); err != nil {
		return repository.User{}, err
	}
	return r.GetByID(ctx, id)
}

func (r *userRepo) SetPendingEmail(ctx context.Context, id, email string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET pending_email = ?, updated_at = datetime('now') WHERE id = ? AND deleted_at IS NULL`, email, id)
	return err
}

func (r *userRepo) ClearPendingEmail(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET pending_email = NULL, updated_at = datetime('now') WHERE id = ?`, id)
	return err
}

func (r *userRepo) ConfirmEmailChange(ctx context.Context, id, pendingEmail string) (repository.User, error) {
	if _, err := r.db.ExecContext(ctx, `UPDATE users SET email = pending_email, pending_email = NULL, updated_at = datetime('now') WHERE id = ? AND pending_email = ? AND deleted_at IS NULL`, id, pendingEmail); err != nil {
		return repository.User{}, err
	}
	return r.GetByID(ctx, id)
}

func (r *userRepo) SoftDelete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET deleted_at = datetime('now'), updated_at = datetime('now') WHERE id = ? AND deleted_at IS NULL`, id)
	return err
}

func (r *userRepo) AnonymizeDeletedUser(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET email = 'deleted_' || id || '@deleted.invalid', display_name = 'Deleted user', password_hash = '', avatar_storage_key = NULL, avatar_url = NULL, pending_email = NULL, auth_methods = '[]', updated_at = datetime('now') WHERE id = ?`, id)
	return err
}

func (r *userRepo) HardDelete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	return err
}

func (r *userRepo) GetByGoogleID(ctx context.Context, googleUserID string) (repository.User, error) {
	return r.getOne(ctx, `SELECT id, email, password_hash, name, created_at, updated_at, oidc_sub, oidc_issuer, canva_user_id, google_user_id, avatar_url, auth_methods, avatar_storage_key, pending_email, display_name, deleted_at FROM users WHERE google_user_id = ? AND deleted_at IS NULL LIMIT 1`, googleUserID)
}

func (r *userRepo) GetByCanvaID(ctx context.Context, canvaUserID string) (repository.User, error) {
	return r.getOne(ctx, `SELECT id, email, password_hash, name, created_at, updated_at, oidc_sub, oidc_issuer, canva_user_id, google_user_id, avatar_url, auth_methods, avatar_storage_key, pending_email, display_name, deleted_at FROM users WHERE canva_user_id = ? AND deleted_at IS NULL LIMIT 1`, canvaUserID)
}

func (r *userRepo) GetByOIDC(ctx context.Context, issuer, sub string) (repository.User, error) {
	return r.getOne(ctx, `SELECT id, email, password_hash, name, created_at, updated_at, oidc_sub, oidc_issuer, canva_user_id, google_user_id, avatar_url, auth_methods, avatar_storage_key, pending_email, display_name, deleted_at FROM users WHERE oidc_issuer = ? AND oidc_sub = ? AND deleted_at IS NULL LIMIT 1`, issuer, sub)
}

func (r *userRepo) CreateWithGoogle(ctx context.Context, u repository.User) (repository.User, error) {
	_, err := r.q.CreateUserWithGoogle(ctx, dbgen.CreateUserWithGoogleParams{
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
	return r.GetByID(ctx, u.ID)
}

func (r *userRepo) CreateWithOIDC(ctx context.Context, u repository.User) (repository.User, error) {
	_, err := r.q.CreateUserWithOIDC(ctx, dbgen.CreateUserWithOIDCParams{
		ID:          u.ID,
		Email:       u.Email,
		Name:        u.Name,
		OidcIssuer:  u.OidcIssuer,
		OidcSub:     u.OidcSub,
		AvatarUrl:   u.AvatarUrl,
		AuthMethods: u.AuthMethods,
	})
	if err != nil {
		return repository.User{}, err
	}
	return r.GetByID(ctx, u.ID)
}

func (r *userRepo) CreateWithCanva(ctx context.Context, u repository.User) (repository.User, error) {
	_, err := r.q.CreateUserWithCanva(ctx, dbgen.CreateUserWithCanvaParams{
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
	return r.GetByID(ctx, u.ID)
}

func (r *userRepo) LinkGoogle(ctx context.Context, u repository.User) (repository.User, error) {
	_, err := r.q.LinkGoogle(ctx, dbgen.LinkGoogleParams{
		ID:           u.ID,
		GoogleUserID: u.GoogleUserID,
		AvatarUrl:    u.AvatarUrl,
		AuthMethods:  u.AuthMethods,
	})
	if err != nil {
		return repository.User{}, err
	}
	return r.GetByID(ctx, u.ID)
}

func (r *userRepo) LinkOIDC(ctx context.Context, u repository.User) (repository.User, error) {
	_, err := r.q.LinkOIDC(ctx, dbgen.LinkOIDCParams{
		ID:          u.ID,
		OidcIssuer:  u.OidcIssuer,
		OidcSub:     u.OidcSub,
		AvatarUrl:   u.AvatarUrl,
		AuthMethods: u.AuthMethods,
	})
	if err != nil {
		return repository.User{}, err
	}
	return r.GetByID(ctx, u.ID)
}

func (r *userRepo) LinkCanva(ctx context.Context, u repository.User) (repository.User, error) {
	_, err := r.q.LinkCanva(ctx, dbgen.LinkCanvaParams{
		ID:          u.ID,
		CanvaUserID: u.CanvaUserID,
		AvatarUrl:   u.AvatarUrl,
		AuthMethods: u.AuthMethods,
	})
	if err != nil {
		return repository.User{}, err
	}
	return r.GetByID(ctx, u.ID)
}

func (r *userRepo) UnlinkGoogle(ctx context.Context, u repository.User) (repository.User, error) {
	_, err := r.q.UnlinkGoogle(ctx, dbgen.UnlinkGoogleParams{
		ID:          u.ID,
		AuthMethods: u.AuthMethods,
	})
	if err != nil {
		return repository.User{}, err
	}
	return r.GetByID(ctx, u.ID)
}

func (r *userRepo) UnlinkOIDC(ctx context.Context, u repository.User) (repository.User, error) {
	_, err := r.q.UnlinkOIDC(ctx, dbgen.UnlinkOIDCParams{
		ID:          u.ID,
		AuthMethods: u.AuthMethods,
	})
	if err != nil {
		return repository.User{}, err
	}
	return r.GetByID(ctx, u.ID)
}

func (r *userRepo) UnlinkCanva(ctx context.Context, u repository.User) (repository.User, error) {
	_, err := r.q.UnlinkCanva(ctx, dbgen.UnlinkCanvaParams{
		ID:          u.ID,
		AuthMethods: u.AuthMethods,
	})
	if err != nil {
		return repository.User{}, err
	}
	return r.GetByID(ctx, u.ID)
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
	defer tx.Rollback() //nolint:errcheck
	if err := fn(&userRepo{q: r.q.WithTx(tx), sqlDB: r.sqlDB, db: tx}); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *userRepo) getOne(ctx context.Context, query string, args ...any) (repository.User, error) {
	row := r.db.QueryRowContext(ctx, query, args...)
	var u repository.User
	err := row.Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.Name,
		&u.CreatedAt,
		&u.UpdatedAt,
		&u.OidcSub,
		&u.OidcIssuer,
		&u.CanvaUserID,
		&u.GoogleUserID,
		&u.AvatarUrl,
		&u.AuthMethods,
		&u.AvatarStorageKey,
		&u.PendingEmail,
		&u.DisplayName,
		&u.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.User{}, apperr.ErrNotFound
		}
		return repository.User{}, err
	}
	return u, nil
}
