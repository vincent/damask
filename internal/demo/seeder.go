//go:build demo

// Package demo implements the demo workspace seeder, wiper, and reset scheduler.
// All demo functionality is gated behind DEMO_MODE=true in config.
package demo

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"log/slog"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"damask/server/internal/audit"
	"damask/server/internal/config"
	"damask/server/internal/jobs"
	"damask/server/internal/queue"
	"damask/server/internal/storage"
	"damask/server/internal/transform"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// bcryptHashOfDemo is a pre-computed bcrypt hash of the password "demo"
// at cost 10. Never bcrypt at runtime during seed — it's slow.
const bcryptHashOfDemo = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

// Seeder creates and populates the demo workspace.
type Seeder struct {
	db          *sql.DB
	storage     storage.Storage
	trf         transform.Transformer
	tmb         transform.Thumbnailer
	jobQueue    queue.JobQueue // optional (may be nil); see New
	cfg         config.DemoConfig
	lastResetAt time.Time // set after each successful reset; zero on first boot
}

// New returns a Seeder ready to use. q is used to enqueue extract_media_tags
// jobs for video/audio assets during seeding (RA-6.2); pass nil if no queue
// is available — seeding still succeeds, assets simply won't have media tags
// until something else triggers extraction.
func New(
	db *sql.DB,
	stor storage.Storage,
	cfg config.DemoConfig,
	trf transform.Transformer,
	tmb transform.Thumbnailer,
	q queue.JobQueue,
) *Seeder {
	return &Seeder{db: db, storage: stor, cfg: cfg, trf: trf, tmb: tmb, jobQueue: q}
}

// ids holds the stable IDs created during seeding so later steps can reference them.
type ids struct {
	workspaceID string
	userID      string
	aliceID     string
	marcID      string

	// projects
	brandProjectID   string
	summerProjectID  string
	websiteProjectID string
	archiveProjectID string

	// folders (brand)
	logosFolder  string
	colorsFolder string

	// folders (summer)
	photoFolder  string
	videoFolder  string
	socialFolder string

	// folders (website)
	wireframesFolder string
	uiCompFolder     string
	exportsFolder    string

	// folders (archive)
	printReadyFolder string

	// field definitions (asset scope)
	fieldClient       string
	fieldStatus       string
	fieldUsageRights  string
	fieldPhotographer string
	fieldLicensed     string

	// field definitions (project scope)
	pfieldClient string
	pfieldBudget string
	pfieldPhase  string

	// key assets (for versioning and events)
	assetHomepageV2  string
	assetLogoPrimary string
	assetBeachHero   string
	assetStudioHero  string

	// share
	shareID string

	// all asset ids (for event seeding)
	allAssets []assetMeta

	// manifestByName indexes the loaded asset manifest by final filename,
	// used by seedTags/seedFieldValues to look up tags/fields per asset.
	manifestByName map[string]manifestEntry
}

type assetMeta struct {
	id        string
	name      string
	projectID string
}

// SeedIfEmpty seeds the demo workspace only if it has no projects yet.
// Used on startup to handle first boot and crash recovery.
func (s *Seeder) SeedIfEmpty(ctx context.Context) error {
	var workspaceID string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM workspaces WHERE is_demo = 1 LIMIT 1`).Scan(&workspaceID)
	if err != nil {
		return nil // workspace doesn't exist yet
	}

	var count int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE workspace_id = ?`, workspaceID).Scan(&count)
	if count > 0 {
		return nil // already seeded
	}

	return s.Seed(ctx)
}

// Seed creates the demo workspace content from scratch.
// Call this only after Wipe() or on first boot.
// The workspace and user rows must already exist (created by EnsureWorkspace).
func (s *Seeder) Seed(ctx context.Context) error {
	slog.InfoContext(ctx, "demo: seed started")
	start := time.Now()

	var d ids

	// Load the stable workspace and user IDs
	row := s.db.QueryRowContext(ctx, `SELECT id FROM workspaces WHERE is_demo = 1 LIMIT 1`)
	if err := row.Scan(&d.workspaceID); err != nil {
		return fmt.Errorf("demo: find demo workspace: %w", err)
	}
	row = s.db.QueryRowContext(ctx, `SELECT id FROM users WHERE email = ? LIMIT 1`, s.cfg.UserEmail)
	if err := row.Scan(&d.userID); err != nil {
		return fmt.Errorf("demo: find demo user: %w", err)
	}

	// Create ghost users for the activity log (they may already exist from a previous seed)
	base := strings.TrimPrefix(s.cfg.UserEmail, "demo") // e.g. "@example.com"
	aliceID, err := s.ensureGhostUser(ctx, "alice"+base, "Alice")
	if err != nil {
		return fmt.Errorf("demo: ghost user alice: %w", err)
	}
	d.aliceID = aliceID

	marcID, err := s.ensureGhostUser(ctx, "marc"+base, "Marc")
	if err != nil {
		return fmt.Errorf("demo: ghost user marc: %w", err)
	}
	d.marcID = marcID

	// Seed all content inside a transaction where possible
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("demo: begin tx: %w", err)
	}
	defer tx.Rollback()

	if err := s.seedFieldDefinitions(ctx, tx, &d); err != nil {
		return err
	}
	if err := s.seedProjects(ctx, tx, &d); err != nil {
		return err
	}
	if err := s.seedFolders(ctx, tx, &d); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("demo: commit structure tx: %w", err)
	}

	// Asset creation writes to storage outside the transaction
	if err := s.seedAssets(ctx, &d); err != nil {
		return err
	}

	// Second transaction for everything that references assets
	tx2, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("demo: begin tx2: %w", err)
	}
	defer tx2.Rollback()

	if err := s.seedTags(ctx, tx2, &d); err != nil {
		return err
	}
	if err := s.seedFieldValues(ctx, tx2, &d); err != nil {
		return err
	}
	if err := s.seedShare(ctx, tx2, &d); err != nil {
		return err
	}
	if err := s.seedIngressSource(ctx, tx2, &d); err != nil {
		return err
	}

	if err := tx2.Commit(); err != nil {
		return fmt.Errorf("demo: commit data tx: %w", err)
	}

	// Events are best-effort; written outside any transaction
	if err := s.seedEvents(ctx, &d); err != nil {
		slog.WarnContext(ctx, "demo: seed events (non-fatal)", "error", err)
	}

	slog.InfoContext(
		ctx,
		"demo: seed complete",
		"assets_created",
		len(d.allAssets),
		"duration_ms",
		time.Since(start).Milliseconds(),
	)
	return nil
}

// EnsureWorkspace creates the workspace and user rows if they don't exist.
// These rows are kept stable across resets (not wiped), so this is idempotent.
func (s *Seeder) EnsureWorkspace(ctx context.Context) error {
	// Check if already exists
	var existing string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM workspaces WHERE is_demo = 1 LIMIT 1`).Scan(&existing)
	if err == nil {
		return nil // already exists
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("demo: check workspace: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("demo: begin workspace tx: %w", err)
	}
	defer tx.Rollback()

	workspaceID := "demo_ws_" + uuid.NewString()
	userID := "demo_usr_" + uuid.NewString()
	ingestToken := uuid.NewString()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO workspaces (id, name, ingest_token, is_demo, created_at, updated_at)
		VALUES (?, ?, ?, 1, datetime('now'), datetime('now'))
	`, workspaceID, s.cfg.WorkspaceName, ingestToken)
	if err != nil {
		return fmt.Errorf("demo: create workspace: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES (?, ?, ?, 'Demo User', datetime('now'), datetime('now'))
	`, userID, s.cfg.UserEmail, bcryptHashOfDemo)
	if err != nil {
		return fmt.Errorf("demo: create user: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO workspace_members (workspace_id, user_id, role, created_at)
		VALUES (?, ?, 'editor', datetime('now'))
	`, workspaceID, userID)
	if err != nil {
		return fmt.Errorf("demo: create member: %w", err)
	}

	return tx.Commit()
}

// GetWorkspaceID returns the demo workspace ID, or ("", false) if not found.
func (s *Seeder) GetWorkspaceID(ctx context.Context) (string, bool) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM workspaces WHERE is_demo = 1 LIMIT 1`).Scan(&id)
	if err != nil {
		return "", false
	}
	return id, true
}

// GetDemoUser returns the (userID, workspaceID) for the demo session, or an error
// if the demo workspace does not exist (mid-reset).
func (s *Seeder) GetDemoUser(ctx context.Context) (userID, workspaceID string, err error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT u.id, w.id
		FROM workspaces w
		JOIN workspace_members wm ON wm.workspace_id = w.id
		JOIN users u ON u.id = wm.user_id
		WHERE w.is_demo = 1 AND u.email = ?
		LIMIT 1
	`, s.cfg.UserEmail)
	err = row.Scan(&userID, &workspaceID)
	return userID, workspaceID, err
}

// VerifyDemoPassword checks that the provided password matches the demo user.
// Returns false (not an error) if the password is wrong.
func VerifyDemoPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(bcryptHashOfDemo), []byte(password))
	return err == nil
}

// ResetInterval returns the configured reset interval.
func (s *Seeder) ResetInterval() time.Duration {
	return time.Duration(s.cfg.ResetIntervalHours) * time.Hour
}

// LastResetAt returns the time of the last successful reset (zero if none yet).
func (s *Seeder) LastResetAt() time.Time {
	return s.lastResetAt
}

// NextResetAt returns the estimated time of the next reset.
// Returns zero if the reset interval is not configured.
func (s *Seeder) NextResetAt() time.Time {
	if s.lastResetAt.IsZero() || s.cfg.ResetIntervalHours == 0 {
		return time.Time{}
	}
	return s.lastResetAt.Add(s.ResetInterval())
}

// GetUsage returns the current asset count and total storage bytes used by the demo workspace.
func (s *Seeder) GetUsage(ctx context.Context, workspaceID string) (assetCount, storageUsed int64, err error) {
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT a.id),
		       COALESCE(SUM(av.size), 0)
		FROM assets a
		LEFT JOIN asset_versions av ON av.id = a.current_version_id
		WHERE a.workspace_id = ?
	`, workspaceID).Scan(&assetCount, &storageUsed)
	return assetCount, storageUsed, err
}

// --- field definitions ---

func (s *Seeder) seedFieldDefinitions(ctx context.Context, tx *sql.Tx, d *ids) error {
	type fieldDef struct {
		id       string
		scope    string
		name     string
		key      string
		ftype    string
		options  *string
		position int
	}

	opts := func(o string) *string { return &o }

	assetFields := []fieldDef{
		{newID("fd"), "asset", "Client", "client", "select", opts(`["Sportswear Co","Internal","Archived"]`), 0},
		{newID("fd"), "asset", "Status", "status", "select", opts(`["Draft","In Review","Approved","Rejected"]`), 1},
		{newID("fd"), "asset", "Usage Rights Expiry", "usage_rights", "date", nil, 2},
		{newID("fd"), "asset", "Photographer", "photographer", "text", nil, 3},
		{newID("fd"), "asset", "Licensed", "licensed", "boolean", nil, 4},
	}
	projFields := []fieldDef{
		{newID("fd"), "project", "Client", "client", "text", nil, 0},
		{newID("fd"), "project", "Budget (€)", "budget", "number", nil, 1},
		{
			newID("fd"),
			"project",
			"Phase",
			"phase",
			"select",
			opts(`["Discovery","Production","Delivery","Archived"]`),
			2,
		},
	}

	insertFD := func(f fieldDef) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO field_definitions
			  (id, workspace_id, created_by, scope, name, key, field_type, options, required, position, inherit_from_project, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?, 0, datetime('now'), datetime('now'))
		`, f.id, d.workspaceID, d.userID, f.scope, f.name, f.key, f.ftype, f.options, f.position)
		return err
	}

	for i, f := range assetFields {
		if err := insertFD(f); err != nil {
			return fmt.Errorf("demo: asset field def %d: %w", i, err)
		}
		switch f.key {
		case "client":
			d.fieldClient = f.id
		case "status":
			d.fieldStatus = f.id
		case "usage_rights":
			d.fieldUsageRights = f.id
		case "photographer":
			d.fieldPhotographer = f.id
		case "licensed":
			d.fieldLicensed = f.id
		}
	}
	for i, f := range projFields {
		if err := insertFD(f); err != nil {
			return fmt.Errorf("demo: project field def %d: %w", i, err)
		}
		switch f.key {
		case "client":
			d.pfieldClient = f.id
		case "budget":
			d.pfieldBudget = f.id
		case "phase":
			d.pfieldPhase = f.id
		}
	}
	return nil
}

// --- projects ---

func (s *Seeder) seedProjects(ctx context.Context, tx *sql.Tx, d *ids) error {
	type proj struct {
		id    string
		name  string
		desc  string
		color string
	}
	projects := []proj{
		{newID("proj"), "Brand Identity", "Sportswear client brand system", "#6366f1"},
		{newID("proj"), "Summer Campaign", "Photography and video for the summer launch", "#f59e0b"},
		{newID("proj"), "Website Redesign", "Full redesign of the marketing site", "#10b981"},
		{newID("proj"), "Q2 Campaign Archive", "Archived Q2 assets", "#6b7280"},
	}

	for i, p := range projects {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO projects (id, workspace_id, name, description, color, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))
		`, p.id, d.workspaceID, p.name, p.desc, p.color)
		if err != nil {
			return fmt.Errorf("demo: project %d: %w", i, err)
		}
	}

	d.brandProjectID = projects[0].id
	d.summerProjectID = projects[1].id
	d.websiteProjectID = projects[2].id
	d.archiveProjectID = projects[3].id
	return nil
}

// --- folders ---

func (s *Seeder) seedFolders(ctx context.Context, tx *sql.Tx, d *ids) error {
	type folder struct {
		id        string
		projectID string
		name      string
	}
	folders := []folder{
		// brand
		{newID("fld"), d.brandProjectID, "Logos"},
		{newID("fld"), d.brandProjectID, "Colors & Typography"},
		// summer
		{newID("fld"), d.summerProjectID, "Photography"},
		{newID("fld"), d.summerProjectID, "Video"},
		{newID("fld"), d.summerProjectID, "Social"},
		// website
		{newID("fld"), d.websiteProjectID, "Wireframes"},
		{newID("fld"), d.websiteProjectID, "UI Components"},
		{newID("fld"), d.websiteProjectID, "Exports"},
		// archive
		{newID("fld"), d.archiveProjectID, "Print Ready"},
	}

	for i, f := range folders {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO folders (id, workspace_id, project_id, name, position, created_at)
			VALUES (?, ?, ?, ?, ?, datetime('now'))
		`, f.id, d.workspaceID, f.projectID, f.name, i)
		if err != nil {
			return fmt.Errorf("demo: folder %q: %w", f.name, err)
		}
	}

	d.logosFolder = folders[0].id
	d.colorsFolder = folders[1].id
	d.photoFolder = folders[2].id
	d.videoFolder = folders[3].id
	d.socialFolder = folders[4].id
	d.wireframesFolder = folders[5].id
	d.uiCompFolder = folders[6].id
	d.exportsFolder = folders[7].id
	d.printReadyFolder = folders[8].id
	return nil
}

// --- assets ---

// assetSpec pairs a manifest entry with the workspace-specific project/folder
// IDs it resolves to, and the total version count it needs (base + extras).
type assetSpec struct {
	entry        manifestEntry
	projectID    string
	folderID     string
	makeVersions int
}

func (s *Seeder) seedAssets(ctx context.Context, d *ids) error {
	rng := rand.New(rand.NewSource(42))

	specs, err := s.buildAssetSpecs(d)
	if err != nil {
		return err
	}

	for i := range specs {
		sp := &specs[i]
		assetID := newID("ast")
		ext := transform.MimeToExt(sp.entry.Mime)

		data, width, height, err := s.generateFile(sp, 0, rng)
		if err != nil {
			return fmt.Errorf("demo: generate %s: %w", sp.entry.Name, err)
		}

		storageKey := fmt.Sprintf("demo/%s/%s/%s%s", d.workspaceID, assetID, assetID, ext)
		if err := s.storage.Put(ctx, storageKey, bytes.NewReader(data)); err != nil {
			return fmt.Errorf("demo: store %s: %w", sp.entry.Name, err)
		}

		contentHash := md5hex(data)

		var widthPtr, heightPtr *int64
		if width > 0 {
			w64 := int64(width)
			widthPtr = &w64
		}
		if height > 0 {
			h64 := int64(height)
			heightPtr = &h64
		}

		// Backdate assets over 14 days for realism
		createdAt := backdateRand(rng, 14)

		_, err = s.db.ExecContext(ctx, `
			INSERT INTO assets
			  (id, workspace_id, project_id, folder_id, original_filename, storage_key,
			   mime_type, size, width, height, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, assetID, d.workspaceID, sp.projectID, nullStr(sp.folderID),
			sp.entry.Name, storageKey, sp.entry.Mime, len(data),
			widthPtr, heightPtr,
			createdAt.Format("2006-01-02T15:04:05Z"),
			createdAt.Format("2006-01-02T15:04:05Z"))
		if err != nil {
			return fmt.Errorf("demo: insert asset %s: %w", sp.entry.Name, err)
		}

		// Create initial version row
		versionID := newID("ver")
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO asset_versions
			  (id, asset_id, workspace_id, version_num, storage_key, content_hash,
			   mime_type, size, width, height, created_by, created_at, is_current)
			VALUES (?, ?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, 1)
		`, versionID, assetID, d.workspaceID,
			storageKey, contentHash,
			sp.entry.Mime, len(data), widthPtr, heightPtr,
			d.userID, createdAt.Format("2006-01-02T15:04:05Z"))
		if err != nil {
			return fmt.Errorf("demo: insert version for %s: %w", sp.entry.Name, err)
		}

		// Link current_version_id on the asset
		_, err = s.db.ExecContext(ctx, `UPDATE assets SET current_version_id = ? WHERE id = ?`, versionID, assetID)
		if err != nil {
			return fmt.Errorf("demo: link version for %s: %w", sp.entry.Name, err)
		}

		// Generate thumbnail synchronously
		thumbData, thumbExt, tErr := s.tmb.GenerateThumbnailData(ctx, s.storage, sp.entry.Mime, storageKey)
		if tErr == nil && thumbData != nil {
			thumbKey := fmt.Sprintf("demo/%s/%s/versions/%s/thumb%s", d.workspaceID, assetID, versionID, thumbExt)
			if putErr := s.storage.Put(ctx, thumbKey, bytes.NewReader(thumbData)); putErr == nil {
				s.db.ExecContext(ctx, `UPDATE asset_versions SET thumbnail_key = ? WHERE id = ?`, thumbKey, versionID)
				s.db.ExecContext(ctx, `UPDATE assets SET thumbnail_key = ? WHERE id = ?`, thumbKey, assetID)
			} else {
				slog.WarnContext(ctx, "demo: store thumbnail failed", "name", sp.entry.Name, "error", putErr)
			}
		} else {
			slog.WarnContext(ctx, "demo: thumbnail generation failed", "name", sp.entry.Name, "error", tErr)
		}

		// Enqueue media-tag extraction for video/audio assets (RA-6.2), same
		// as a real upload would. Best-effort: seeding still succeeds if no
		// queue was wired up, or if enqueueing fails.
		if s.jobQueue != nil &&
			(strings.HasPrefix(sp.entry.Mime, "video/") || strings.HasPrefix(sp.entry.Mime, "audio/")) {
			if err := jobs.EnqueueExtractMediaTagsJob(ctx, s.jobQueue, d.workspaceID, assetID); err != nil {
				slog.WarnContext(ctx, "demo: enqueue extract_media_tags failed", "name", sp.entry.Name, "error", err)
			}
		}

		d.allAssets = append(d.allAssets, assetMeta{id: assetID, name: sp.entry.Name, projectID: sp.projectID})

		// Capture key asset IDs by name
		switch sp.entry.Name {
		case "homepage-v2.png":
			d.assetHomepageV2 = assetID
		case "logo-primary-light.png":
			d.assetLogoPrimary = assetID
		case "hero-shot-beach.jpg":
			d.assetBeachHero = assetID
		case "hero-shot-studio.jpg":
			d.assetStudioHero = assetID
		}

		// Create additional versions where requested
		if sp.makeVersions > 1 {
			for v := 2; v <= sp.makeVersions; v++ {
				if err := s.addVersion(ctx, d, assetID, sp, v, rng); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (s *Seeder) addVersion(
	ctx context.Context,
	d *ids,
	assetID string,
	sp *assetSpec,
	versionNum int,
	rng *rand.Rand,
) error {
	data, width, height, err := s.generateFile(sp, versionNum-1, rng)
	if err != nil {
		return fmt.Errorf("demo: generate version %d of %s: %w", versionNum, sp.entry.Name, err)
	}

	storageKey := fmt.Sprintf("demo/%s/%s_v%d", d.workspaceID, assetID, versionNum)
	if err := s.storage.Put(ctx, storageKey, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("demo: store version %d of %s: %w", versionNum, sp.entry.Name, err)
	}

	contentHash := md5hex(data)
	versionID := newID("ver")

	comment := versionComment(sp.entry, versionNum)
	var commentPtr *string
	if comment != "" {
		commentPtr = &comment
	}

	var widthPtr, heightPtr *int64
	if width > 0 {
		w64 := int64(width)
		widthPtr = &w64
	}
	if height > 0 {
		h64 := int64(height)
		heightPtr = &h64
	}

	createdAt := backdateRand(rng, 10)

	// Mark previous current versions as non-current
	_, err = s.db.ExecContext(ctx, `UPDATE asset_versions SET is_current = 0 WHERE asset_id = ?`, assetID)
	if err != nil {
		return fmt.Errorf("demo: unset current versions: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO asset_versions
		  (id, asset_id, workspace_id, version_num, storage_key, content_hash,
		   mime_type, size, width, height, comment, created_by, created_at, is_current)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
	`, versionID, assetID, d.workspaceID,
		versionNum, storageKey, contentHash,
		sp.entry.Mime, len(data), widthPtr, heightPtr,
		commentPtr, d.userID, createdAt.Format("2006-01-02T15:04:05Z"))
	if err != nil {
		return fmt.Errorf("demo: insert version %d of %s: %w", versionNum, sp.entry.Name, err)
	}

	// Update asset's current_version_id
	_, err = s.db.ExecContext(ctx, `UPDATE assets SET current_version_id = ? WHERE id = ?`, versionID, assetID)
	if err != nil {
		return err
	}

	// Generate thumbnail synchronously
	thumbData, thumbExt, tErr := s.tmb.GenerateThumbnailData(ctx, s.storage, sp.entry.Mime, storageKey)
	if tErr == nil && thumbData != nil {
		thumbKey := fmt.Sprintf("demo/%s/%s/versions/%s/thumb%s", d.workspaceID, assetID, versionID, thumbExt)
		if putErr := s.storage.Put(ctx, thumbKey, bytes.NewReader(thumbData)); putErr == nil {
			s.db.ExecContext(ctx, `UPDATE asset_versions SET thumbnail_key = ? WHERE id = ?`, thumbKey, versionID)
			s.db.ExecContext(ctx, `UPDATE assets SET thumbnail_key = ? WHERE id = ?`, thumbKey, assetID)
		}
	}

	return nil
}

// versionComment returns the manifest-declared comment for versionNum (2..N),
// where entry.Versions[0] describes version 2, entry.Versions[1] describes
// version 3, and so on. Version 1 (the base file) never has a comment.
func versionComment(e manifestEntry, versionNum int) string {
	idx := versionNum - 2
	if idx < 0 || idx >= len(e.Versions) {
		return ""
	}
	return e.Versions[idx].Comment
}

// --- tags ---

func (s *Seeder) seedTags(ctx context.Context, tx *sql.Tx, d *ids) error {
	tagNames := []string{
		"approved",
		"hero",
		"social",
		"print",
		"web",
		"draft",
		"archive",
		"photography",
		"video",
		"brand",
	}
	tagIDs := map[string]string{}

	for _, name := range tagNames {
		id := newID("tag")
		_, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO tags (id, workspace_id, name) VALUES (?, ?, ?)
		`, id, d.workspaceID, name)
		if err != nil {
			return fmt.Errorf("demo: tag %q: %w", name, err)
		}
		// Fetch actual ID (may have been inserted by a concurrent seed)
		row := tx.QueryRowContext(ctx, `SELECT id FROM tags WHERE workspace_id = ? AND name = ?`, d.workspaceID, name)
		var actualID string
		if err := row.Scan(&actualID); err != nil {
			return fmt.Errorf("demo: fetch tag id %q: %w", name, err)
		}
		tagIDs[name] = actualID
	}

	addTag := func(assetID, tagName string) error {
		_, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO asset_tags (asset_id, tag_id) VALUES (?, ?)`,
			assetID, tagIDs[tagName])
		return err
	}

	// Tag assignment is manifest-driven (RA-4): each entry declares the tags
	// that genuinely apply to its content, validated against the fixed
	// vocabulary above when the manifest loads.
	for _, am := range d.allAssets {
		entry, ok := d.manifestByName[am.name]
		if !ok {
			continue
		}
		for _, tagName := range entry.Tags {
			if err := addTag(am.id, tagName); err != nil {
				return fmt.Errorf("demo: tag asset %s with %q: %w", am.name, tagName, err)
			}
		}
	}

	return nil
}

// --- field values ---

func (s *Seeder) seedFieldValues(ctx context.Context, tx *sql.Tx, d *ids) error {
	fieldIDByKey := map[string]string{
		"client":       d.fieldClient,
		"status":       d.fieldStatus,
		"usage_rights": d.fieldUsageRights,
		"photographer": d.fieldPhotographer,
		"licensed":     d.fieldLicensed,
	}

	upsert := func(assetID, fieldID string, text *string, date *string, boolean *int64) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO asset_field_values (id, asset_id, field_id, value_text, value_number, value_date, value_boolean, created_by)
			VALUES (?, ?, ?, ?, NULL, ?, ?, ?)
			ON CONFLICT(asset_id, field_id) DO UPDATE SET
			  value_text    = excluded.value_text,
			  value_date    = excluded.value_date,
			  value_boolean = excluded.value_boolean,
			  updated_at    = datetime('now')
		`, newID("afv"), assetID, fieldID, text, date, boolean, d.userID)
		return err
	}

	// Field values are manifest-driven (RA-1): each entry declares the
	// key/value pairs that apply to it, resolved against the field
	// definitions created in seedFieldDefinitions.
	for _, am := range d.allAssets {
		entry, ok := d.manifestByName[am.name]
		if !ok {
			continue
		}
		for key, val := range entry.Fields {
			fieldID := fieldIDByKey[key]
			if fieldID == "" {
				continue
			}
			switch key {
			case "usage_rights":
				date, err := resolveFieldDate(val)
				if err != nil {
					return fmt.Errorf("demo: asset %s field %q: %w", am.name, key, err)
				}
				if err := upsert(am.id, fieldID, nil, &date, nil); err != nil {
					return err
				}
			case "licensed":
				b := int64(0)
				if val == "true" {
					b = 1
				}
				if err := upsert(am.id, fieldID, nil, nil, &b); err != nil {
					return err
				}
			default:
				v := val
				if err := upsert(am.id, fieldID, &v, nil, nil); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// resolveFieldDate turns a manifest usage_rights value into an ISO date.
// A "+180d" / "-365d" style value is relative to seed time; anything else is
// taken as a literal date.
func resolveFieldDate(val string) (string, error) {
	if len(val) > 1 && (val[0] == '+' || val[0] == '-') && strings.HasSuffix(val, "d") {
		days, err := strconv.Atoi(val[1 : len(val)-1])
		if err != nil {
			return "", fmt.Errorf("invalid relative offset %q: %w", val, err)
		}
		if val[0] == '-' {
			days = -days
		}
		return time.Now().AddDate(0, 0, days).Format("2006-01-02"), nil
	}
	return val, nil
}

// --- share ---

func (s *Seeder) seedShare(ctx context.Context, tx *sql.Tx, d *ids) error {
	shareID := newID("sh")
	d.shareID = shareID
	expires := time.Now().Add(30 * 24 * time.Hour).Format("2006-01-02T15:04:05Z")

	_, err := tx.ExecContext(ctx, `
		INSERT INTO shares (id, workspace_id, created_by, label, target_type, target_id,
		                    expires_at, allow_comments, allow_download, created_at)
		VALUES (?, ?, ?, 'Summer Campaign — Client Delivery', 'project', ?,
		        ?, 1, 1, datetime('now'))
	`, shareID, d.workspaceID, d.userID, d.summerProjectID, expires)
	if err != nil {
		return fmt.Errorf("demo: create share: %w", err)
	}

	// Find a hero asset to attach comments to
	heroAssetID := d.assetBeachHero
	if heroAssetID == "" {
		heroAssetID = d.assetStudioHero
	}
	if heroAssetID == "" && len(d.allAssets) > 0 {
		heroAssetID = d.allAssets[0].id
	}

	comments := []struct{ name, body string }{
		{"John (Sportswear Co)", "Love the beach shot! Can we get a version without the logo?"},
		{"John (Sportswear Co)", "The studio shot is perfect, approved."},
	}
	for _, c := range comments {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO share_comments (id, share_id, asset_id, author_name, body, created_at)
			VALUES (?, ?, ?, ?, ?, datetime('now'))
		`, newID("sc"), shareID, heroAssetID, c.name, c.body)
		if err != nil {
			return fmt.Errorf("demo: share comment: %w", err)
		}
	}

	return nil
}

// --- ingress source ---

func (s *Seeder) seedIngressSource(ctx context.Context, tx *sql.Tx, d *ids) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO ingress_sources
		  (id, workspace_id, created_by, type, label, config, public_token,
		   enabled, poll_interval_min, created_at, updated_at)
		VALUES (?, ?, ?, 'email_api', 'Inbound Email (Demo)', '', ?,
		        0, 60, datetime('now'), datetime('now'))
	`, newID("is"), d.workspaceID, d.userID, uuid.NewString())
	return err
}

// --- events ---

func (s *Seeder) seedEvents(ctx context.Context, d *ids) error {
	if len(d.allAssets) == 0 {
		return nil
	}

	rng := rand.New(rand.NewSource(99))
	actors := []struct {
		id   string
		kind string
	}{
		{d.userID, "user"},
		{d.aliceID, "user"},
		{d.marcID, "user"},
	}

	eventCount := 0
	insert := func(workspaceID, assetID, userID, actorType, eventType, payload string, t time.Time) error {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO asset_events (id, workspace_id, asset_id, user_id, actor_type, event_type, payload, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, newID("ev"), workspaceID, assetID, userID, actorType, eventType, payload, t.UTC().Format("2006-01-02T15:04:05Z"))
		if err == nil {
			eventCount++
		}
		return err
	}

	// asset_created events — one per asset, spread over 14 days
	for _, am := range d.allAssets {
		actor := actors[rng.Intn(len(actors))]
		t := businessTime(rng, 14)
		if err := insert(d.workspaceID, am.id, actor.id, actor.kind, "asset_created",
			`{"source":"upload"}`, t); err != nil {
			return err
		}
	}

	// asset_tagged events
	for range 12 {
		am := d.allAssets[rng.Intn(len(d.allAssets))]
		actor := actors[rng.Intn(len(actors))]
		t := businessTime(rng, 12)
		tags := []string{"approved", "hero", "social", "brand", "photography"}
		tag := tags[rng.Intn(len(tags))]
		payload := fmt.Sprintf(`{"tag":%q}`, tag)
		if err := insert(d.workspaceID, am.id, actor.id, actor.kind, "asset_tagged", payload, t); err != nil {
			return err
		}
	}

	// asset_renamed events
	oldNames := []string{"IMG_0042.jpg", "untitled-1.jpg", "scan_final.jpg", "DSC00123.jpg", "new_asset.png"}
	for range 5 {
		am := d.allAssets[rng.Intn(len(d.allAssets))]
		actor := actors[rng.Intn(len(actors))]
		t := businessTime(rng, 10)
		payload, err := json.Marshal(audit.AssetRenamedPayload{
			V: 1, Before: oldNames[rng.Intn(len(oldNames))], After: am.name,
		})
		if err != nil {
			return err
		}
		if err := insert(d.workspaceID, am.id, actor.id, actor.kind, "asset_renamed", string(payload), t); err != nil {
			return err
		}
	}

	// asset_version_uploaded events for versioned assets
	for _, assetID := range []string{d.assetHomepageV2, d.assetLogoPrimary, d.assetBeachHero} {
		if assetID == "" {
			continue
		}
		actor := actors[rng.Intn(len(actors))]
		t := businessTime(rng, 7)
		if err := insert(d.workspaceID, assetID, actor.id, actor.kind, "asset_version_uploaded",
			`{"version_num":2}`, t); err != nil {
			return err
		}
	}

	// asset_field_set events
	statusBeforeValues := []string{"Draft", "In Review"}
	for range 10 {
		am := d.allAssets[rng.Intn(len(d.allAssets))]
		actor := actors[rng.Intn(len(actors))]
		t := businessTime(rng, 8)
		payload, err := json.Marshal(audit.AssetFieldSetPayload{
			V: 1, FieldKey: "status", FieldName: "Status",
			Before: statusBeforeValues[rng.Intn(len(statusBeforeValues))], After: "Approved",
		})
		if err != nil {
			return err
		}
		if err := insert(d.workspaceID, am.id, actor.id, actor.kind, "asset_field_set", string(payload), t); err != nil {
			return err
		}
	}

	// asset_moved events
	for range 4 {
		am := d.allAssets[rng.Intn(len(d.allAssets))]
		actor := actors[rng.Intn(len(actors))]
		t := businessTime(rng, 9)
		fromFolderID, toFolderID := d.logosFolder, d.videoFolder
		payload, err := json.Marshal(audit.AssetMovedPayload{
			V: 1, BeforeFolderID: &fromFolderID, AfterFolderID: &toFolderID,
		})
		if err != nil {
			return err
		}
		if err := insert(d.workspaceID, am.id, actor.id, actor.kind, "asset_moved", string(payload), t); err != nil {
			return err
		}
	}

	// asset_shared event
	if len(d.allAssets) > 0 {
		am := d.allAssets[0]
		t := businessTime(rng, 5)
		if err := insert(d.workspaceID, am.id, d.userID, "user", "asset_shared",
			fmt.Sprintf(`{"share_id":%q}`, d.shareID), t); err != nil {
			return err
		}
	}

	// asset_deleted + asset_restored pairs
	for range 4 {
		am := d.allAssets[rng.Intn(len(d.allAssets))]
		actor := actors[rng.Intn(len(actors))]
		t1 := businessTime(rng, 6)
		t2 := t1.Add(2 * time.Hour)
		if err := insert(d.workspaceID, am.id, actor.id, actor.kind, "asset_deleted", `{}`, t1); err != nil {
			return err
		}
		if err := insert(d.workspaceID, am.id, actor.id, actor.kind, "asset_restored", `{}`, t2); err != nil {
			return err
		}
	}

	slog.InfoContext(ctx, "demo: seed complete events", "events_created", eventCount)
	return nil
}

// --- helpers ---

func (s *Seeder) ensureGhostUser(ctx context.Context, email, name string) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM users WHERE email = ? LIMIT 1`, email).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("demo: check ghost user %s: %w", email, err)
	}

	id = newID("ghost")
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES (?, ?, '', ?, datetime('now'), datetime('now'))
	`, id, email, name)
	if err != nil {
		return "", fmt.Errorf("demo: insert ghost user %s: %w", email, err)
	}
	return id, nil
}

func newID(prefix string) string {
	return prefix + "_" + uuid.NewString()
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func md5hex(data []byte) string {
	h := md5.Sum(data)
	return hex.EncodeToString(h[:])
}

// backdateRand returns a random time within the last n days.
func backdateRand(rng *rand.Rand, days int) time.Time {
	offset := time.Duration(rng.Intn(days*24)) * time.Hour
	return time.Now().Add(-offset)
}

// businessTime returns a random weekday business-hours timestamp within the last n days.
func businessTime(rng *rand.Rand, days int) time.Time {
	for {
		t := backdateRand(rng, days)
		if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
			continue
		}
		hour := 8 + rng.Intn(10) // 8am–5pm
		return time.Date(t.Year(), t.Month(), t.Day(), hour, rng.Intn(60), 0, 0, time.UTC)
	}
}

// --- file generation ---

// buildAssetSpecs loads the asset manifest and resolves each entry's
// project/folder keys to the workspace-specific IDs created earlier in Seed.
func (s *Seeder) buildAssetSpecs(d *ids) ([]assetSpec, error) {
	mf, err := loadManifest()
	if err != nil {
		return nil, err
	}

	projectIDs := map[string]string{
		"brand":   d.brandProjectID,
		"summer":  d.summerProjectID,
		"website": d.websiteProjectID,
		"archive": d.archiveProjectID,
	}
	folderIDs := map[string]string{
		"":           "",
		"logos":      d.logosFolder,
		"colors":     d.colorsFolder,
		"photo":      d.photoFolder,
		"video":      d.videoFolder,
		"social":     d.socialFolder,
		"wireframes": d.wireframesFolder,
		"ui":         d.uiCompFolder,
		"exports":    d.exportsFolder,
		"printready": d.printReadyFolder,
	}

	d.manifestByName = make(map[string]manifestEntry, len(mf.Assets))
	specs := make([]assetSpec, 0, len(mf.Assets))
	for _, e := range mf.Assets {
		d.manifestByName[e.Name] = e
		specs = append(specs, assetSpec{
			entry:        e,
			projectID:    projectIDs[e.Project],
			folderID:     folderIDs[e.Folder],
			makeVersions: len(e.Versions) + 1,
		})
	}
	return specs, nil
}

// generateFile produces the bytes for one version of an asset.
// versionOffset is 0 for the base (first) version and increases for each
// later version added via addVersion. For file-backed real photos/videos
// with more than one version, every version except the last (the "final")
// renders as a desaturated, cropped "draft" derived from the same real file
// (RA-3.4) — no extra curated file needed per version.
func (s *Seeder) generateFile(
	sp *assetSpec,
	versionOffset int,
	_ *rand.Rand,
) (data []byte, width, height int, err error) {
	e := sp.entry
	isFinal := versionOffset >= sp.makeVersions-1

	if e.File != "" {
		data, err = assetsFS.ReadFile("assets/" + e.File)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("demo: read %s: %w", e.File, err)
		}
		if !isFinal && (e.Mime == "image/jpeg" || e.Mime == "image/png") {
			var w, h int
			data, w, h, err = draftVariant(data, e.Mime)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("demo: draft variant of %s: %w", e.Name, err)
			}
			return data, w, h, nil
		}
		return data, e.Width, e.Height, nil
	}

	switch e.Generator {
	case "wireframe":
		data = generateWireframePNG(e.Width, e.Height, hexColor(e.BGColor))
		return data, e.Width, e.Height, nil

	case "svg":
		data = generateSVG(e.Name)
		return data, 0, 0, nil

	case "pdf":
		data = generatePDF(e.Name)
		return data, 0, 0, nil

	case "zip":
		data, err = generateZip(e.Name)
		return data, 0, 0, err

	default:
		return nil, 0, 0, fmt.Errorf("demo: asset %s has neither file nor a known generator", e.Name)
	}
}

// draftVariant derives an "earlier draft" look from a real photo/PNG: a
// slight inward crop plus grayscale conversion. Used for every
// non-final version of a manifest entry that declares more than one version,
// so DM-1.5's "visually distinct versions" goal holds without needing
// separate curated files per version.
func draftVariant(data []byte, mime string) (out []byte, width, height int, err error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, err
	}

	b := img.Bounds()
	insetX := b.Dx() * 6 / 100
	insetY := b.Dy() * 6 / 100
	cropRect := image.Rect(b.Min.X+insetX, b.Min.Y+insetY, b.Max.X-insetX, b.Max.Y-insetY)

	gray := image.NewGray(image.Rect(0, 0, cropRect.Dx(), cropRect.Dy()))
	draw.Draw(gray, gray.Bounds(), img, cropRect.Min, draw.Src)

	var buf bytes.Buffer
	if mime == "image/png" {
		err = png.Encode(&buf, gray)
	} else {
		err = jpeg.Encode(&buf, gray, &jpeg.Options{Quality: 80})
	}
	if err != nil {
		return nil, 0, 0, err
	}
	return buf.Bytes(), gray.Bounds().Dx(), gray.Bounds().Dy(), nil
}

// hexColor parses a "rrggbb" manifest bg_color into an opaque [color.RGBA],
// falling back to light grey on any malformed value.
func hexColor(s string) color.RGBA {
	v, err := strconv.ParseUint(s, 16, 32)
	if len(s) != 6 || err != nil {
		return color.RGBA{R: 240, G: 240, B: 240, A: 255}
	}
	return color.RGBA{
		R: uint8(v >> 16 & 0xff), //nolint:gosec // masked to a byte, cannot overflow
		G: uint8(v >> 8 & 0xff),  //nolint:gosec // masked to a byte, cannot overflow
		B: uint8(v & 0xff),       //nolint:gosec // masked to a byte, cannot overflow
		A: 255,
	}
}

// generateWireframePNG draws a generic low-fidelity wireframe (nav bar, hero
// block, a row of content cards, footer bar) instead of a solid rectangle —
// used for the Website Redesign project's mockup/UI-component slots, which
// are synthetic by nature and aren't meant to look like photography (RA-3.3).
func generateWireframePNG(w, h int, bg color.RGBA) []byte {
	if w == 0 {
		w = 800
	}
	if h == 0 {
		h = 600
	}

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)

	fillRect := func(x0, y0, x1, y1 int, c color.RGBA) {
		if x1 <= x0 || y1 <= y0 {
			return
		}
		draw.Draw(img, image.Rect(x0, y0, x1, y1), &image.Uniform{C: c}, image.Point{}, draw.Src)
	}

	line := shade(bg, -60)
	box := shade(bg, -20)

	// Top nav bar with a logo chip and a few nav-item chips.
	navH := max(h/10, 20)
	if navH > h {
		navH = h
	}
	fillRect(0, 0, w, navH, line)
	fillRect(w/40, navH/4, w/40+80, navH*3/4, box)
	for i, x := 0, w-w/10-70; i < 3 && x > w/2; i, x = i+1, x-90 {
		fillRect(x, navH/4, x+70, navH*3/4, box)
	}

	// Hero block.
	heroY0 := navH + h/20
	heroY1 := min(heroY0+h*3/10, h)
	fillRect(w/10, heroY0, w-w/10, heroY1, box)

	// A row of three content cards.
	cardY0 := heroY1 + h/20
	cardY1 := cardY0 + h/5
	if cardY1 <= h {
		margin := w / 10
		gap := w / 40
		cardW := (w - 2*margin - 2*gap) / 3
		x := margin
		for range 3 {
			fillRect(x, cardY0, x+cardW, cardY1, box)
			x += cardW + gap
		}
	}

	// Footer bar.
	footerH := h / 14
	fillRect(0, h-footerH, w, h, line)

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// shade lightens (positive delta) or darkens (negative delta) a colour,
// clamped to the valid byte range.
func shade(c color.RGBA, delta int) color.RGBA {
	adj := func(v uint8) uint8 {
		n := max(int(v)+delta, 0)
		if n > 255 {
			n = 255
		}
		return uint8(n)
	}
	return color.RGBA{R: adj(c.R), G: adj(c.G), B: adj(c.B), A: 255}
}

func generateSVG(label string) []byte {
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
  <rect width="100" height="100" fill="#6366f1"/>
  <circle cx="50" cy="50" r="30" fill="#fff" opacity="0.8"/>
  <text x="50" y="54" font-family="sans-serif" font-size="8" text-anchor="middle" fill="#6366f1">%s</text>
</svg>`, label)
	return []byte(svg)
}

// pdfEscape escapes the characters PDF string literals treat specially.
func pdfEscape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`)
	return r.Replace(s)
}

// generatePDF builds a single-page PDF that reads as "a document" even at
// thumbnail size — a title, a rule, a few placeholder body lines, and a
// footer — rather than one line of small text on an otherwise blank page.
// Byte offsets in the xref table are computed from the actual object bytes
// (not hand-counted), so the file is valid without needing viewer repair.
func generatePDF(label string) []byte {
	lines := []string{
		"Lorem ipsum dolor sit amet, consectetur adipiscing elit.",
		"Sed do eiusmod tempor incididunt ut labore et dolore magna.",
		"Ut enim ad minim veniam, quis nostrud exercitation ullamco.",
		"Duis aute irure dolor in reprehenderit in voluptate velit.",
	}

	var content bytes.Buffer
	fmt.Fprintf(&content, "0.85 0.85 0.85 rg 0 0 612 792 re f\n")
	fmt.Fprintf(&content, "1 1 1 rg 36 36 540 720 re f\n")
	fmt.Fprintf(&content, "0 0 0 rg BT /F2 20 Tf 72 700 Td (%s) Tj ET\n", pdfEscape(label))
	fmt.Fprintf(&content, "0.6 0.6 0.6 RG 1 w 72 676 m 540 676 l S\n")
	y := 640
	for _, line := range lines {
		fmt.Fprintf(&content, "0.2 0.2 0.2 rg BT /F1 11 Tf 72 %d Td (%s) Tj ET\n", y, pdfEscape(line))
		y -= 22
	}
	fmt.Fprintf(&content, "0.6 0.6 0.6 RG 1 w 72 90 m 540 90 l S\n")
	fmt.Fprintf(&content, "0.5 0.5 0.5 rg BT /F1 8 Tf 72 74 Td (Damask DAM - demo placeholder document) Tj ET\n")

	objs := []string{
		"<</Type/Catalog/Pages 2 0 R>>",
		"<</Type/Pages/Kids[3 0 R]/Count 1>>",
		"<</Type/Page/MediaBox[0 0 612 792]/Parent 2 0 R/Contents 4 0 R" +
			"/Resources<</Font<</F1<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>" +
			"/F2<</Type/Font/Subtype/Type1/BaseFont/Helvetica-Bold>>>>>>>>",
		fmt.Sprintf("<</Length %d>>\nstream\n%sendstream", content.Len(), content.String()),
	}

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")

	offsets := make([]int, len(objs)+1) // 1-indexed; offsets[0] unused (free object 0)
	for i, o := range objs {
		offsets[i+1] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj%s\nendobj\n", i+1, o)
	}

	xrefStart := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(objs)+1)
	buf.WriteString("0000000000 65535 f \n")
	for _, off := range offsets[1:] {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&buf, "trailer<</Size %d/Root 1 0 R>>\nstartxref\n%d\n%%%%EOF", len(objs)+1, xrefStart)

	return buf.Bytes()
}

func generateZip(label string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("readme.txt")
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(w, "Demo archive: %s\n", label)
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
