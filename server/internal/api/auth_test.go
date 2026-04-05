package api_test

import (
	"damask/server/internal/api"
	th "damask/server/internal/tests_helpers"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegister_Success(t *testing.T) {
	env := th.SetupTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/auth/register",
		th.JsonStr(`{"name":"Alice","email":"alice@example.com","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var body api.AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.User.Email != "alice@example.com" {
		t.Errorf("email = %q, want %q", body.User.Email, "alice@example.com")
	}
	if body.Token == "" {
		t.Error("expected non-empty token in response body")
	}

	cookie := th.FindCookie(resp, "auth_token")
	if cookie == nil {
		t.Fatal("auth_token cookie not set")
	}
	if !cookie.HttpOnly {
		t.Error("cookie should be HttpOnly")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	env := th.SetupTestApp(t)
	th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := httptest.NewRequest(http.MethodPost, "/auth/register",
		th.JsonStr(`{"name":"Alice2","email":"alice@example.com","password":"password456"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}

func TestRegister_InvalidBody(t *testing.T) {
	env := th.SetupTestApp(t)

	cases := []struct {
		name string
		body string
	}{
		{"missing name", `{"email":"a@b.com","password":"password123"}`},
		{"missing email", `{"name":"Alice","password":"password123"}`},
		{"missing password", `{"name":"Alice","email":"a@b.com"}`},
		{"short password", `{"name":"Alice","email":"a@b.com","password":"short"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := env.App.Test(req)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			if resp.StatusCode != http.StatusUnprocessableEntity {
				t.Fatalf("expected 422, got %d", resp.StatusCode)
			}
		})
	}
}

func TestLogin_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := httptest.NewRequest(http.MethodPost, "/auth/login",
		th.JsonStr(`{"email":"alice@example.com","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	cookie := th.FindCookie(resp, "auth_token")
	if cookie == nil {
		t.Fatal("auth_token cookie not set on login")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	env := th.SetupTestApp(t)
	th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := httptest.NewRequest(http.MethodPost, "/auth/login",
		th.JsonStr(`{"email":"alice@example.com","password":"wrongpassword"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	env := th.SetupTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/auth/login",
		th.JsonStr(`{"email":"nobody@example.com","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestLogout(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodPost, "/auth/logout", nil, result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	cookie := th.FindCookie(resp, "auth_token")
	if cookie == nil {
		t.Fatal("expected auth_token cookie in response")
	}
	if cookie.Value != "" {
		t.Errorf("expected empty cookie value, got %q", cookie.Value)
	}
}

func TestRefresh_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodPost, "/auth/refresh", nil, result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	newCookie := th.FindCookie(resp, "auth_token")
	if newCookie == nil {
		t.Fatal("expected new auth_token cookie after refresh")
	}
}

func TestRefresh_Unauthenticated(t *testing.T) {
	env := th.SetupTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
