package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

	turn, err := e.Provider.StartTurn(ctx, contracts.ProviderRequest{
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

	seq := int64(1)
	usage := contracts.UsageSummary{}
	for {
		toolResults, turnErr := e.processProviderTurn(ctx, spec.RunID, &seq, turn, &usage)
		if turnErr != nil {
			terminalErr := normalizeRuntimeError(turnErr, contracts.ErrToolExecutionCode, "tool execution failed", false)
			_ = e.emitFailureAndFinish(ctx, spec.RunID, &seq, usage, terminalErr)
			return contracts.RunOutcome{}, terminalErr
		}
		if turn.Done {
			break
		}

		nextTurn, continueErr := e.Provider.ContinueTurn(ctx, turn, toolResults)
		if continueErr != nil {
			terminalErr := contracts.WrapRuntimeError(
				contracts.ErrProviderTimeoutCode,
				"failed to continue provider turn",
				true,
				continueErr,
			)
			_ = e.emitFailureAndFinish(ctx, spec.RunID, &seq, usage, terminalErr)
			return contracts.RunOutcome{}, terminalErr
		}
		turn = nextTurn
	}

	outcome := contracts.RunOutcome{
		RunID:     spec.RunID,
		Status:    "completed",
		Completed: true,
		Usage:     usage,
	}
	if err := e.finish(ctx, spec.RunID, seq+1, outcome); err != nil {
		return contracts.RunOutcome{}, err
	}

	return outcome, nil
}

func (e *Engine) processProviderTurn(ctx context.Context, runID string, seq *int64, turn contracts.ProviderTurn, usage *contracts.UsageSummary) ([]contracts.ToolResult, error) {
	results := make([]contracts.ToolResult, 0)
	for _, rawEvent := range turn.ModelEvents {
		modelEvent := rawEvent
		usage.InputTokens += modelEvent.Usage.InputTokens
		usage.OutputTokens += modelEvent.Usage.OutputTokens
		usage.TotalTokens += modelEvent.Usage.TotalTokens

		for i, call := range modelEvent.ToolCalls {
			if strings.TrimSpace(call.CallID) == "" {
				modelEvent.ToolCalls[i].CallID = fmt.Sprintf("tool-call-%d", *seq+int64(i)+1)
			}
		}

		actions, err := e.Protocol.ApplyModelEvent(ctx, modelEvent)
		if err != nil {
			return nil, err
		}
		for _, action := range actions {
			if action.Event != nil {
				ev := *action.Event
				*seq = *seq + 1
				ev.Seq = *seq
				ev.RunID = runID
				ev.OccurredAt = time.Now().UTC()
				if err := e.publish(ctx, runID, ev); err != nil {
					return nil, err
				}
			}
			if action.Kind != "tool_call" || action.ToolCall == nil {
				continue
			}
			if e.ToolExecutor == nil {
				return nil, contracts.NewRuntimeError(
					contracts.ErrProtocolViolationCode,
					"tool executor adapter is required",
					false,
				)
			}

			result, execErr := e.ToolExecutor.ExecuteTool(ctx, *action.ToolCall)
			if result.CallID == "" {
				result.CallID = action.ToolCall.CallID
			}
			if result.Status == "" {
				result.Status = "completed"
			}
			if execErr != nil {
				result.Status = "failed"
				if result.Error == "" {
					result.Error = execErr.Error()
				}
			}

			resultActions, err := e.Protocol.ApplyToolResult(ctx, result)
			if err != nil {
				return nil, err
			}
			for _, resultAction := range resultActions {
				if resultAction.Event == nil {
					continue
				}
				ev := *resultAction.Event
				*seq = *seq + 1
				ev.Seq = *seq
				ev.RunID = runID
				ev.OccurredAt = time.Now().UTC()
				if err := e.publish(ctx, runID, ev); err != nil {
					return nil, err
				}
			}

			results = append(results, result)
			if execErr != nil {
				return nil, contracts.WrapRuntimeError(
					contracts.ErrToolExecutionCode,
					"tool execution failed",
					false,
					execErr,
				)
			}
		}
	}
	return results, nil
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

func (e *Engine) finish(ctx context.Context, runID string, seq int64, out contracts.RunOutcome) error {
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
	return e.publish(ctx, runID, contracts.RuntimeEvent{
		RunID:      runID,
		Type:       "run_done",
		Seq:        seq,
		OccurredAt: time.Now().UTC(),
		Usage:      out.Usage,
		Payload: map[string]any{
			"status": out.Status,
		},
	})
}

func normalizeRuntimeError(err error, defaultCode, defaultMessage string, defaultRetryable bool) *contracts.RuntimeError {
	var runtimeErr *contracts.RuntimeError
	if errors.As(err, &runtimeErr) {
		return runtimeErr
	}
	message := strings.TrimSpace(defaultMessage)
	if strings.TrimSpace(err.Error()) != "" {
		message = strings.TrimSpace(err.Error())
	}
	return contracts.WrapRuntimeError(defaultCode, message, defaultRetryable, err)
}

func (e *Engine) emitFailureAndFinish(ctx context.Context, runID string, seq *int64, usage contracts.UsageSummary, runtimeErr *contracts.RuntimeError) error {
	if runtimeErr == nil {
		runtimeErr = contracts.NewRuntimeError(contracts.ErrToolExecutionCode, "runtime failed", false)
	}

	message := strings.TrimSpace(runtimeErr.Message)
	if message == "" {
		message = strings.TrimSpace(runtimeErr.Error())
	}
	if message == "" {
		message = "runtime failed"
	}

	*seq = *seq + 1
	if err := e.publish(ctx, runID, contracts.RuntimeEvent{
		RunID:      runID,
		Type:       "session_error",
		Seq:        *seq,
		OccurredAt: time.Now().UTC(),
		Payload: map[string]any{
			"code":    strings.TrimSpace(runtimeErr.Code),
			"message": message,
		},
	}); err != nil {
		return err
	}

	outcome := contracts.RunOutcome{
		RunID:     runID,
		Status:    "failed",
		Completed: false,
		Usage:     usage,
		ErrorCode: strings.TrimSpace(runtimeErr.Code),
		Message:   message,
	}
	if outcome.ErrorCode == "" {
		outcome.ErrorCode = contracts.ErrToolExecutionCode
	}

	if err := e.finish(ctx, runID, *seq+1, outcome); err != nil {
		return err
	}
	*seq = *seq + 1
	return nil
}

var _ contracts.Runtime = (*Engine)(nil)
