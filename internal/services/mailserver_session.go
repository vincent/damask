package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
	"damask/server/internal/slug"

	"github.com/DusanKasan/parsemail"
	"github.com/emersion/go-smtp"
	"github.com/google/uuid"
)

// A Session is returned after successful login.
type Session struct {
	hooks []Hook
	from  string
	to    string
	db    *dbgen.Queries
	queue queue.JobQueue
}

// AuthPlain implements authentication using SASL PLAIN.
func (s Session) AuthPlain(username, password string) error {
	return errors.New("invalid username or password")
}

func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	log.Println("Mail from:", from)
	s.from = from
	return nil
}

func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	s.to = to
	return nil
}

func (s *Session) Data(r io.Reader) error {
	email, err := parsemail.Parse(r) // returns Email struct and error
	if err != nil {
		return err
	}

	for _, h := range s.hooks {
		if h.Address != s.to {
			continue
		}
		if err := h.Trigger(s.from, email); err != nil {
			return err
		}
	}

	parts := strings.Split(s.to, "@")
	if len(parts) != 2 {
		return fmt.Errorf("invalid recipient address: %s", s.to)
	}
	localPart := parts[0]

	if s.db != nil && s.queue != nil && localPart != s.to {
		ctx := context.Background()
		token, tag := slug.ParseSubaddress(localPart)
		src, err := s.db.GetIngressSourceByPublicToken(ctx, token)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				log.Printf("mailserver: lookup source token %q: %v", token, err)
			}
			return nil
		}
		if src.Enabled == 0 {
			log.Printf("mailserver: ignore disabled source %q", token)
			return nil
		}

		// Resolve folder from +tag subaddress if present
		var overrideFolderID string
		if tag != "" {
			if folder, err := s.db.GetFolderBySlug(ctx, dbgen.GetFolderBySlugParams{
				WorkspaceID: src.WorkspaceID,
				Slug:        &tag,
			}); err == nil {
				overrideFolderID = folder.ID
			} else {
				log.Printf("mailserver: no folder for tag %q in workspace %s (falling back to default)", tag, src.WorkspaceID)
			}
		}

		for _, att := range email.Attachments {
			if err := s.ingestAttachment(ctx, src, att, overrideFolderID); err != nil {
				log.Printf("mailserver: ingest %q for source %s: %v", att.Filename, src.ID, err)
			}
		}
	}

	return nil
}

func (s *Session) ingestAttachment(ctx context.Context, src dbgen.IngressSource, att parsemail.Attachment, overrideFolderID string) error {
	tmp, err := os.CreateTemp("", "email-ingest-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := io.Copy(tmp, att.Data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp: %w", err)
	}
	_ = tmp.Close()

	filename := att.Filename
	if filename == "" {
		filename = uuid.NewString()
	}

	entryID := uuid.NewString()
	entry, err := s.db.InsertIngressLogEntry(ctx, dbgen.InsertIngressLogEntryParams{
		ID:       entryID,
		SourceID: src.ID,
		RemoteID: tmpPath,
		Filename: filename,
	})
	if errors.Is(err, sql.ErrNoRows) { // INSERT OR IGNORE: duplicate
		_ = os.Remove(tmpPath)
		return nil
	}
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("insert log entry: %w", err)
	}

	payload, _ := json.Marshal(struct {
		SourceID         string `json:"source_id"`
		WorkspaceID      string `json:"workspace_id"`
		LogEntryID       string `json:"log_entry_id"`
		RemoteID         string `json:"remote_id"`
		Filename         string `json:"filename"`
		TmpPath          string `json:"tmp_path,omitempty"`
		OverrideFolderID string `json:"override_folder_id,omitempty"`
	}{
		SourceID:         src.ID,
		WorkspaceID:      src.WorkspaceID,
		LogEntryID:       entry.ID,
		RemoteID:         tmpPath,
		Filename:         filename,
		TmpPath:          tmpPath,
		OverrideFolderID: overrideFolderID,
	})

	if _, err := s.queue.Enqueue(ctx, src.WorkspaceID, queue.JobTypeIngestFetch, string(payload)); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("enqueue: %w", err)
	}
	return nil
}

func (s *Session) Reset() {}

func (s *Session) Logout() error {
	return nil
}
