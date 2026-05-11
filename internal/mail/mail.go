package mail

import (
	"bytes"
	"context"
	"damask/server/internal/telemetry"
	"embed"
	"html/template"
	"log/slog"

	"github.com/wneessen/go-mail"
	"go.opentelemetry.io/otel/codes"
)

//go:embed templates/*.html
var templates embed.FS

type MailSenderConfig struct {
	Sender   string
	Host     string
	Port     int
	User     string
	Password string
	BaseUrl  string
}

type Mailer interface {
	SendInvite(ctx context.Context, to, role, token string) error
	SendWelcome(ctx context.Context, to, username, workspaceID string) error
	SendInviteAccepted(ctx context.Context, to, newMemberName, newMemberEmail, role string) error
	SendIngressSourceAdded(ctx context.Context, to, sourceName, workspaceID string) error
	SendIngressSourceFailed(ctx context.Context, to, sourceName, errMsg, workspaceID string) error
	SendIngressSourceDisabled(ctx context.Context, to, sourceName, errMsg, workspaceID string) error
	SendCommentPosted(ctx context.Context, to, authorName, shareLabel, commentBody string) error
	SendPasswordReset(ctx context.Context, to, token string) error
	SendEmailChangeConfirmation(ctx context.Context, to, newEmail, token string) error
}

func NewMailer(config *MailSenderConfig) Mailer {
	mailer := &MailerImpl{
		config,
		nil,
	}
	if len(config.Host) > 0 {
		client, err := mail.NewClient(
			config.Host,
			mail.WithPort(config.Port),
			mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover),
			mail.WithTLSPortPolicy(mail.TLSOpportunistic),
			mail.WithUsername(config.User),
			mail.WithPassword(config.Password),
		)
		if err != nil {
			slog.Error("mailer: failed to create new mail delivery client", "error", err, "host", config.Host, "port", config.Port)
			panic("unrecoverable error: mailer config is not usable")
		}
		mailer.client = client
	}
	return mailer
}

type MailerImpl struct {
	config *MailSenderConfig
	client *mail.Client
}

func (m *MailerImpl) PrepareAndSendWithTemplate(ctx context.Context, templateName string, to, subject string, data map[string]string) (err error) {
	ctx, span := telemetry.StartSpan(ctx, "mail.prepare")
	defer telemetry.EndSpan(span, err)

	content, err := templates.ReadFile("templates/" + templateName + ".html")
	if err != nil {
		slog.ErrorContext(ctx, "mailer: failed to load mail template", "error", err)
		return err
	}

	tpl, err := template.New("base").Parse(string(content))
	if err != nil {
		slog.ErrorContext(ctx, "mailer: failed to parse mail template", "error", err)
		return err
	}

	data["BaseUrl"] = m.config.BaseUrl
	data["FooterText"] = "Lorem ipsum"
	if len(data["ActionUrl"]) > 0 {
		data["ActionUrl"] = m.config.BaseUrl + data["ActionUrl"]
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		slog.ErrorContext(ctx, "mailer: failed to execute mail template", "error", err)
		return err
	}

	msg, err := m.Prepare(to, subject, buf.String())
	if err != nil {
		slog.ErrorContext(ctx, "mailer: failed to prepare mail", "error", err)
		return err

	} else {
		slog.InfoContext(ctx, "mailer: send mail", "to", to)
		err = m.Send(ctx, msg)
		if err != nil {
			slog.ErrorContext(ctx, "mailer: failed to send mail", "error", err, "host", m.config.Host, "port", m.config.Port)
			return err
		}
	}

	return nil
}

func (m *MailerImpl) PrepareAndSend(ctx context.Context, to, subject string, data map[string]string) error {
	return m.PrepareAndSendWithTemplate(ctx, "base", to, subject, data)
}

func (m *MailerImpl) Prepare(to, subject, body string) (message *mail.Msg, err error) {
	message = mail.NewMsg()
	if err := message.From(m.config.Sender); err != nil {
		slog.Error("mailer: failed to set [from] address", "error", err)
		return nil, err
	}
	if err := message.To(to); err != nil {
		slog.Error("mailer: failed to set [to] address", "error", err)
		return nil, err
	}
	message.Subject(subject)
	message.SetBodyString(mail.TypeTextHTML, body)
	// TODO: handle text
	return message, nil
}

func (m *MailerImpl) Send(ctx context.Context, message *mail.Msg) (err error) {
	ctx, span := telemetry.StartSpan(ctx, "mail.send")
	defer telemetry.EndSpan(span, err)

	if m.client != nil {
		if err := m.client.DialAndSendWithContext(ctx, message); err != nil {
			slog.ErrorContext(ctx, "mailer: failed to deliver mail", "error", err)
			return err
		}
	} else {
		err = message.WriteToSendmailWithContext(ctx, "sendmail")
		if err != nil {
			slog.ErrorContext(ctx, "mailer: failed to deliver mail with local sendmail", "error", err)
		}
	}

	span.SetStatus(codes.Ok, "mail sent")

	return err
}

func (m *MailerImpl) SendInvite(ctx context.Context, to, role, token string) error {
	return m.PrepareAndSend(
		ctx,
		to,
		"You've been invited to a Damask workspace",
		map[string]string{
			"Title":      "You're invited!",
			"Text":       "Someone thinks you'd be a great fit! you've been invited to collaborate on a Damask workspace as " + role + ". Damask is a self-hosted digital asset manager: upload, organise, tag, and share your files with your team. Click below to create your account and join the workspace.",
			"ActionText": "Accept invitation",
			"ActionUrl":  "/invite?token=" + token,
		},
	)
}

func (m *MailerImpl) SendWelcome(ctx context.Context, to, username, workspaceID string) error {
	return m.PrepareAndSend(
		ctx,
		to,
		"Welcome to Damask, "+username+"!",
		map[string]string{
			"Title":      "Your workspace is ready, " + username + "!",
			"Text":       "Welcome to Damask, your home for digital assets. Upload images, videos, PDFs, and more; organise them with folders, projects, and tags; create shareable links with optional passwords and expiry; and set up ingress sources to pull files in automatically.",
			"ActionText": "Explore your library",
			"ActionUrl":  "/library?ws=" + workspaceID,
		},
	)
}

func (m *MailerImpl) SendInviteAccepted(ctx context.Context, to, newMemberName, newMemberEmail, role string) error {
	return m.PrepareAndSend(
		ctx,
		to,
		"A new member joined your workspace",
		map[string]string{
			"Title": "A new member joined your workspace",
			"Text":  newMemberName + " (" + newMemberEmail + ") accepted your invite and joined as " + role + ".",
		},
	)
}

func (m *MailerImpl) SendIngressSourceAdded(ctx context.Context, to, sourceName, workspaceID string) error {
	return m.PrepareAndSend(
		ctx,
		to,
		"Ingress source configured: "+sourceName,
		map[string]string{
			"Title":      "Ingress source configured",
			"Text":       "Your ingress source \"" + sourceName + "\" is set up and will start polling shortly.",
			"ActionText": "View sources",
			"ActionUrl":  "/library/ingress?ws=" + workspaceID,
		},
	)
}

func (m *MailerImpl) SendIngressSourceFailed(ctx context.Context, to, sourceName, errMsg, workspaceID string) error {
	return m.PrepareAndSend(
		ctx,
		to,
		"Ingress source error: "+sourceName,
		map[string]string{
			"Title":      "Ingress source error",
			"Text":       "The ingress source \"" + sourceName + "\" encountered an error: " + errMsg,
			"ActionText": "View sources",
			"ActionUrl":  "/library/ingress?ws=" + workspaceID,
		},
	)
}

func (m *MailerImpl) SendIngressSourceDisabled(ctx context.Context, to, sourceName, errMsg, workspaceID string) error {
	return m.PrepareAndSend(
		ctx,
		to,
		"Ingress source disabled: "+sourceName,
		map[string]string{
			"Title":      "Ingress source disabled",
			"Text":       "The ingress source \"" + sourceName + "\" has been disabled after too many consecutive errors. Last error: " + errMsg + "\n\nEdit the source to re-enable polling.",
			"ActionText": "View sources",
			"ActionUrl":  "/library/ingress?ws=" + workspaceID,
		},
	)
}

func (m *MailerImpl) SendPasswordReset(ctx context.Context, to, token string) error {
	return m.PrepareAndSend(
		ctx,
		to,
		"Reset your Damask password",
		map[string]string{
			"Title":      "Reset your password",
			"Text":       "Someone requested a password reset for your Damask account. If this wasn't you, you can ignore this email. This link expires in 1 hour.",
			"ActionText": "Reset password",
			"ActionUrl":  "/reset-password?token=" + token,
		},
	)
}

func (m *MailerImpl) SendEmailChangeConfirmation(ctx context.Context, to, newEmail, token string) error {
	return m.PrepareAndSend(
		ctx,
		to,
		"Confirm your email address change",
		map[string]string{
			"Title":      "Confirm your email change",
			"Text":       "You requested to change your Damask account email to " + newEmail + ". If you didn't request this, your account is safe and no change has been made. This link expires in 24 hours.",
			"ActionText": "Confirm email change",
			"ActionUrl":  "/auth/confirm-email-change?token=" + token,
		},
	)
}

func (m *MailerImpl) SendCommentPosted(ctx context.Context, to, authorName, shareLabel, commentBody string) error {
	return m.PrepareAndSend(
		ctx,
		to,
		"New comment on your share: "+shareLabel,
		map[string]string{
			"Title": "New comment on \"" + shareLabel + "\"",
			"Text":  authorName + " wrote: " + commentBody,
		},
	)
}
