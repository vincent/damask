package service

import (
	"context"
	"log/slog"
	"maps"
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

func publishWorkflowTriggerAsync(
	ctx context.Context,
	publisher WorkflowTriggerPublisher,
	eventType string,
	data map[string]any,
) {
	if _, ok := publisher.(nopWorkflowTriggerPublisher); ok {
		return
	}

	payload := make(map[string]any, len(data))
	maps.Copy(payload, data)

	bgCtx := context.WithoutCancel(ctx)
	go func() {
		if err := publisher.Dispatch(bgCtx, eventType, payload); err != nil {
			slog.WarnContext(bgCtx, "workflow trigger dispatch failed", "trigger_type", eventType, "error", err)
		}
	}()
}
