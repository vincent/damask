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

func TestCreateVariantAutomationParamsValidate(t *testing.T) {
	tests := []struct {
		name string
		in   service.CreateVariantAutomationParams
		want error
	}{
		{"workspace", service.CreateVariantAutomationParams{AssetID: "ast_1", Scope: service.AutomationScopeWorkspace}, nil},
		{"project", service.CreateVariantAutomationParams{AssetID: "ast_1", Scope: service.AutomationScopeProject}, nil},
		{"folder", service.CreateVariantAutomationParams{AssetID: "ast_1", Scope: service.AutomationScopeFolder}, nil},
		{"asset", service.CreateVariantAutomationParams{AssetID: "ast_1", Scope: service.AutomationScopeAsset}, nil},
		{"missing asset", service.CreateVariantAutomationParams{Scope: service.AutomationScopeWorkspace}, apperr.ErrInvalidInput},
		{"bad scope", service.CreateVariantAutomationParams{AssetID: "ast_1", Scope: "global"}, apperr.ErrInvalidInput},
	}
	for _, tt := range tests {
		err := tt.in.Validate()
		if tt.want == nil && err != nil {
			t.Fatalf("%s: unexpected error: %v", tt.name, err)
		}
		if tt.want != nil && !errors.Is(err, tt.want) {
			t.Fatalf("%s: expected %v, got %v", tt.name, tt.want, err)
		}
	}
}
