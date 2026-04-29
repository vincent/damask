package mailserver

import (
	"fmt"

	"github.com/DusanKasan/parsemail"
)

type Hook struct {
	Address     string `yaml:"address"`
	Name        string `yaml:"name"`
	WorkspaceID string `yaml:"workspace_id"`
}

func NewHook(emailAddress, name, WorkspaceID string) *Hook {
	return &Hook{
		Address:     emailAddress,
		Name:        name,
		WorkspaceID: WorkspaceID,
	}
}

func (h Hook) Trigger(from string, email parsemail.Email) error {

	fmt.Println("trigger from", from, email.Subject)

	return nil
}
