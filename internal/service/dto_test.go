package service_test

import (
	"errors"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/service"
)

const validWorkflowGraph = `{"nodes":[{"id":"n1","type":"trigger.manual","config":{},"position":{"x":0,"y":0}}],"edges":[]}`

func TestCreateWorkflowParamsValidate(t *testing.T) {
	tests := []struct {
		name string
		in   service.CreateWorkflowParams
	}{
		{
			name: "ok",
			in: service.CreateWorkflowParams{
				Name:  "My Workflow",
				Graph: validWorkflowGraph,
			},
		},
		{
			name: "invalid graph topology",
			in: service.CreateWorkflowParams{
				Name:  "Broken",
				Graph: `{"nodes":[],"edges":[]}`,
			},
		},
		{
			name: "invalid failure email",
			in: service.CreateWorkflowParams{
				Name:                 "Broken",
				Graph:                validWorkflowGraph,
				NotifyOnFailureEmail: "not-an-email",
			},
		},
	}

	for _, tt := range tests {
		err := tt.in.Validate()
		switch tt.name {
		case "ok":
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", tt.name, err)
			}
		default:
			if !errors.Is(err, apperr.ErrInvalidInput) {
				t.Fatalf("%s: expected ErrInvalidInput, got %v", tt.name, err)
			}
		}
	}
}

func TestUpdateWorkflowParamsValidate(t *testing.T) {
	graph := validWorkflowGraph
	badGraph := `{"nodes":[{"id":"n1","type":"trigger.manual","config":{},"position":{"x":0,"y":0}},{"id":"n2","type":"trigger.manual","config":{},"position":{"x":1,"y":1}}],"edges":[]}`
	badEmail := "bad"

	if err := (service.UpdateWorkflowParams{Graph: &graph}).Validate(); err != nil {
		t.Fatalf("expected valid graph update, got %v", err)
	}
	if err := (service.UpdateWorkflowParams{Graph: &badGraph}).Validate(); !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for graph, got %v", err)
	}
	if err := (service.UpdateWorkflowParams{NotifyOnFailureEmail: &badEmail}).Validate(); !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for email, got %v", err)
	}
}
