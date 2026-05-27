package testhelpers

// Integration tests helpers.
// Prefer using testutil helpers if you dont need a real DB.

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"damask/server/internal/api"
	"damask/server/internal/auth"
	"damask/server/internal/config"
	dbpkg "damask/server/internal/db"
	"damask/server/internal/events"
	"damask/server/internal/imagerouter"
	"damask/server/internal/jobs"
	"damask/server/internal/mail"
	"damask/server/internal/media/ingest"
	"damask/server/internal/queue"
	repomemory "damask/server/internal/repository/memory"
	reposqlc "damask/server/internal/repository/sqlc"
	"damask/server/internal/service"
	"damask/server/internal/storage"
	"damask/server/internal/transform"
	"damask/server/internal/versioning"
	"damask/server/internal/workflow"

	"github.com/gofiber/fiber/v3"
)

func TestMain(m *testing.M) {
	api.BcryptCost = bcrypt.MinCost //nolint:reassign // tests only
	m.Run()
}

// TestEnv holds the shared dependencies for a single test's app instance.
type TestEnv struct {
	App        *fiber.App
	HTTPServer *api.Server
	JobServer  *jobs.JobServer
	Maker      *auth.Maker
	Database   *sql.DB
	Storage    storage.Storage
	Config     *config.Config
}

// TestOption configures a test environment.
type TestOption func(*config.Config)

// WithBodyLimit sets a custom BodyLimit on the test app's config.
func WithBodyLimit(n int) TestOption {
	return func(cfg *config.Config) { cfg.BodyLimit = n }
}

func WithImageRouterAPIKey(key string) TestOption {
	return func(cfg *config.Config) { cfg.ImageRouter.APIKey = key }
}

func WithImageRouterDefaults(model, bgRemoveModel string) TestOption {
	return func(cfg *config.Config) {
		if model != "" {
			cfg.ImageRouter.DefaultModel = model
		}
		if bgRemoveModel != "" {
			cfg.ImageRouter.DefaultBgRemoveModel = bgRemoveModel
		}
	}
}

func WithImageRouterRetryPaidOnFreeLimit(enabled bool) TestOption {
	return func(cfg *config.Config) { cfg.ImageRouter.RetryPaidOnFreeLimit = enabled }
}

func SetupTestApp(t *testing.T, opts ...TestOption) *TestEnv {
	t.Helper()
	u, _ := url.Parse("http://localhost")
	cfg := &config.Config{
		JWTSecret: "test-app-secret-for-tests!!",
		AppSecret: "test-app-secret-for-tests!!",
		AppEnv:    "development",
		BaseURL:   u,
		ImageRouter: config.ImageRouterConfig{
			DefaultModel:         "black-forest-labs/FLUX.1-fill-dev",
			DefaultBgRemoveModel: "bria/remove-background",
			RetryPaidOnFreeLimit: false,
		},
	}
	for _, o := range opts {
		o(cfg)
	}

	queries, sqlDB, err := dbpkg.Open(":memory:?_foreign_keys=ON")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		err := sqlDB.Close()
		if err != nil {
			t.Fatalf("close db: %v", err)
		}
	})

	eventsHub := events.NewEventHub()

	maker, err := auth.NewMaker("test-secret-key-must-be-32chars!!")
	if err != nil {
		t.Fatalf("auth maker: %v", err)
	}

	stor, err := storage.NewAferoMemoryStorage()
	if err != nil {
		t.Fatalf("storage: %v", err)
	}

	q := queue.New(queries, 1)

	noopMailer := mail.NewMailer(&mail.Config{})
	trf := transform.NewTransformer()
	tmb := transform.NewThumbnailer(trf)
	media := ingest.NewRegistry(trf)
	injestor := service.NewAssetInjestor(queries, sqlDB, stor, q, media)
	workspaceRepo := reposqlc.NewWorkspaceRepo(queries, sqlDB)
	resolveImageRouterKey := imagerouter.NewKeyResolver(workspaceRepo, cfg.AppSecret, cfg.ImageRouter.APIKey)
	storageSvc := service.NewStorageService(queries)
	h := api.NewHTTPServer(queries, sqlDB, maker, stor, eventsHub, q, noopMailer, trf, cfg, nil, storageSvc)
	workflowExec := workflow.NewExecutor(
		workflow.Deps{
			Workflows: reposqlc.NewWorkflowRepo(queries, sqlDB),
			Runs:      reposqlc.NewWorkflowRunRepo(queries, sqlDB),
			Queue:     q,
			Config:    cfg,
		},
	)
	exportSvc := service.NewExportServiceWithRepos(queries, sqlDB, stor, cfg.AppSecret, q, repomemory.NewExportConfigRepo(), repomemory.NewExportRunRepo())
	exifSvc := service.NewExifService(queries, stor)
	fieldSvc := service.NewFieldService(reposqlc.NewFieldRepo(queries, sqlDB))
	textTrackSvc := service.NewTextTrackService(queries, q, stor)
	j := jobs.NewJobServer(
		queries,
		sqlDB,
		stor,
		eventsHub,
		q,
		noopMailer,
		trf,
		tmb,
		cfg,
		injestor,
		resolveImageRouterKey,
		workflowExec,
		exportSvc,
		exifSvc,
		fieldSvc,
		textTrackSvc,
		storageSvc,
	)
	app := api.NewRouter(queries, sqlDB, maker, stor, eventsHub, q, noopMailer, trf, cfg, nil, nil, storageSvc)
	return &TestEnv{App: app, HTTPServer: h, JobServer: j, Maker: maker, Database: sqlDB, Storage: stor, Config: cfg}
}

// AuthResult holds the parsed outcome of a register or login response.
type AuthResult struct {
	Cookie      *http.Cookie
	Token       string
	UserID      string
	WorkspaceID string
}

// Register calls POST /auth/Register and returns the parsed result.
// Fails the test if the request fails or returns a non-201 status.
func Register(t *testing.T, env *TestEnv, name, email, password string) AuthResult {
	t.Helper()
	body := fmt.Sprintf(`{"name":%q,"email":%q,"password":%q}`, name, email, password)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("register request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", resp.StatusCode)
	}

	var parsed api.AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("decode register response: %v", err)
	}

	return AuthResult{
		Cookie:      FindCookie(resp, "auth_token"),
		Token:       parsed.Token,
		UserID:      parsed.User.ID,
		WorkspaceID: parsed.Workspace.ID,
	}
}

// FindCookie returns the named cookie from a response, or nil.
func FindCookie(resp *http.Response, name string) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// AuthRequest builds an HTTP request and attaches a cookie if provided.
func AuthRequest(method, path string, body io.Reader, cookie *http.Cookie) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), method, path, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return req
}

// BearerRequest builds an HTTP request with an Authorization: Bearer header.
func BearerRequest(method, path string, body io.Reader, token string) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), method, path, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

// MintEditorToken inserts a user + member row directly and returns a signed token.
// Use this to set up non-owner fixture users without going through the invite flow.
func MintEditorToken(t *testing.T, env *TestEnv, workspaceID string, role auth.Role) string {
	t.Helper()
	userID := string("test-editor-" + role + "-id")
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)

	_, err := env.Database.ExecContext(
		t.Context(),
		`INSERT INTO users (id, email, password_hash, name, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		userID, role+"@example.com", string(hash), "Test "+role, now,
	)
	if err != nil {
		t.Fatalf("insert editor user: %v", err)
	}

	_, err = env.Database.ExecContext(
		t.Context(),
		`INSERT INTO workspace_members (workspace_id, user_id, role, created_at)
		 VALUES (?, ?, ?, ?)`,
		workspaceID, userID, role, now,
	)
	if err != nil {
		t.Fatalf("insert editor member: %v", err)
	}

	token, err := env.Maker.CreateToken(userID, workspaceID, time.Hour)
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}
	return token
}

// JSONStr returns an io.Reader wrapping a JSON string literal.
func JSONStr(s string) io.Reader {
	return strings.NewReader(s)
}

// JSONBody marshals v to JSON and returns an io.Reader suitable for AuthRequest/BearerRequest.
func JSONBody(v any) io.Reader {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("JSONBody: marshal failed: %v", err))
	}
	return bytes.NewReader(b)
}

// BuildVersionUploadRequest creates a multipart upload request for POST /assets/:id/versions.
func BuildVersionUploadRequest(
	t *testing.T,
	assetID string,
	filename string,
	content []byte,
	comment string,
	cookie *http.Cookie,
) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if comment != "" {
		if err := w.WriteField("comment", comment); err != nil {
			t.Fatalf("write comment field: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		fmt.Sprintf("/api/v1/assets/%s/versions", assetID),
		&body,
	)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return req
}

// MakeJPEG creates a minimal valid JPEG in memory.
func MakeJPEG(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.RGBA{R: 100, G: 150, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, nil)
	return buf.Bytes()
}

// BuildUploadRequest creates a multipart/form-data request with a file field.
// Optional extra form fields can be passed as additional map arguments.
func BuildUploadRequest(
	t *testing.T,
	filename string,
	content []byte,
	cookie *http.Cookie,
	extraFields ...map[string]string,
) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	for _, fields := range extraFields {
		for k, v := range fields {
			if err := w.WriteField(k, v); err != nil {
				t.Fatalf("write form field %q: %v", k, err)
			}
		}
	}
	err = w.Close()
	if err != nil {
		t.Fatalf("close form: %v", err)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/assets", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return req
}

// UploadAsset uploads an asset via the API. AV-2.1 ensures the v1 version row
// is created automatically inside the upload handler, so no manual seeding is needed.
func UploadAsset(t *testing.T, env *TestEnv, cookie *http.Cookie) api.AssetResponse {
	t.Helper()
	req := BuildUploadRequest(t, "original.jpg", MakeJPEG(100, 100), cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("upload asset: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload asset: expected 201, got %d: %s", resp.StatusCode, b)
	}
	var asset api.AssetResponse
	if err := json.NewDecoder(resp.Body).Decode(&asset); err != nil {
		t.Fatalf("decode asset: %v", err)
	}
	return asset
}

// DrainJobs registers all job handlers and synchronously processes every pending job
// in the queue. Use this in tests to run thumbnail/variant jobs without starting the
// background worker goroutines.
func DrainJobs(t *testing.T, env *TestEnv) {
	t.Helper()
	env.JobServer.DrainForTest(context.Background())
}

// SeedVersionV1 inserts a v1 asset_versions row for assets created via the old upload
// path (before AV-2.1 integration). It also sets assets.current_version_id.
func SeedVersionV1(t *testing.T, env *TestEnv, asset api.AssetResponse) string {
	t.Helper()
	versionID := "ver_v1_" + asset.ID

	// Resolve the owner user ID from workspace membership.
	var createdBy string
	err := env.Database.QueryRowContext(
		t.Context(),
		`SELECT user_id FROM workspace_members WHERE workspace_id = ? ORDER BY created_at LIMIT 1`,
		asset.WorkspaceID,
	).Scan(&createdBy)
	if err != nil {
		t.Fatalf("resolve owner for seed: %v", err)
	}

	// Look up the real storage_key from the assets table so the file endpoint works.
	var storageKey string
	if err := env.Database.QueryRow(
		`SELECT storage_key FROM assets WHERE id = ?`, asset.ID,
	).Scan(&storageKey); err != nil {
		t.Fatalf("lookup storage key: %v", err)
	}

	// Compute the real SHA-256 of the stored file so dedup logic works correctly.
	contentHash := "seed-hash-" + asset.ID // fallback if storage read fails
	if rc, err := env.Storage.Get(storageKey); err == nil {
		if h, _, hErr := versioning.HashReader(rc); hErr == nil {
			contentHash = h
		}
		rc.Close() //nolint:gosec,errcheck // file read only for test data
	}

	_, err = env.Database.ExecContext(
		t.Context(), `
		INSERT OR IGNORE INTO asset_versions (
			id, asset_id, workspace_id, version_num, storage_key, content_hash,
			mime_type, size, width, height, created_by, created_at, is_current
		) VALUES (?, ?, ?, 1, ?, ?, ?, ?, ?, ?, ?, datetime('now'), 1)
	`,
		versionID, asset.ID, asset.WorkspaceID,
		storageKey,
		contentHash,
		asset.MimeType, asset.Size,
		asset.Width, asset.Height,
		createdBy,
	)
	if err != nil {
		t.Fatalf("seed v1 version: %v", err)
	}

	_, err = env.Database.ExecContext(
		t.Context(),
		`UPDATE assets SET current_version_id = ? WHERE id = ?`, versionID, asset.ID,
	)
	if err != nil {
		t.Fatalf("set current_version_id: %v", err)
	}
	return versionID
}

// SetupWithOwner creates a test app and registers one owner user.
// This is the universal preamble for most test functions.
func SetupWithOwner(t *testing.T, opts ...TestOption) (*TestEnv, AuthResult) {
	t.Helper()
	env := SetupTestApp(t, opts...)
	owner := Register(t, env, "Owner", "owner@test.com", "password123")
	return env, owner
}

// UploadTestAsset uploads an asset for the given auth cookie and returns the asset ID.
// This is a convenience wrapper around UploadAsset that extracts just the ID.
func (env *TestEnv) UploadTestAsset(t *testing.T, cookie *http.Cookie) string {
	t.Helper()
	asset := UploadAsset(t, env, cookie)
	return asset.ID
}

// CreateProject creates a project and returns the full response.
func CreateProject(t *testing.T, env *TestEnv, cookie *http.Cookie, name, color string) api.ProjectResponse {
	t.Helper()
	body := fmt.Sprintf(`{"name":%q,"color":%q}`, name, color)
	req := AuthRequest(http.MethodPost, "/api/v1/projects", JSONStr(body), cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("create project: expected 201, got %d: %s", resp.StatusCode, b)
	}
	var project api.ProjectResponse
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		t.Fatalf("decode project: %v", err)
	}
	return project
}
