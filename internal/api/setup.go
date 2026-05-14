package api

import (
	"context"
	"errors"

	"damask/server/internal/apperr"
	"damask/server/internal/service"

	"github.com/gofiber/fiber/v3"
)

// requireSetupMode is a middleware that blocks setup routes once the wizard is complete.
func (s *Server) requireSetupMode(c fiber.Ctx) error {
	status, err := s.setup.Status(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal error"})
	}
	if status.Configured && status.OwnerExists {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "setup_complete"})
	}
	return c.Next()
}

// handleSetupStatus returns whether config and owner exist.
func (s *Server) handleSetupStatus(c fiber.Ctx) error {
	status, err := s.setup.Status(c.Context())
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "internal error")
	}
	return c.JSON(status)
}

// handleValidateStorage dry-runs storage connectivity.
func (s *Server) handleValidateStorage(c fiber.Ctx) error {
	body, ok := decodeAndValidate(c, validateStorageRequest{})
	if !ok {
		return nil
	}
	reason, err := s.setup.ValidateStorage(c.Context(), body.toParams())
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "internal error")
	}
	if reason != "" {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"ok":     false,
			"reason": reason,
		})
	}
	return c.JSON(fiber.Map{"ok": true})
}

// handleSetupDeps returns all external dep statuses.
func (s *Server) handleSetupDeps(c fiber.Ctx) error {
	statuses, err := s.setup.CheckDeps(c.Context())
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "internal error")
	}
	return c.JSON(statuses)
}

// handleWriteConfig writes the provided config to damask.env.
func (s *Server) handleWriteConfig(c fiber.Ctx) error {
	body, ok := decodeAndValidate(c, writeConfigRequest{})
	if !ok {
		return nil
	}
	if err := s.setup.WriteConfig(c.Context(), body.toParams()); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "internal error")
	}
	return c.JSON(fiber.Map{"ok": true})
}

// handleCreateOwner creates the first workspace + owner account.
func (s *Server) handleCreateOwner(c fiber.Ctx) error {
	body, ok := decodeAndValidate(c, createOwnerRequest{})
	if !ok {
		return nil
	}
	if err := s.setup.CreateOwner(c.Context(), body.toParams()); err != nil {
		if errors.Is(err, apperr.ErrConflict) {
			return errRes(c, fiber.StatusConflict, err.Error())
		}
		return errRes(c, fiber.StatusInternalServerError, "internal error")
	}
	return c.JSON(fiber.Map{"ok": true})
}

// --- request types ---

type validateStorageRequest struct {
	Type           string `json:"type"`
	LocalPath      string `json:"localPath"`
	S3Bucket       string `json:"s3Bucket"`
	S3Region       string `json:"s3Region"`
	S3Endpoint     string `json:"s3Endpoint"`
	S3AccessKey    string `json:"s3AccessKey"`
	S3SecretKey    string `json:"s3SecretKey"`
	SFTPHost       string `json:"sftpHost"`
	SFTPPort       int    `json:"sftpPort"`
	SFTPUser       string `json:"sftpUser"`
	SFTPKeyPath    string `json:"sftpKeyPath"`
	SFTPRemotePath string `json:"sftpRemotePath"`
}

func (r validateStorageRequest) Valid(_ context.Context) map[string]string {
	p := service.StorageParams{
		Type:      r.Type,
		LocalPath: r.LocalPath,
		S3Bucket:  r.S3Bucket,
		S3Region:  r.S3Region,
		SFTPHost:  r.SFTPHost,
		SFTPPort:  r.SFTPPort,
		SFTPUser:  r.SFTPUser,
	}
	if err := p.Validate(); err != nil {
		return map[string]string{"storage": err.Error()}
	}
	return nil
}

func (r validateStorageRequest) toParams() service.StorageParams {
	return service.StorageParams{
		Type:           r.Type,
		LocalPath:      r.LocalPath,
		S3Bucket:       r.S3Bucket,
		S3Region:       r.S3Region,
		S3Endpoint:     r.S3Endpoint,
		S3AccessKey:    r.S3AccessKey,
		S3SecretKey:    r.S3SecretKey,
		SFTPHost:       r.SFTPHost,
		SFTPPort:       r.SFTPPort,
		SFTPUser:       r.SFTPUser,
		SFTPKeyPath:    r.SFTPKeyPath,
		SFTPRemotePath: r.SFTPRemotePath,
	}
}

type writeConfigRequest struct {
	Port             int    `json:"port"`
	BaseURL          string `json:"baseURL"`
	Type             string `json:"type"`
	LocalPath        string `json:"localPath"`
	S3Bucket         string `json:"s3Bucket"`
	S3Region         string `json:"s3Region"`
	S3Endpoint       string `json:"s3Endpoint"`
	S3AccessKey      string `json:"s3AccessKey"`
	S3SecretKey      string `json:"s3SecretKey"`
	SFTPHost         string `json:"sftpHost"`
	SFTPPort         int    `json:"sftpPort"`
	SFTPUser         string `json:"sftpUser"`
	SFTPKeyPath      string `json:"sftpKeyPath"`
	SFTPRemotePath   string `json:"sftpRemotePath"`
	SMTPHost         string `json:"smtpHost"`
	SMTPPort         int    `json:"smtpPort"`
	SMTPUser         string `json:"smtpUser"`
	SMTPPass         string `json:"smtpPass"`
	OIDCIssuer       string `json:"oidcIssuer"`
	OIDCClientID     string `json:"oidcClientID"`
	OIDCClientSecret string `json:"oidcClientSecret"`
}

func (r writeConfigRequest) Valid(_ context.Context) map[string]string {
	p := service.EnvParams{
		Port: r.Port,
		StorageParams: service.StorageParams{
			Type:      r.Type,
			LocalPath: r.LocalPath,
			S3Bucket:  r.S3Bucket,
			S3Region:  r.S3Region,
			SFTPHost:  r.SFTPHost,
			SFTPPort:  r.SFTPPort,
			SFTPUser:  r.SFTPUser,
		},
	}
	if err := p.Validate(); err != nil {
		return map[string]string{"config": err.Error()}
	}
	return nil
}

func (r writeConfigRequest) toParams() service.EnvParams {
	return service.EnvParams{
		Port:    r.Port,
		BaseURL: r.BaseURL,
		StorageParams: service.StorageParams{
			Type:           r.Type,
			LocalPath:      r.LocalPath,
			S3Bucket:       r.S3Bucket,
			S3Region:       r.S3Region,
			S3Endpoint:     r.S3Endpoint,
			S3AccessKey:    r.S3AccessKey,
			S3SecretKey:    r.S3SecretKey,
			SFTPHost:       r.SFTPHost,
			SFTPPort:       r.SFTPPort,
			SFTPUser:       r.SFTPUser,
			SFTPKeyPath:    r.SFTPKeyPath,
			SFTPRemotePath: r.SFTPRemotePath,
		},
		SMTPHost:         r.SMTPHost,
		SMTPPort:         r.SMTPPort,
		SMTPUser:         r.SMTPUser,
		SMTPPass:         r.SMTPPass,
		OIDCIssuer:       r.OIDCIssuer,
		OIDCClientID:     r.OIDCClientID,
		OIDCClientSecret: r.OIDCClientSecret,
	}
}

type createOwnerRequest struct {
	WorkspaceName string `json:"workspaceName"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	Password      string `json:"password"`
}

func (r createOwnerRequest) Valid(_ context.Context) map[string]string {
	p := service.OwnerParams{
		WorkspaceName: r.WorkspaceName,
		Name:          r.Name,
		Email:         r.Email,
		Password:      r.Password,
	}
	if err := p.Validate(); err != nil {
		return map[string]string{"owner": err.Error()}
	}
	return nil
}

func (r createOwnerRequest) toParams() service.OwnerParams {
	return service.OwnerParams{
		WorkspaceName: r.WorkspaceName,
		Name:          r.Name,
		Email:         r.Email,
		Password:      r.Password,
	}
}
