package service

import (
	"context"
	"log/slog"
)

type nopWorkflowTriggerPublisher struct{}

func (nopWorkflowTriggerPublisher) Dispatch(context.Context, string, map[string]any) error {
	return nil
}

func workflowTriggerPublisherOrNop(publishers ...WorkflowTriggerPublisher) WorkflowTriggerPublisher {
	if len(publishers) > 0 && publishers[0] != nil {
		return publishers[0]
	}
	return nopWorkflowTriggerPublisher{}
}

func publishWorkflowTriggerAsync(publisher WorkflowTriggerPublisher, eventType string, data map[string]any) {
	if _, ok := publisher.(nopWorkflowTriggerPublisher); ok {
		return
	}

	payload := make(map[string]any, len(data))
	for k, v := range data {
		payload[k] = v
	}

	go func() {
		if err := publisher.Dispatch(context.Background(), eventType, payload); err != nil {
			slog.Warn("workflow trigger dispatch failed", "trigger_type", eventType, "error", err)
		}
	}()
}
