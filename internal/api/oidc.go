package api

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"damask/server/internal/auth"
	"damask/server/internal/config"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/services"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

const (
	oidcStateCookie = "oidc_state"
	oidcPKCECookie  = "oidc_pkce"
	stateCookieTTL  = 10 * 60 // 10 minutes
)

// --- state / PKCE helpers ---

func generateRandom(n int) (string, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func pkceChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func (s *Server) setStateCookie(c fiber.Ctx, state string) {
	c.Cookie(&fiber.Cookie{
		Name:     oidcStateCookie,
		Value:    state,
		MaxAge:   stateCookieTTL,
		HTTPOnly: true,
		SameSite: "Lax",
		Path:     "/",
		Secure:   s.cfg.AppEnv != "development",
	})
}

func (s *Server) setPKCECookie(c fiber.Ctx, verifier string) {
	c.Cookie(&fiber.Cookie{
		Name:     oidcPKCECookie,
		Value:    verifier,
		MaxAge:   stateCookieTTL,
		HTTPOnly: true,
		SameSite: "Lax",
		Path:     "/",
		Secure:   s.cfg.AppEnv != "development",
	})
}

func (s *Server) clearOIDCCookies(c fiber.Ctx) {
	for _, name := range []string{oidcStateCookie, oidcPKCECookie} {
		c.Cookie(&fiber.Cookie{Name: name, Value: "", MaxAge: -1, Path: "/"})
	}
}

// initiateOAuth starts an OAuth flow: sets state+PKCE cookies, returns the redirect URL.
func (s *Server) initiateOAuth(c fiber.Ctx, oauth2Cfg oauth2.Config, extraOpts ...oauth2.AuthCodeOption) error {
	state, err := generateRandom(32)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not generate state")
	}
	verifier, err := generateRandom(64)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not generate pkce verifier")
	}
	s.setStateCookie(c, state)
	s.setPKCECookie(c, verifier)

	opts := append([]oauth2.AuthCodeOption{
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("code_challenge", pkceChallenge(verifier)),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	}, extraOpts...)

	return c.Redirect().To(oauth2Cfg.AuthCodeURL(state, opts...))
}

// validateOAuthCallback validates state + exchanges code. Returns (token, pkceVerifier, error).
func (s *Server) validateOAuthCallback(c fiber.Ctx, oauth2Cfg oauth2.Config) (*oauth2.Token, error) {
	stateParam := c.Query("state")
	stateCookie := c.Cookies(oidcStateCookie)
	if stateParam == "" || stateCookie == "" || !hmac.Equal([]byte(stateParam), []byte(stateCookie)) {
		s.clearOIDCCookies(c)
		return nil, fmt.Errorf("state mismatch")
	}
	if errParam := c.Query("error"); errParam != "" {
		s.clearOIDCCookies(c)
		return nil, fmt.Errorf("provider error: %s", errParam)
	}

	verifier := c.Cookies(oidcPKCECookie)
	s.clearOIDCCookies(c)

	token, err := oauth2Cfg.Exchange(c.Context(), c.Query("code"),
		oauth2.SetAuthURLParam("code_verifier", verifier),
	)
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %w", err)
	}
	return token, nil
}

// --- upsert helpers ---

// upsertOIDCUser finds or creates a user from OIDC claims. Returns (user, workspaceID, error).
func (s *Server) upsertOIDCUser(c fiber.Ctx, issuer, sub, email, name, avatarURL string, isGoogle bool) (dbgen.User, string, error) {
	ctx := c.Context()

	// 1. Look up by provider identity.
	var user dbgen.User
	var lookupErr error
	if isGoogle {
		user, lookupErr = s.db.GetUserByGoogleID(ctx, &sub)
	} else {
		user, lookupErr = s.db.GetUserByOIDC(ctx, dbgen.GetUserByOIDCParams{OidcIssuer: &issuer, OidcSub: &sub})
	}

	if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
		return dbgen.User{}, "", fmt.Errorf("db lookup: %w", lookupErr)
	}

	if lookupErr == nil {
		// Found — update avatar.
		methods := user.AuthMethods
		if isGoogle {
			u, err := s.db.LinkGoogle(ctx, dbgen.LinkGoogleParams{
				GoogleUserID: &sub,
				AvatarUrl:    nilIfEmpty(avatarURL),
				AuthMethods:  methods,
				ID:           user.ID,
			})
			if err != nil {
				return dbgen.User{}, "", err
			}
			user = u
		} else {
			u, err := s.db.LinkOIDC(ctx, dbgen.LinkOIDCParams{
				OidcIssuer:  &issuer,
				OidcSub:     &sub,
				AvatarUrl:   nilIfEmpty(avatarURL),
				AuthMethods: methods,
				ID:          user.ID,
			})
			if err != nil {
				return dbgen.User{}, "", err
			}
			user = u
		}
		wsID, err := s.firstWorkspaceID(ctx, user.ID)
		if err != nil {
			return dbgen.User{}, "", err
		}
		return user, wsID, nil
	}

	// 2. Look up by email — link to existing account.
	existing, err := s.db.GetUserByEmail(ctx, email)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return dbgen.User{}, "", err
	}
	if err == nil {
		methods := addAuthMethod(existing.AuthMethods, methodName(isGoogle))
		if isGoogle {
			existing, err = s.db.LinkGoogle(ctx, dbgen.LinkGoogleParams{
				GoogleUserID: &sub,
				AvatarUrl:    nilIfEmpty(avatarURL),
				AuthMethods:  methods,
				ID:           existing.ID,
			})
		} else {
			existing, err = s.db.LinkOIDC(ctx, dbgen.LinkOIDCParams{
				OidcIssuer:  &issuer,
				OidcSub:     &sub,
				AvatarUrl:   nilIfEmpty(avatarURL),
				AuthMethods: methods,
				ID:          existing.ID,
			})
		}
		if err != nil {
			return dbgen.User{}, "", err
		}
		wsID, err := s.firstWorkspaceID(ctx, existing.ID)
		if err != nil {
			return dbgen.User{}, "", err
		}
		return existing, wsID, nil
	}

	// 3. New user — create with workspace.
	tx, err := s.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return dbgen.User{}, "", err
	}
	defer tx.Rollback()
	qtx := s.db.WithTx(tx)

	userID := uuid.New().String()
	initMethodsB, _ := json.Marshal([]string{methodName(isGoogle)})
	initMethods := string(initMethodsB)
	if isGoogle {
		user, err = qtx.CreateUserWithGoogle(ctx, dbgen.CreateUserWithGoogleParams{
			ID: userID, Email: email, Name: name,
			GoogleUserID: &sub, AvatarUrl: nilIfEmpty(avatarURL), AuthMethods: initMethods,
		})
	} else {
		user, err = qtx.CreateUserWithOIDC(ctx, dbgen.CreateUserWithOIDCParams{
			ID: userID, Email: email, Name: name,
			OidcIssuer: &issuer, OidcSub: &sub, AvatarUrl: nilIfEmpty(avatarURL), AuthMethods: initMethods,
		})
	}
	if err != nil {
		return dbgen.User{}, "", err
	}
	ws, err := services.CreateWorkspaceForUser(ctx, qtx, name+"'s Workspace", userID)
	if err != nil {
		return dbgen.User{}, "", err
	}
	if err := tx.Commit(); err != nil {
		return dbgen.User{}, "", err
	}
	return user, ws.ID, nil
}

func methodName(isGoogle bool) string {
	if isGoogle {
		return "google"
	}
	return "oidc"
}

func addAuthMethod(current, method string) string {
	var methods []string
	_ = json.Unmarshal([]byte(current), &methods)
	for _, m := range methods {
		if m == method {
			return current
		}
	}
	methods = append(methods, method)
	b, _ := json.Marshal(methods)
	return string(b)
}

func removeAuthMethod(current, method string) string {
	var methods []string
	_ = json.Unmarshal([]byte(current), &methods)
	out := methods[:0]
	for _, m := range methods {
		if m != method {
			out = append(out, m)
		}
	}
	b, _ := json.Marshal(out)
	return string(b)
}

// --- OIDC handlers ---

// GET /auth/oidc/login
func (s *Server) handleOIDCLogin(c fiber.Ctx) error {
	rt := config.GetOIDCRuntime()
	if rt == nil {
		return errRes(c, fiber.StatusServiceUnavailable, "OIDC not configured")
	}
	return s.initiateOAuth(c, rt.OAuth2Config)
}

// GET /auth/oidc/callback
func (s *Server) handleOIDCCallback(c fiber.Ctx) error {
	rt := config.GetOIDCRuntime()
	if rt == nil {
		return c.Redirect().To("/login?error=oidc_error")
	}

	token, err := s.validateOAuthCallback(c, rt.OAuth2Config)
	if err != nil {
		return c.Redirect().To("/login?error=oidc_exchange")
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return c.Redirect().To("/login?error=oidc_exchange")
	}
	idToken, err := rt.Verifier.Verify(c.Context(), rawIDToken)
	if err != nil {
		return c.Redirect().To("/login?error=oidc_exchange")
	}

	var claims struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return c.Redirect().To("/login?error=oidc_exchange")
	}
	if !claims.EmailVerified {
		return c.Redirect().To("/login?error=email_not_verified")
	}

	user, workspaceID, err := s.upsertOIDCUser(c, idToken.Issuer, claims.Sub, claims.Email, claims.Name, claims.Picture, false)
	if err != nil {
		return c.Redirect().To("/login?error=oidc_exchange")
	}

	jwtToken, err := s.tokenMaker.CreateToken(user.ID, workspaceID, sessionDuration)
	if err != nil {
		return c.Redirect().To("/login?error=oidc_exchange")
	}
	s.setAuthCookie(c, jwtToken)
	return c.Redirect().To("/")
}

// --- Google handlers ---

// GET /auth/google/login
func (s *Server) handleGoogleLogin(c fiber.Ctx) error {
	rt := config.GetGoogleRuntime()
	if rt == nil {
		return errRes(c, fiber.StatusServiceUnavailable, "Google login not configured")
	}
	return s.initiateOAuth(c, rt.OAuth2Config,
		oauth2.SetAuthURLParam("prompt", "select_account"),
	)
}

// GET /auth/google/callback
func (s *Server) handleGoogleCallback(c fiber.Ctx) error {
	rt := config.GetGoogleRuntime()
	if rt == nil {
		return c.Redirect().To("/login?error=oidc_error")
	}

	token, err := s.validateOAuthCallback(c, rt.OAuth2Config)
	if err != nil {
		return c.Redirect().To("/login?error=oidc_exchange")
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return c.Redirect().To("/login?error=oidc_exchange")
	}
	idToken, err := rt.Verifier.Verify(c.Context(), rawIDToken)
	if err != nil {
		return c.Redirect().To("/login?error=oidc_exchange")
	}

	var claims struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return c.Redirect().To("/login?error=oidc_exchange")
	}
	if !claims.EmailVerified {
		return c.Redirect().To("/login?error=email_not_verified")
	}

	user, workspaceID, err := s.upsertOIDCUser(c, idToken.Issuer, claims.Sub, claims.Email, claims.Name, claims.Picture, true)
	if err != nil {
		return c.Redirect().To("/login?error=oidc_exchange")
	}

	jwtToken, err := s.tokenMaker.CreateToken(user.ID, workspaceID, sessionDuration)
	if err != nil {
		return c.Redirect().To("/login?error=oidc_exchange")
	}
	s.setAuthCookie(c, jwtToken)
	return c.Redirect().To("/")
}

// --- Canva handlers ---

const canvaAuthURL = "https://www.canva.com/api/oauth/authorize"
const canvaTokenURL = "https://api.canva.com/rest/v1/oauth/token"
const canvaMeURL = "https://api.canva.com/rest/v1/users/me"

func (s *Server) canvaOAuth2Config() oauth2.Config {
	return oauth2.Config{
		ClientID:     s.cfg.Canva.ClientID,
		ClientSecret: s.cfg.Canva.ClientSecret,
		RedirectURL:  s.cfg.BaseURL.String() + "/auth/canva/callback",
		Endpoint: oauth2.Endpoint{
			AuthURL:  canvaAuthURL,
			TokenURL: canvaTokenURL,
		},
		Scopes: []string{"profile:read"},
	}
}

// GET /auth/canva/login
func (s *Server) handleCanvaLogin(c fiber.Ctx) error {
	if s.cfg.Canva.ClientID == "" {
		return errRes(c, fiber.StatusServiceUnavailable, "Canva login not configured")
	}
	return s.initiateOAuth(c, s.canvaOAuth2Config())
}

// GET /auth/canva/callback
func (s *Server) handleCanvaCallback(c fiber.Ctx) error {
	if s.cfg.Canva.ClientID == "" {
		return c.Redirect().To("/login?error=oidc_error")
	}

	token, err := s.validateOAuthCallback(c, s.canvaOAuth2Config())
	if err != nil {
		return c.Redirect().To("/login?error=oidc_exchange&error_step=validate_token")
	}

	// Fetch user profile from Canva.
	meCtx, meCancel := context.WithTimeout(c.Context(), 10*time.Second)
	defer meCancel()
	req, _ := http.NewRequestWithContext(meCtx, http.MethodGet, canvaMeURL, nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return c.Redirect().To("/login?error=oidc_exchange&error_step=user_profile")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return c.Redirect().To("/login?error=oidc_exchange&error_step=user_profile_response")
	}

	var me struct {
		Profile struct {
			UserID      string `json:"user_id"`
			DisplayName string `json:"display_name"`
			Email       string `json:"email"`
		} `json:"profile"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&me); err != nil {
		return c.Redirect().To("/login?error=oidc_exchange&error_step=user_profile_decode")
	}

	user, workspaceID, err := s.upsertCanvaUser(c, me.Profile.UserID, me.Profile.Email, me.Profile.DisplayName, "")
	if err != nil {
		return c.Redirect().To("/login?error=oidc_exchange&error_step=upsert_user")
	}

	jwtToken, err := s.tokenMaker.CreateToken(user.ID, workspaceID, sessionDuration)
	if err != nil {
		return c.Redirect().To("/login?error=oidc_exchange&error_step=create_token")
	}
	s.setAuthCookie(c, jwtToken)
	return c.Redirect().To("/")
}

func (s *Server) upsertCanvaUser(c fiber.Ctx, canvaID, email, name, avatarURL string) (dbgen.User, string, error) {
	ctx := c.Context()

	user, err := s.db.GetUserByCanvaID(ctx, &canvaID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return dbgen.User{}, "", err
	}
	if err == nil {
		methods := addAuthMethod(user.AuthMethods, "canva")
		u, err := s.db.LinkCanva(ctx, dbgen.LinkCanvaParams{
			CanvaUserID: &canvaID,
			AvatarUrl:   nilIfEmpty(avatarURL),
			AuthMethods: methods,
			ID:          user.ID,
		})
		if err != nil {
			return dbgen.User{}, "", err
		}
		wsID, err := s.firstWorkspaceID(ctx, u.ID)
		if err != nil {
			return dbgen.User{}, "", err
		}
		return u, wsID, nil
	}

	if email != "" {
		existing, err := s.db.GetUserByEmail(ctx, email)
		if err == nil {
			methods := addAuthMethod(existing.AuthMethods, "canva")
			existing, err = s.db.LinkCanva(ctx, dbgen.LinkCanvaParams{
				CanvaUserID: &canvaID,
				AvatarUrl:   nilIfEmpty(avatarURL),
				AuthMethods: methods,
				ID:          existing.ID,
			})
			if err != nil {
				return dbgen.User{}, "", err
			}
			wsID, err := s.firstWorkspaceID(ctx, existing.ID)
			if err != nil {
				return dbgen.User{}, "", err
			}
			return existing, wsID, nil
		}
	}

	tx, err := s.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return dbgen.User{}, "", err
	}
	defer tx.Rollback()
	qtx := s.db.WithTx(tx)

	if name == "" {
		name = "Canva User"
	}
	syntheticEmail := "canva+" + canvaID + "@canva.local"
	if email != "" {
		syntheticEmail = email
	}
	userID := uuid.New().String()
	newUser, err := qtx.CreateUserWithCanva(ctx, dbgen.CreateUserWithCanvaParams{
		ID: userID, Email: syntheticEmail, Name: name,
		CanvaUserID: &canvaID, AvatarUrl: nilIfEmpty(avatarURL), AuthMethods: `["canva"]`,
	})
	if err != nil {
		return dbgen.User{}, "", err
	}
	ws, err := services.CreateWorkspaceForUser(ctx, qtx, name+"'s Workspace", userID)
	if err != nil {
		return dbgen.User{}, "", err
	}
	if err := tx.Commit(); err != nil {
		return dbgen.User{}, "", err
	}
	return newUser, ws.ID, nil
}

// --- me handler ---

type meResponse struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Email        string  `json:"email"`
	AvatarURL    *string `json:"avatar_url"`
	AuthMethods  string  `json:"auth_methods"`
	OIDCLinked   bool    `json:"oidc_linked"`
	GoogleLinked bool    `json:"google_linked"`
	CanvaLinked  bool    `json:"canva_linked"`
}

// GET /auth/me
func (s *Server) handleGetMe(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	user, err := s.db.GetUserByID(c.Context(), claims.UserID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load user")
	}
	return c.JSON(meResponse{
		ID:           user.ID,
		Name:         user.Name,
		Email:        user.Email,
		AvatarURL:    user.AvatarUrl,
		AuthMethods:  user.AuthMethods,
		OIDCLinked:   user.OidcSub != nil,
		GoogleLinked: user.GoogleUserID != nil,
		CanvaLinked:  user.CanvaUserID != nil,
	})
}

// --- unlink handlers ---

// DELETE /auth/oidc/link
func (s *Server) handleUnlinkOIDC(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	user, err := s.db.GetUserByID(c.Context(), claims.UserID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load user")
	}
	if hasOnlyMethod(user.AuthMethods, "oidc") {
		return errRes(c, fiber.StatusUnprocessableEntity, "set a password before removing your last sign-in method")
	}
	methods := removeAuthMethod(user.AuthMethods, "oidc")
	if _, err := s.db.UnlinkOIDC(c.Context(), dbgen.UnlinkOIDCParams{
		AuthMethods: methods, ID: user.ID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not unlink")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// DELETE /auth/google/link
func (s *Server) handleUnlinkGoogle(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	user, err := s.db.GetUserByID(c.Context(), claims.UserID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load user")
	}
	if hasOnlyMethod(user.AuthMethods, "google") {
		return errRes(c, fiber.StatusUnprocessableEntity, "set a password before removing your last sign-in method")
	}
	methods := removeAuthMethod(user.AuthMethods, "google")
	if _, err := s.db.UnlinkGoogle(c.Context(), dbgen.UnlinkGoogleParams{
		AuthMethods: methods, ID: user.ID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not unlink")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// DELETE /auth/canva/link
func (s *Server) handleUnlinkCanva(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	user, err := s.db.GetUserByID(c.Context(), claims.UserID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load user")
	}
	if hasOnlyMethod(user.AuthMethods, "canva") {
		return errRes(c, fiber.StatusUnprocessableEntity, "set a password before removing your last sign-in method")
	}
	methods := removeAuthMethod(user.AuthMethods, "canva")
	if _, err := s.db.UnlinkCanva(c.Context(), dbgen.UnlinkCanvaParams{
		AuthMethods: methods, ID: user.ID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not unlink")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// --- signed state helpers for /integrations OAuth ---

type oauthState struct {
	WorkspaceID string `json:"workspace_id"`
	UserID      string `json:"user_id"`
	Nonce       string `json:"nonce"`
	RedirectTo  string `json:"redirect_to,omitempty"`
}

func signState(payload oauthState, secret string) (string, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	enc := base64.RawURLEncoding.EncodeToString(b)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(enc))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return enc + "." + sig, nil
}

func verifyState(raw, secret string) (oauthState, error) {
	parts := strings.SplitN(raw, ".", 2)
	if len(parts) != 2 {
		return oauthState{}, fmt.Errorf("invalid state format")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0]))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return oauthState{}, fmt.Errorf("state signature invalid")
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return oauthState{}, err
	}
	var s oauthState
	if err := json.Unmarshal(b, &s); err != nil {
		return oauthState{}, err
	}
	return s, nil
}

// --- util ---

func hasOnlyMethod(authMethods, method string) bool {
	var methods []string
	_ = json.Unmarshal([]byte(authMethods), &methods)
	return len(methods) == 1 && methods[0] == method
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// firstWorkspaceID returns the first workspace ID for the given user.
func (s *Server) firstWorkspaceID(ctx context.Context, userID string) (string, error) {
	workspaces, err := s.db.ListWorkspacesByUserID(ctx, userID)
	if err != nil || len(workspaces) == 0 {
		return "", fmt.Errorf("no workspace found for user")
	}
	return workspaces[0].ID, nil
}
