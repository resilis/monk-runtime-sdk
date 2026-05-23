package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/resilis/monk-runtime-sdk/contracts"
)

type ProtocolEngine struct {
	state   contracts.ProtocolState
	pending map[string]contracts.ToolCall
	seq     int64
}

func NewProtocolEngine() *ProtocolEngine {
	return &ProtocolEngine{
		state:   contracts.ProtocolStateIdle,
		pending: make(map[string]contracts.ToolCall),
	}
}

func (e *ProtocolEngine) State() contracts.ProtocolState {
	return e.state
}

func (e *ProtocolEngine) NextActions(context.Context) ([]contracts.RuntimeAction, error) {
	return nil, nil
}

func (e *ProtocolEngine) ApplyModelEvent(_ context.Context, ev contracts.ModelEvent) ([]contracts.RuntimeAction, error) {
	e.state = contracts.ProtocolStateRunning
	actions := make([]contracts.RuntimeAction, 0, len(ev.ToolCalls)+1)

	if ev.AssistantDelta != "" {
		e.seq++
		actions = append(actions, contracts.RuntimeAction{
			Kind: "event",
			Event: &contracts.RuntimeEvent{
				Type:       "assistant_delta",
				Seq:        e.seq,
				OccurredAt: time.Now().UTC(),
				Payload: map[string]any{
					"delta": ev.AssistantDelta,
				},
			},
		})
	}

	for _, call := range ev.ToolCalls {
		if call.CallID == "" {
			return nil, contracts.NewRuntimeError(
				contracts.ErrProtocolViolationCode,
				"tool call missing call_id",
				false,
			)
		}
		if _, ok := e.pending[call.CallID]; ok {
			return nil, contracts.NewRuntimeError(
				contracts.ErrProtocolViolationCode,
				fmt.Sprintf("duplicate tool call_id %q", call.CallID),
				false,
			)
		}
		e.pending[call.CallID] = call
		e.seq++
		actions = append(actions, contracts.RuntimeAction{
			Kind:     "tool_call",
			ToolCall: &call,
			Event: &contracts.RuntimeEvent{
				Type:       "tool_use",
				CallID:     call.CallID,
				Tool:       call.Tool,
				Seq:        e.seq,
				OccurredAt: time.Now().UTC(),
				Payload: map[string]any{
					"args": call.Args,
				},
			},
		})
	}

	if ev.Done {
		e.state = contracts.ProtocolStateCompleted
	}

	return actions, nil
}

func (e *ProtocolEngine) ApplyToolResult(_ context.Context, res contracts.ToolResult) ([]contracts.RuntimeAction, error) {
	if res.CallID == "" {
		return nil, contracts.NewRuntimeError(
			contracts.ErrProtocolViolationCode,
			"tool result missing call_id",
			false,
		)
	}
	call, ok := e.pending[res.CallID]
	if !ok {
		return nil, contracts.NewRuntimeError(
			contracts.ErrProtocolViolationCode,
			fmt.Sprintf("tool result without matching tool_use for call_id %q", res.CallID),
			false,
		)
	}
	delete(e.pending, res.CallID)
	e.seq++

	action := contracts.RuntimeAction{
		Kind:       "tool_result",
		ToolResult: &res,
		Event: &contracts.RuntimeEvent{
			Type:       "tool_result",
			CallID:     res.CallID,
			Tool:       call.Tool,
			Seq:        e.seq,
			OccurredAt: time.Now().UTC(),
			Payload: map[string]any{
				"status": res.Status,
				"result": res.Result,
				"error":  res.Error,
			},
		},
	}

	return []contracts.RuntimeAction{action}, nil
}

var _ contracts.ProtocolEngine = (*ProtocolEngine)(nil)
