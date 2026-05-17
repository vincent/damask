package workflow

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"damask/server/internal/audit"
	"damask/server/internal/events"
	"damask/server/internal/repository"
	"damask/server/internal/telemetry"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type WorkflowExecutor struct {
	deps Deps
}

func NewExecutor(deps Deps) *WorkflowExecutor {
	return &WorkflowExecutor{deps: deps}
}

func (e *WorkflowExecutor) Run(ctx context.Context, runID string) error {
	var err error
	ctx, span := tracer.Start(ctx, "workflow.run", trace.WithAttributes(attribute.String("workflow.run_id", runID)))
	defer telemetry.EndSpan(span, err)

	run, err := e.deps.Runs.GetByID(ctx, runID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	wf, err := e.deps.Workflows.GetByID(ctx, run.WorkspaceID, run.WorkflowID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetAttributes(
		attribute.String("workflow.id", wf.ID),
		attribute.String("damask.workspace_id", wf.WorkspaceID),
	)

	var graph Graph
	if err := json.Unmarshal([]byte(wf.Graph), &graph); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid graph JSON")
		_ = e.deps.Runs.SetFinal(ctx, repository.SetWorkflowRunFinalParams{
			ID:          runID,
			Status:      "failed",
			Context:     run.Context,
			Error:       ptr(err.Error()),
			CompletedAt: nowPtr(),
		})
		return err
	}

	rc := NewRunContext(jsonToMap(run.TriggerData))
	rc.Set("workspace_id", wf.WorkspaceID)
	rc.Set("workflow_id", wf.ID)
	rc.Set("workflow_created_by", wf.CreatedBy)

	if err := e.deps.Runs.SetStatus(ctx, runID, "running"); err != nil {
		return err
	}

	trigger, err := graph.TriggerNode()
	if err != nil {
		return err
	}
	runErr := e.executeNode(ctx, &graph, trigger, rc, runID, wf.WorkspaceID, wf.ID)

	status := "completed"
	if runErr != nil {
		status = "failed"
		span.RecordError(runErr)
		span.SetStatus(codes.Error, runErr.Error())
	}

	finalErr := e.deps.Runs.SetFinal(ctx, repository.SetWorkflowRunFinalParams{
		ID:          runID,
		Status:      status,
		Context:     mustJSON(rc),
		Error:       errStringPtr(runErr),
		CompletedAt: nowPtr(),
	})
	if finalErr != nil && runErr == nil {
		runErr = finalErr
	}
	_ = e.deps.Workflows.TouchLastRunAt(ctx, wf.ID)
	if status == "failed" && runErr != nil {
		e.reportRunFailure(ctx, wf, runID, rc, runErr)
	}
	return runErr
}

func (e *WorkflowExecutor) executeNode(ctx context.Context, g *Graph, node GraphNode, rc *RunContext, runID, workspaceID, workflowID string) error {
	var err error
	ctx, span := tracer.Start(ctx, "workflow.step", trace.WithAttributes(
		attribute.String("workflow.node_id", node.ID),
		attribute.String("workflow.node_type", node.Type),
		attribute.String("workflow.run_id", runID),
	))
	defer telemetry.EndSpan(span, err)

	n, err := Build(e.deps, node.Type)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	stepID := newID()
	step, err := e.deps.Runs.CreateStep(ctx, repository.CreateWorkflowRunStepParams{
		ID:        stepID,
		RunID:     runID,
		NodeID:    node.ID,
		NodeType:  node.Type,
		Status:    "running",
		Attempt:   1,
		InputCtx:  mustJSON(rc),
		StartedAt: nowPtr(),
	})
	if err != nil {
		return err
	}

	policy := retryPolicyFromConfig(node.Config)
	var outPort string
	var updates map[string]any
	var execErr error
	for attempt := 1; ; attempt++ {
		if attempt > 1 {
			_ = e.deps.Runs.IncrementStepAttempt(ctx, step.ID)
			span.AddEvent("retry", trace.WithAttributes(attribute.Int("attempt", attempt)))
		}
		outPort, updates, execErr = n.Execute(ctx, rc, node.Config)
		if execErr == nil || !policy.ShouldRetry(attempt, execErr) {
			break
		}
		time.Sleep(policy.WaitFor(attempt))
	}
	if execErr != nil {
		span.RecordError(execErr)
		span.SetStatus(codes.Error, execErr.Error())
		_ = e.deps.Runs.SetStepFailed(ctx, stepID, execErr.Error())
		e.publishStepEvent(ctx, workspaceID, rc, runID, workflowID, node.ID, "failed", execErr.Error())
		for _, next := range g.Successors(node.ID, "error") {
			if err := e.executeNode(ctx, g, next, rc.Clone(), runID, workspaceID, workflowID); err != nil {
				return err
			}
		}
		if len(g.Successors(node.ID, "error")) > 0 {
			return nil
		}
		return execErr
	}

	rc.Merge(updates)
	span.SetAttributes(attribute.String("workflow.output_port", outPort))
	_ = e.deps.Runs.SetStepCompleted(ctx, stepID, mustJSON(rc))
	e.publishStepEvent(ctx, workspaceID, rc, runID, workflowID, node.ID, "completed", "")

	successors := g.Successors(node.ID, outPort)
	switch len(successors) {
	case 0:
		return nil
	case 1:
		return e.executeNode(ctx, g, successors[0], rc, runID, workspaceID, workflowID)
	default:
		// Each parallel branch gets its own snapshot of rc; writes in one branch
		// do not propagate to siblings or to the parent after the fan-out joins.
		var wg sync.WaitGroup
		var mu sync.Mutex
		var firstErr error
		for _, next := range successors {
			wg.Add(1)
			go func(next GraphNode) {
				defer wg.Done()
				if err := e.executeNode(ctx, g, next, rc.Clone(), runID, workspaceID, workflowID); err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
				}
			}(next)
		}
		wg.Wait()
		return firstErr
	}
}

func (e *WorkflowExecutor) reportRunFailure(ctx context.Context, wf repository.Workflow, runID string, rc *RunContext, runErr error) {
	if e.deps.Hub != nil {
		assetID, _ := rcGetString(rc, "asset_id")
		e.deps.Hub.Publish(ctx, wf.WorkspaceID, events.Event{
			Type:       "workflow_run_failed",
			AssetID:    assetID,
			WorkflowID: wf.ID,
			RunID:      runID,
			Error:      runErr.Error(),
		})
	}

	if e.deps.Audit != nil {
		if assetID, ok := rcGetString(rc, "asset_id"); ok && assetID != "" {
			e.deps.Audit.WriteAsset(ctx, audit.AssetEvent{
				WorkspaceID: wf.WorkspaceID,
				AssetID:     assetID,
				ActorType:   "system",
				EventType:   "workflow_run_failed",
				Payload: map[string]any{
					"workflow_id":   wf.ID,
					"workflow_name": wf.Name,
					"run_id":        runID,
					"error":         runErr.Error(),
				},
			})
		}
	}

	if e.deps.Mailer != nil && wf.NotifyOnFailureEmail != "" {
		_ = e.deps.Mailer.SendWorkflowRunFailed(ctx, wf.NotifyOnFailureEmail, wf.Name, runErr.Error(), wf.WorkspaceID)
	}
}

func (e *WorkflowExecutor) publishStepEvent(ctx context.Context, workspaceID string, rc *RunContext, runID, workflowID, nodeID, status, errMsg string) {
	if e.deps.Hub == nil {
		return
	}
	assetID, _ := rcGetString(rc, "asset_id")
	e.deps.Hub.Publish(ctx, workspaceID, events.Event{
		Type:       "workflow_run_step_updated",
		AssetID:    assetID,
		WorkflowID: workflowID,
		RunID:      runID,
		NodeID:     nodeID,
		Status:     status,
		Error:      errMsg,
	})
}

func errStringPtr(err error) *string {
	if err == nil {
		return nil
	}
	msg := err.Error()
	return &msg
}
