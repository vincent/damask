package services

import (
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"

	"github.com/emersion/go-smtp"
)

type MailServer interface {
	Start() error
	AddHook(hook Hook)

	NewSession(c *smtp.Conn) (smtp.Session, error)
}

type MailServerImpl struct {
	srv   *smtp.Server
	hooks []Hook
	db    *dbgen.Queries
	queue *queue.Queue
}

func NewMailServer(addr, domain string, db *dbgen.Queries, q *queue.Queue) MailServer {
	be := &MailServerImpl{db: db, queue: q}
	be.srv = smtp.NewServer(be)

	be.srv.Addr = addr
	be.srv.Domain = domain
	be.srv.AllowInsecureAuth = true

	return be
}

func (s *MailServerImpl) Start() error {
	err := s.srv.ListenAndServe()
	return err
}

// NewSession is called after client greeting (EHLO, HELO).
func (bkd *MailServerImpl) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &Session{
		hooks: bkd.hooks,
		db:    bkd.db,
		queue: bkd.queue,
	}, nil
}

// AddHook adds a new hook to the mail server.
func (b *MailServerImpl) AddHook(hook Hook) {
	b.hooks = append(b.hooks, hook)
}
