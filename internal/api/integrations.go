package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"
	oauthpkg "damask/server/internal/oauth"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// connectionResponse is the safe public shape — tokens are never included.
type connectionResponse struct {
	ID            string   `json:"id"`
	Provider      string   `json:"provider"`
	ProviderEmail string   `json:"provider_email,omitempty"`
	Scopes        []string `json:"scopes"`
	ConnectedAt   string   `json:"connected_at"`
}

func toConnectionResponse(c dbgen.OauthConnection) connectionResponse {
	var scopes []string
	_ = json.Unmarshal([]byte(c.Scopes), &scopes)
	email := ""
	if c.ProviderEmail != nil {
		email = *c.ProviderEmail
	}
	return connectionResponse{
		ID:            c.ID,
		Provider:      c.Provider,
		ProviderEmail: email,
		Scopes:        scopes,
		ConnectedAt:   c.CreatedAt,
	}
}

// GET /integrations/connections
func (s *Server) handleListConnections(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	conns, err := s.db.ListOAuthConnectionsByWorkspace(c.Context(), claims.WorkspaceID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list connections")
	}
	out := make([]connectionResponse, len(conns))
	for i, conn := range conns {
		out[i] = toConnectionResponse(conn)
	}
	return c.JSON(out)
}

// DELETE /integrations/connections/:id
func (s *Server) handleDeleteConnection(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")
	_, err := s.db.GetOAuthConnectionByID(c.Context(), dbgen.GetOAuthConnectionByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "connection not found")
	}
	if err := s.db.DeleteOAuthConnection(c.Context(), dbgen.DeleteOAuthConnectionParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not delete connection")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// googleDriveOAuth2Config builds the oauth2.Config for Drive import scopes.
func (s *Server) googleDriveOAuth2Config() oauth2.Config {
	return oauth2.Config{
		ClientID:     s.cfg.Google.ClientID,
		ClientSecret: s.cfg.Google.ClientSecret,
		RedirectURL:  s.cfg.BaseURL.String() + "/integrations/callback/google",
		Endpoint:     google.Endpoint,
		Scopes: []string{
			"openid", "email", "profile",
			"https://www.googleapis.com/auth/drive.readonly",
		},
	}
}

// canvaImportOAuth2Config builds the oauth2.Config for Canva import scopes.
func (s *Server) canvaImportOAuth2Config() oauth2.Config {
	return oauth2.Config{
		ClientID:     s.cfg.Canva.ClientID,
		ClientSecret: s.cfg.Canva.ClientSecret,
		RedirectURL:  s.cfg.BaseURL.String() + "/integrations/callback/canva",
		Endpoint: oauth2.Endpoint{
			AuthURL:  canvaAuthURL,
			TokenURL: canvaTokenURL,
		},
		Scopes: []string{"profile:read", "design:content:read"},
	}
}

// GET /integrations/connect/google
func (s *Server) handleConnectGoogle(c fiber.Ctx) error {
	if s.cfg.Google.ClientID == "" {
		return errRes(c, fiber.StatusServiceUnavailable, "Google not configured")
	}
	claims := auth.GetClaims(c)
	nonce, err := generateRandom(16)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not generate nonce")
	}
	state, err := signState(oauthState{
		WorkspaceID: claims.WorkspaceID,
		UserID:      claims.UserID,
		Nonce:       nonce,
		RedirectTo:  "/library/settings/integrations?connected=google",
	}, s.cfg.AppSecret)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not generate state")
	}

	cfg := s.googleDriveOAuth2Config()
	url := cfg.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	)
	return c.Redirect().To(url)
}

// GET /integrations/callback/google
func (s *Server) handleCallbackGoogle(c fiber.Ctx) error {
	rawState := c.Query("state")
	st, err := verifyState(rawState, s.cfg.AppSecret)
	if err != nil {
		return c.Redirect().To("/library/settings/integrations?error=invalid_state")
	}
	if c.Query("error") != "" {
		return c.Redirect().To("/library/settings/integrations?error=provider_error")
	}

	cfg := s.googleDriveOAuth2Config()
	token, err := cfg.Exchange(c.Context(), c.Query("code"))
	if err != nil {
		return c.Redirect().To("/library/settings/integrations?error=exchange_failed")
	}

	// Get Google user info to label the connection.
	sub, email, err := googleUserInfo(c.Context(), token.AccessToken)
	if err != nil {
		return c.Redirect().To("/library/settings/integrations?error=userinfo_failed")
	}

	if err := s.upsertConnection(c.Context(), st.WorkspaceID, st.UserID, "google", sub, email, token, cfg.Scopes); err != nil {
		return c.Redirect().To("/library/settings/integrations?error=save_failed")
	}

	redirect := st.RedirectTo
	if redirect == "" || !strings.HasPrefix(redirect, "/") {
		redirect = "/library/settings/integrations?connected=google"
	}
	return c.Redirect().To(redirect)
}

// GET /integrations/connect/canva
func (s *Server) handleConnectCanva(c fiber.Ctx) error {
	if s.cfg.Canva.ClientID == "" {
		return errRes(c, fiber.StatusServiceUnavailable, "Canva not configured")
	}
	claims := auth.GetClaims(c)
	nonce, err := generateRandom(16)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not generate nonce")
	}
	state, err := signState(oauthState{
		WorkspaceID: claims.WorkspaceID,
		UserID:      claims.UserID,
		Nonce:       nonce,
		RedirectTo:  "/library/settings/integrations?connected=canva",
	}, s.cfg.AppSecret)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not generate state")
	}

	verifier, err := generateRandom(64)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not generate pkce verifier")
	}
	s.setPKCECookie(c, verifier)

	cfg := s.canvaImportOAuth2Config()
	url := cfg.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", pkceChallenge(verifier)),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
	return c.Redirect().To(url)
}

// GET /integrations/callback/canva
func (s *Server) handleCallbackCanva(c fiber.Ctx) error {
	rawState := c.Query("state")
	st, err := verifyState(rawState, s.cfg.AppSecret)
	if err != nil {
		return c.Redirect().To("/library/settings/integrations?error=invalid_state")
	}
	if c.Query("error") != "" {
		return c.Redirect().To("/library/settings/integrations?error=provider_error")
	}

	verifier := c.Cookies(oidcPKCECookie)
	if verifier == "" {
		return c.Redirect().To("/library/settings/integrations?error=pkce_error")
	}
	c.Cookie(&fiber.Cookie{Name: oidcPKCECookie, Value: "", MaxAge: -1, Path: "/"})

	cfg := s.canvaImportOAuth2Config()
	token, err := cfg.Exchange(c.Context(), c.Query("code"),
		oauth2.SetAuthURLParam("code_verifier", verifier),
	)
	if err != nil {
		return c.Redirect().To("/library/settings/integrations?error=exchange_failed")
	}

	// Get Canva user info.
	canvaID, email, err := canvaUserInfo(c.Context(), token.AccessToken)
	if err != nil {
		return c.Redirect().To("/library/settings/integrations?error=userinfo_failed")
	}

	if err := s.upsertConnection(c.Context(), st.WorkspaceID, st.UserID, "canva", canvaID, email, token, cfg.Scopes); err != nil {
		return c.Redirect().To("/library/settings/integrations?error=save_failed")
	}

	redirect := st.RedirectTo
	if redirect == "" || !strings.HasPrefix(redirect, "/") {
		redirect = "/library/settings/integrations?connected=canva"
	}
	return c.Redirect().To(redirect)
}

// upsertConnection creates or updates an oauth_connections row.
func (s *Server) upsertConnection(ctx context.Context, workspaceID, userID, provider, providerUserID, providerEmail string, token *oauth2.Token, scopes []string) error {
	encAccess, err := oauthpkg.EncryptToken(s.cfg.AppSecret, token.AccessToken)
	if err != nil {
		return err
	}
	var encRefresh *string
	if token.RefreshToken != "" {
		enc, err := oauthpkg.EncryptToken(s.cfg.AppSecret, token.RefreshToken)
		if err != nil {
			return err
		}
		encRefresh = &enc
	}
	var expiresAt *string
	if !token.Expiry.IsZero() {
		s := token.Expiry.UTC().Format(time.RFC3339)
		expiresAt = &s
	}
	scopesJSON, _ := json.Marshal(scopes)

	// Try to update existing connection for same workspace+provider+account.
	existing, err := s.db.GetOAuthConnectionByProviderUserID(ctx, dbgen.GetOAuthConnectionByProviderUserIDParams{
		WorkspaceID:    workspaceID,
		Provider:       provider,
		ProviderUserID: &providerUserID,
	})
	if err == nil {
		_, err = s.db.UpdateOAuthConnectionTokens(ctx, dbgen.UpdateOAuthConnectionTokensParams{
			AccessToken:  encAccess,
			RefreshToken: encRefresh,
			ExpiresAt:    expiresAt,
			ID:           existing.ID,
		})
		return err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	email := nilIfEmpty(providerEmail)
	_, err = s.db.CreateOAuthConnection(ctx, dbgen.CreateOAuthConnectionParams{
		ID:             uuid.New().String(),
		WorkspaceID:    workspaceID,
		CreatedBy:      userID,
		Provider:       provider,
		ProviderUserID: &providerUserID,
		ProviderEmail:  email,
		Scopes:         string(scopesJSON),
		AccessToken:    encAccess,
		RefreshToken:   encRefresh,
		ExpiresAt:      expiresAt,
	})
	return err
}

// googleUserInfo calls the Google userinfo endpoint, returns (sub, email, error).
func googleUserInfo(ctx context.Context, accessToken string) (string, string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("userinfo: status %d", resp.StatusCode)
	}
	var info struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", "", err
	}
	return info.Sub, info.Email, nil
}

// canvaUserInfo calls the Canva /v1/users/me endpoint, returns (user_id, email, error).
func canvaUserInfo(ctx context.Context, accessToken string) (string, string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, canvaMeURL, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("canva userinfo: status %d", resp.StatusCode)
	}
	var me struct {
		Profile struct {
			UserID string `json:"user_id"`
			Email  string `json:"email"`
		} `json:"profile"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&me); err != nil {
		return "", "", err
	}
	return me.Profile.UserID, me.Profile.Email, nil
}
