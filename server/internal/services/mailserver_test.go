package services

import (
	"strings"
	"testing"
)

const rawEmail = "From: sender@example.com\r\n" +
	"To: hook@example.com\r\n" +
	"Subject: Test Subject\r\n" +
	"MIME-Version: 1.0\r\n" +
	"Content-Type: text/plain; charset=utf-8\r\n" +
	"\r\n" +
	"Hello, world!\r\n"

func TestSession_HookTriggeredOnMatchingRecipient(t *testing.T) {
	hook := Hook{
		Address:     "hook@example.com",
		Name:        "test-hook",
		WorkspaceID: "ws-1",
	}
	session := &Session{hooks: []Hook{hook}}

	if err := session.Mail("sender@example.com", nil); err != nil {
		t.Fatalf("Mail: %v", err)
	}

	if err := session.Rcpt("hook@example.com", nil); err != nil {
		t.Fatalf("Rcpt: %v", err)
	}

	if err := session.Data(strings.NewReader(rawEmail)); err != nil {
		t.Fatalf("Data: %v", err)
	}
}

func TestSession_RcptRejectsUnknownRecipient(t *testing.T) {
	hook := Hook{
		Address:     "hook@example.com",
		Name:        "test-hook",
		WorkspaceID: "ws-1",
	}
	session := &Session{hooks: []Hook{hook}}

	if err := session.Mail("sender@example.com", nil); err != nil {
		t.Fatalf("Mail: %v", err)
	}

	err := session.Rcpt("other@example.com", nil)
	if err == nil {
		t.Fatal("expected error for unknown recipient, got nil")
	}
}

func TestMailServerImpl_AddHookAndNewSession(t *testing.T) {
	hook := Hook{
		Address:     "hook@example.com",
		Name:        "test-hook",
		WorkspaceID: "ws-1",
	}

	impl := &MailServerImpl{}
	impl.AddHook(hook)

	session, err := impl.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	s, ok := session.(*Session)
	if !ok {
		t.Fatalf("expected *Session, got %T", session)
	}
	if len(s.hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(s.hooks))
	}
	if s.hooks[0].Address != hook.Address {
		t.Errorf("hook address: got %q, want %q", s.hooks[0].Address, hook.Address)
	}
}

func TestSession_DataNotTriggeredForNonMatchingHook(t *testing.T) {
	// Hook address differs from the accepted recipient — Data should not call Trigger.
	// We register two hooks: only one matches the recipient.
	hookMatch := Hook{Address: "hook@example.com", Name: "match", WorkspaceID: "ws-1"}
	hookOther := Hook{Address: "other@example.com", Name: "other", WorkspaceID: "ws-2"}

	session := &Session{hooks: []Hook{hookOther, hookMatch}}

	if err := session.Mail("sender@example.com", nil); err != nil {
		t.Fatalf("Mail: %v", err)
	}
	if err := session.Rcpt("hook@example.com", nil); err != nil {
		t.Fatalf("Rcpt: %v", err)
	}
	// Data should succeed — only hookMatch is triggered, hookOther is skipped.
	if err := session.Data(strings.NewReader(rawEmail)); err != nil {
		t.Fatalf("Data: %v", err)
	}
}
