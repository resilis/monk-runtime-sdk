package runtime

import (
	"context"
	"time"

	"github.com/resilis/monk-runtime-sdk/contracts"
)

type Engine struct {
	Provider     contracts.ProviderAdapter
	ToolExecutor contracts.ToolExecutor
	Persistence  contracts.PersistenceAdapter
	ControlPlane contracts.ControlPlaneReporter
	EventSink    contracts.EventSink
	Protocol     contracts.ProtocolEngine
}

func NewEngine(provider contracts.ProviderAdapter, toolExecutor contracts.ToolExecutor, persistence contracts.PersistenceAdapter, controlPlane contracts.ControlPlaneReporter, eventSink contracts.EventSink, protocol contracts.ProtocolEngine) *Engine {
	return &Engine{
		Provider:     provider,
		ToolExecutor: toolExecutor,
		Persistence:  persistence,
		ControlPlane: controlPlane,
		EventSink:    eventSink,
		Protocol:     protocol,
	}
}

func (e *Engine) Execute(ctx context.Context, spec contracts.RunSpec) (contracts.RunOutcome, error) {
	if spec.RunID == "" {
		return contracts.RunOutcome{}, contracts.NewRuntimeError(
			contracts.ErrProtocolViolationCode,
			"run_id is required",
			false,
		)
	}
	if e.Provider == nil {
		return contracts.RunOutcome{}, contracts.NewRuntimeError(
			contracts.ErrProtocolViolationCode,
			"provider adapter is required",
			false,
		)
	}

	if e.Protocol == nil {
		e.Protocol = NewProtocolEngine()
	}

	if e.Persistence != nil {
		err := e.Persistence.OnRunStarted(ctx, spec.RunID, contracts.RunMeta{
			OrgID:     spec.OrgID,
			UserID:    spec.UserID,
			AgentName: spec.AgentName,
		})
		if err != nil {
			return contracts.RunOutcome{}, contracts.WrapRuntimeError(
				contracts.ErrAdapterPersistCode,
				"failed to persist run start",
				false,
				err,
			)
		}
	}

	_, err := e.Provider.StartTurn(ctx, contracts.ProviderRequest{
		RunID:          spec.RunID,
		AgentName:      spec.AgentName,
		Messages:       spec.Messages,
		StrictToolMode: spec.StrictToolMode,
	})
	if err != nil {
		return contracts.RunOutcome{}, contracts.WrapRuntimeError(
			contracts.ErrProviderTimeoutCode,
			"failed to start provider turn",
			true,
			err,
		)
	}

	startedEvent := contracts.RuntimeEvent{
		RunID:      spec.RunID,
		Type:       "run_started",
		Seq:        1,
		OccurredAt: time.Now().UTC(),
	}
	if err := e.publish(ctx, spec.RunID, startedEvent); err != nil {
		return contracts.RunOutcome{}, err
	}

	outcome := contracts.RunOutcome{
		RunID:     spec.RunID,
		Status:    "completed",
		Completed: true,
	}
	if err := e.finish(ctx, spec.RunID, outcome); err != nil {
		return contracts.RunOutcome{}, err
	}

	return outcome, nil
}

func (e *Engine) publish(ctx context.Context, runID string, ev contracts.RuntimeEvent) error {
	if ev.RunID == "" {
		ev.RunID = runID
	}
	if e.EventSink != nil {
		if err := e.EventSink.Publish(ctx, ev); err != nil {
			return err
		}
	}
	if e.Persistence != nil {
		if err := e.Persistence.OnRuntimeEvent(ctx, runID, ev); err != nil {
			return contracts.WrapRuntimeError(
				contracts.ErrAdapterPersistCode,
				"failed to persist runtime event",
				false,
				err,
			)
		}
	}
	return nil
}

func (e *Engine) finish(ctx context.Context, runID string, out contracts.RunOutcome) error {
	if e.Persistence != nil {
		if err := e.Persistence.OnRunFinished(ctx, runID, out); err != nil {
			return contracts.WrapRuntimeError(
				contracts.ErrAdapterPersistCode,
				"failed to persist run completion",
				false,
				err,
			)
		}
	}
	if e.ControlPlane != nil {
		if err := e.ControlPlane.ReportUsage(ctx, runID, out.Usage); err != nil {
			return err
		}
	}
	if e.EventSink != nil {
		err := e.EventSink.Publish(ctx, contracts.RuntimeEvent{
			RunID:      runID,
			Type:       "run_done",
			Seq:        2,
			OccurredAt: time.Now().UTC(),
			Usage:      out.Usage,
			Payload: map[string]any{
				"status": out.Status,
			},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

var _ contracts.Runtime = (*Engine)(nil)
