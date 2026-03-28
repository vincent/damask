package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"creativo-dam/server/internal/auth"
	dbpkg "creativo-dam/server/internal/db"
	"creativo-dam/server/internal/storage"

	"github.com/gofiber/fiber/v2"
)

func TestMain(m *testing.M) {
	bcryptCost = bcrypt.MinCost
	m.Run()
}

// testEnv holds the shared dependencies for a single test's app instance.
type testEnv struct {
	app     *fiber.App
	maker   *auth.Maker
	sqlDB   *sql.DB
	storage *storage.LocalStorage
}

// setupTestApp opens a fresh temp-file SQLite DB, runs migrations, and
// returns a configured Fiber app. The DB is closed via t.Cleanup.
func setupTestApp(t *testing.T) *testEnv {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	queries, sqlDB, err := dbpkg.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	maker, err := auth.NewMaker("test-secret-key-must-be-32chars!!")
	if err != nil {
		t.Fatalf("auth maker: %v", err)
	}

	stor, err := storage.NewLocalStorage(filepath.Join(dir, "storage"))
	if err != nil {
		t.Fatalf("storage: %v", err)
	}

	app := New(queries, sqlDB, maker, stor, "development")
	return &testEnv{app: app, maker: maker, sqlDB: sqlDB, storage: stor}
}

// authResult holds the parsed outcome of a register or login response.
type authResult struct {
	Cookie      *http.Cookie
	Token       string
	UserID      string
	WorkspaceID string
}

// register calls POST /auth/register and returns the parsed result.
// Fails the test if the request fails or returns a non-201 status.
func register(t *testing.T, env *testEnv, name, email, password string) authResult {
	t.Helper()
	body := fmt.Sprintf(`{"name":%q,"email":%q,"password":%q}`, name, email, password)
	req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("register request: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", resp.StatusCode)
	}

	var parsed authResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("decode register response: %v", err)
	}

	return authResult{
		Cookie:      findCookie(resp, "auth_token"),
		Token:       parsed.Token,
		UserID:      parsed.User.ID,
		WorkspaceID: parsed.User.WorkspaceID,
	}
}

// findCookie returns the named cookie from a response, or nil.
func findCookie(resp *http.Response, name string) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// authRequest builds an HTTP request and attaches a cookie if provided.
func authRequest(method, path string, body io.Reader, cookie *http.Cookie) *http.Request {
	req := httptest.NewRequest(method, path, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return req
}

// bearerRequest builds an HTTP request with an Authorization: Bearer header.
func bearerRequest(method, path string, body io.Reader, token string) *http.Request {
	req := httptest.NewRequest(method, path, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

// mintEditorToken inserts a user + member row directly and returns a signed token.
// Use this to set up non-owner fixture users without going through the invite flow.
func mintEditorToken(t *testing.T, env *testEnv, workspaceID, role string) string {
	t.Helper()
	userID := "test-editor-" + role + "-id"
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)

	_, err := env.sqlDB.Exec(
		`INSERT INTO users (id, workspace_id, email, password_hash, name, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		userID, workspaceID, role+"@example.com", string(hash), "Test "+role, now,
	)
	if err != nil {
		t.Fatalf("insert editor user: %v", err)
	}

	_, err = env.sqlDB.Exec(
		`INSERT INTO workspace_members (workspace_id, user_id, role, created_at)
		 VALUES (?, ?, ?, ?)`,
		workspaceID, userID, role, now,
	)
	if err != nil {
		t.Fatalf("insert editor member: %v", err)
	}

	token, err := env.maker.CreateToken(userID, workspaceID, time.Hour)
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}
	return token
}

// jsonStr returns an io.Reader wrapping a JSON string literal.
func jsonStr(s string) io.Reader {
	return strings.NewReader(s)
}
