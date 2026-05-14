package service

import (
	"context"
	"database/sql"
	"fmt"
	"net/mail"
	"os"
	"strings"

	"damask/server/internal/apperr"
	"damask/server/internal/desktopconfig"
	"damask/server/internal/repository"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	gsftp "github.com/pkg/sftp"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/ssh"
)

// SetupService orchestrates first-run configuration and owner creation.
type SetupService struct {
	sqlDB      *sql.DB
	users      repository.UserRepository
	workspaces repository.WorkspaceRepository
	configDir  string
	// countAllUsers overrides the default SQL-based count; used in tests with memory repos.
	countAllUsers func(ctx context.Context) (int64, error)
}

// NewSetupService constructs a SetupService backed by a real SQLite database.
func NewSetupService(sqlDB *sql.DB, users repository.UserRepository, workspaces repository.WorkspaceRepository, configDir string) *SetupService {
	return &SetupService{
		sqlDB:      sqlDB,
		users:      users,
		workspaces: workspaces,
		configDir:  configDir,
	}
}

// NewSetupServiceWithCounter constructs a SetupService with a custom user-count function.
// Intended for tests that use in-memory repositories instead of a real SQLite DB.
func NewSetupServiceWithCounter(users repository.UserRepository, workspaces repository.WorkspaceRepository, configDir string, countFn func(ctx context.Context) (int64, error)) *SetupService {
	return &SetupService{
		users:         users,
		workspaces:    workspaces,
		configDir:     configDir,
		countAllUsers: countFn,
	}
}

// --- Status ---

// SetupStatus reports the current wizard state.
type SetupStatus struct {
	Configured  bool `json:"configured"`
	OwnerExists bool `json:"ownerExists"`
}

// Status returns whether damask.env exists and whether any user is in the DB.
func (s *SetupService) Status(ctx context.Context) (SetupStatus, error) {
	configured, err := desktopconfig.Exists(s.configDir)
	if err != nil {
		return SetupStatus{}, fmt.Errorf("setup: check config: %w", err)
	}
	ownerExists, err := s.anyUserExists(ctx)
	if err != nil {
		return SetupStatus{}, err
	}
	return SetupStatus{Configured: configured, OwnerExists: ownerExists}, nil
}

func (s *SetupService) anyUserExists(ctx context.Context) (bool, error) {
	if s.countAllUsers != nil {
		n, err := s.countAllUsers(ctx)
		return n > 0, err
	}
	var count int64
	row := s.sqlDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`)
	if err := row.Scan(&count); err != nil {
		return false, fmt.Errorf("setup: count users: %w", err)
	}
	return count > 0, nil
}

// --- Storage validation ---

// StorageParams holds storage backend configuration from the wizard.
type StorageParams struct {
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

// Validate checks required fields.
func (p StorageParams) Validate() error {
	switch p.Type {
	case "local":
		if p.LocalPath == "" {
			return fmt.Errorf("localPath is required: %w", apperr.ErrInvalidInput)
		}
	case "s3":
		if p.S3Bucket == "" {
			return fmt.Errorf("s3Bucket is required: %w", apperr.ErrInvalidInput)
		}
		if p.S3Region == "" {
			return fmt.Errorf("s3Region is required: %w", apperr.ErrInvalidInput)
		}
	case "sftp":
		if p.SFTPHost == "" {
			return fmt.Errorf("sftpHost is required: %w", apperr.ErrInvalidInput)
		}
		if p.SFTPPort < 1 || p.SFTPPort > 65535 {
			return fmt.Errorf("sftpPort must be between 1 and 65535: %w", apperr.ErrInvalidInput)
		}
		if p.SFTPUser == "" {
			return fmt.Errorf("sftpUser is required: %w", apperr.ErrInvalidInput)
		}
	case "":
		return fmt.Errorf("type is required: %w", apperr.ErrInvalidInput)
	default:
		return fmt.Errorf("unknown storage type %q: %w", p.Type, apperr.ErrInvalidInput)
	}
	return nil
}

// ValidateStorage dry-runs a connection to the configured storage backend.
// Returns a human-readable reason on failure, "" on success.
func (s *SetupService) ValidateStorage(ctx context.Context, p StorageParams) (string, error) {
	if err := p.Validate(); err != nil {
		return err.Error(), nil
	}
	switch p.Type {
	case "local":
		return s.validateLocalStorage(p.LocalPath)
	case "s3":
		return s.validateS3Storage(ctx, p)
	case "sftp":
		return s.validateSFTPStorage(p)
	}
	return "", nil
}

func (s *SetupService) validateLocalStorage(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Sprintf("path does not exist: %s", path), nil
		}
		return fmt.Sprintf("cannot access path: %v", err), nil
	}
	if !info.IsDir() {
		return fmt.Sprintf("%s is not a directory", path), nil
	}
	probe := path + "/.damask_probe"
	f, err := os.Create(probe)
	if err != nil {
		return fmt.Sprintf("path is not writable: %v", err), nil
	}
	f.Close()
	_ = os.Remove(probe)
	return "", nil
}

func (s *SetupService) validateS3Storage(ctx context.Context, p StorageParams) (string, error) {
	optFns := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(p.S3Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(p.S3AccessKey, p.S3SecretKey, "")),
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return fmt.Sprintf("S3 config error: %v", err), nil
	}
	client := awss3.NewFromConfig(cfg, func(o *awss3.Options) {
		if p.S3Endpoint != "" {
			o.BaseEndpoint = &p.S3Endpoint
		}
	})
	ctx10, cancel := context.WithTimeout(ctx, 10*1e9) // 10 second timeout
	defer cancel()
	if _, err := client.HeadBucket(ctx10, &awss3.HeadBucketInput{Bucket: &p.S3Bucket}); err != nil {
		return fmt.Sprintf("S3 bucket check failed: %v", err), nil
	}
	return "", nil
}

func (s *SetupService) validateSFTPStorage(p StorageParams) (string, error) {
	addr := fmt.Sprintf("%s:%d", p.SFTPHost, p.SFTPPort)
	config := &ssh.ClientConfig{
		User:            p.SFTPUser,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // wizard validation, not production
	}
	if p.SFTPKeyPath != "" {
		key, err := os.ReadFile(p.SFTPKeyPath)
		if err != nil {
			return fmt.Sprintf("cannot read SSH key: %v", err), nil
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return fmt.Sprintf("invalid SSH key: %v", err), nil
		}
		config.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	}
	sshConn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Sprintf("SSH connection failed: %v", err), nil
	}
	defer sshConn.Close()

	sftpClient, err := gsftp.NewClient(sshConn)
	if err != nil {
		return fmt.Sprintf("SFTP client failed: %v", err), nil
	}
	defer sftpClient.Close()

	if p.SFTPRemotePath != "" {
		if _, err := sftpClient.Stat(p.SFTPRemotePath); err != nil {
			return fmt.Sprintf("remote path not accessible: %v", err), nil
		}
	}
	return "", nil
}

// --- Deps ---

// CheckDeps delegates to desktopconfig.Check.
func (s *SetupService) CheckDeps(ctx context.Context) ([]desktopconfig.DepStatus, error) {
	return desktopconfig.Check(), nil
}

// --- Config write ---

// EnvParams holds all environment configuration from the wizard.
type EnvParams struct {
	Port    int    `json:"port"`
	BaseURL string `json:"baseURL"`
	StorageParams
	SMTPHost         string `json:"smtpHost"`
	SMTPPort         int    `json:"smtpPort"`
	SMTPUser         string `json:"smtpUser"`
	SMTPPass         string `json:"smtpPass"`
	OIDCIssuer       string `json:"oidcIssuer"`
	OIDCClientID     string `json:"oidcClientID"`
	OIDCClientSecret string `json:"oidcClientSecret"`
}

// Validate checks required fields.
func (p EnvParams) Validate() error {
	if p.Port < 1 || p.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535: %w", apperr.ErrInvalidInput)
	}
	return p.StorageParams.Validate()
}

const passwordPlaceholder = "***"

// WriteConfig converts EnvParams to key=value pairs and writes damask.env.
// Password values equal to the placeholder "***" are not written.
func (s *SetupService) WriteConfig(ctx context.Context, p EnvParams) error {
	m := map[string]string{
		"PORT": fmt.Sprintf("%d", p.Port),
	}
	if p.BaseURL != "" {
		m["BASE_URL"] = p.BaseURL
	}

	switch p.Type {
	case "local":
		m["STORAGE"] = "local"
		m["STORAGE_LOCAL_PATH"] = p.LocalPath
	case "s3":
		m["STORAGE"] = "s3"
		m["STORAGE_S3_BUCKET"] = p.S3Bucket
		m["STORAGE_S3_REGION"] = p.S3Region
		if p.S3Endpoint != "" {
			m["STORAGE_S3_ENDPOINT"] = p.S3Endpoint
		}
		if p.S3AccessKey != "" {
			m["STORAGE_S3_ACCESSKEY"] = p.S3AccessKey
		}
		if p.S3SecretKey != "" && p.S3SecretKey != passwordPlaceholder {
			m["STORAGE_S3_SECRETKEY"] = p.S3SecretKey
		}
	case "sftp":
		m["STORAGE"] = "sftp"
		m["STORAGE_SFTP_HOST"] = p.SFTPHost
		m["STORAGE_SFTP_PORT"] = fmt.Sprintf("%d", p.SFTPPort)
		m["STORAGE_SFTP_USER"] = p.SFTPUser
		if p.SFTPRemotePath != "" {
			m["STORAGE_SFTP_BASE_PATH"] = p.SFTPRemotePath
		}
		if p.SFTPKeyPath != "" {
			m["STORAGE_SFTP_PRIVATE_KEY"] = p.SFTPKeyPath
			m["STORAGE_SFTP_AUTH_METHOD"] = "key"
		}
	}

	if p.SMTPHost != "" {
		m["SMTP_HOST"] = p.SMTPHost
		if p.SMTPPort > 0 {
			m["SMTP_PORT"] = fmt.Sprintf("%d", p.SMTPPort)
		}
		if p.SMTPUser != "" {
			m["SMTP_USER"] = p.SMTPUser
		}
		if p.SMTPPass != "" && p.SMTPPass != passwordPlaceholder {
			m["SMTP_PASS"] = p.SMTPPass
		}
	}

	if p.OIDCIssuer != "" {
		m["OIDC_ISSUER_URL"] = p.OIDCIssuer
		if p.OIDCClientID != "" {
			m["OIDC_CLIENT_ID"] = p.OIDCClientID
		}
		if p.OIDCClientSecret != "" && p.OIDCClientSecret != passwordPlaceholder {
			m["OIDC_CLIENT_SECRET"] = p.OIDCClientSecret
		}
	}

	return desktopconfig.Write(s.configDir, m)
}

// --- Owner + workspace creation ---

// OwnerParams holds the inputs for the owner creation wizard step.
type OwnerParams struct {
	WorkspaceName string `json:"workspaceName"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	Password      string `json:"password"`
}

// Validate checks all required fields and password strength.
func (p OwnerParams) Validate() error {
	var problems []string
	if strings.TrimSpace(p.WorkspaceName) == "" {
		problems = append(problems, "workspaceName is required")
	}
	if strings.TrimSpace(p.Name) == "" {
		problems = append(problems, "name is required")
	}
	if strings.TrimSpace(p.Email) == "" {
		problems = append(problems, "email is required")
	} else if _, err := mail.ParseAddress(p.Email); err != nil {
		problems = append(problems, "email is invalid")
	}
	if len(p.Password) < 12 {
		problems = append(problems, "password must be at least 12 characters")
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s: %w", strings.Join(problems, "; "), apperr.ErrInvalidInput)
	}
	return nil
}

// CreateOwner creates the first workspace and owner user in a single transaction.
// Returns ErrConflict if an owner already exists.
func (s *SetupService) CreateOwner(ctx context.Context, p OwnerParams) error {
	exists, err := s.anyUserExists(ctx)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("owner already exists: %w", apperr.ErrConflict)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(p.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("setup: hash password: %w", err)
	}

	userID := uuid.New().String()
	workspaceID := uuid.New().String()

	return s.workspaces.RunRegistrationTx(ctx, func(ctx context.Context, txUsers repository.UserRepository, txWorkspaces repository.WorkspaceRepository) error {
		if _, err := txUsers.Create(ctx, repository.User{
			ID:           userID,
			Email:        strings.TrimSpace(p.Email),
			Name:         strings.TrimSpace(p.Name),
			PasswordHash: string(hash),
		}); err != nil {
			return fmt.Errorf("setup: create user: %w", err)
		}

		ws, err := txWorkspaces.Create(ctx, repository.Workspace{
			ID:   workspaceID,
			Name: strings.TrimSpace(p.WorkspaceName),
		})
		if err != nil {
			return fmt.Errorf("setup: create workspace: %w", err)
		}

		if err := txWorkspaces.CreateMember(ctx, repository.Member{
			WorkspaceID: ws.ID,
			UserID:      userID,
			Role:        "owner",
		}); err != nil {
			return fmt.Errorf("setup: create member: %w", err)
		}
		return nil
	})
}
