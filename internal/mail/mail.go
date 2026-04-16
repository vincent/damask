package mail

import (
	"bytes"
	"context"
	"embed"
	"html/template"
	"log/slog"

	"github.com/wneessen/go-mail"
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
	PrepareAndSendWithTemplate(ctx context.Context, templateName string, to, subject string, data map[string]string) error
	PrepareAndSend(ctx context.Context, to, subject string, data map[string]string) error
	Prepare(to, subject, body string) (message *mail.Msg, err error)
	Send(ctx context.Context, message *mail.Msg) error
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
		} else {
			mailer.client = client
		}
	}
	return mailer
}

type MailerImpl struct {
	config *MailSenderConfig
	client *mail.Client
}

func (m *MailerImpl) PrepareAndSendWithTemplate(ctx context.Context, templateName string, to, subject string, data map[string]string) error {
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
	}

	msg, err := m.Prepare(to, subject, buf.String())
	if err != nil {
		slog.ErrorContext(ctx, "mailer: failed to prepare mail", "error", err)
		return err

	} else {
		err = m.Send(ctx, msg)
		if err != nil {
			slog.Error("mailer: failed to create new mail delivery client", "error", err, "host", m.config.Host, "port", m.config.Port, "message", msg)
			slog.ErrorContext(ctx, "failed to send mail", "error", err)
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

func (m *MailerImpl) Send(ctx context.Context, message *mail.Msg) error {
	var err error
	if m.client != nil {
		if err := m.client.DialAndSendWithContext(ctx, message); err != nil {
			slog.ErrorContext(ctx, "mailer: failed to deliver mail", "error", err)
			return err
		}
	} else {
		err = message.WriteToSendmailWithContext(ctx, "sendmail")
		slog.ErrorContext(ctx, "mailer: failed to deliver mail with local sendmail", "error", err)
	}

	return err
}
