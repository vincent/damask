package mailserver

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
	queue queue.JobQueue
}

func NewMailServer(addr, domain string, db *dbgen.Queries, q queue.JobQueue) MailServer {
	backend := &MailServerImpl{db: db, queue: q}
	backend.srv = smtp.NewServer(backend)

	backend.srv.Addr = addr
	backend.srv.Domain = domain
	backend.srv.AllowInsecureAuth = true

	return backend
}

func (backend *MailServerImpl) Start() error {
	err := backend.srv.ListenAndServe()
	return err
}

// NewSession is called after client greeting (EHLO, HELO).
func (backend *MailServerImpl) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &Session{
		hooks: backend.hooks,
		db:    backend.db,
		queue: backend.queue,
	}, nil
}

// AddHook adds a new hook to the mail server.
func (backend *MailServerImpl) AddHook(hook Hook) {
	backend.hooks = append(backend.hooks, hook)
}
