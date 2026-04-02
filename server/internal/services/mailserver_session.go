package services

import (
	"errors"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/DusanKasan/parsemail"
	"github.com/emersion/go-smtp"

	nm "net/mail"
)

// A Session is returned after successful login.
type Session struct {
	hooks []Hook
	from  string
	to    string
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
	for _, h := range s.hooks {
		if h.Address == to {
			s.to = to
			return nil
		}
	}

	log.Println("mailserver: Unknown recipient: ", to)

	return fmt.Errorf("unknown recipient: %s", to)
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

		// for _, a := range email.Attachments{
		// 	do stuff with attachment
		// }

		if err := h.Trigger(s.from, email); err != nil {
			return err
		}
	}

	return nil
}

func (s *Session) Reset() {}

func (s *Session) Logout() error {
	return nil
}

func format(email parsemail.Email, text string) string {
	return fmt.Sprintf("mailserver: %s => %s : %s", formatFrom(email.From), email.Subject, text)
}

func formatFrom(addresses []*nm.Address) string {
	if len(addresses) == 0 {
		return "NO FROM ADDRESS"
	}
	var sb strings.Builder
	for _, a := range addresses {
		if sb.Len() > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%s <%s>", a.Name, a.Address))
	}
	return sb.String()
}
