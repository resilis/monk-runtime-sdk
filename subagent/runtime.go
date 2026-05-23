package subagent

import (
	"context"

	"github.com/resilis/monk-runtime-sdk/contracts"
)

type Runtime struct {
	parent contracts.Runtime
}

func NewRuntime(parent contracts.Runtime) *Runtime {
	return &Runtime{parent: parent}
}

func (r *Runtime) RunSubagent(ctx context.Context, req contracts.SubagentRequest) (contracts.SubagentResult, error) {
	if r.parent == nil {
		return contracts.SubagentResult{}, contracts.NewRuntimeError(
			contracts.ErrProtocolViolationCode,
			"subagent parent runtime is required",
			false,
		)
	}

	outcome, err := r.parent.Execute(ctx, contracts.RunSpec{
		RunID:       req.ParentRunID,
		AgentName:   req.AgentName,
		Messages:    req.Messages,
		MemoryPolicy: req.Policy,
	})
	if err != nil {
		return contracts.SubagentResult{}, err
	}

	return contracts.SubagentResult{Outcome: outcome}, nil
}

var _ contracts.SubagentRuntime = (*Runtime)(nil)
