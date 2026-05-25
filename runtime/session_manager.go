package runtime

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/resilis/monk-runtime-sdk/contracts"
)

type SessionManager struct {
	Provider        contracts.ProviderAdapter
	ToolExecutor    contracts.ToolExecutor
	ProtocolFactory func() contracts.ProtocolEngine
	Now             func() time.Time

	idSeq uint64
}

func NewSessionManager(provider contracts.ProviderAdapter, toolExecutor contracts.ToolExecutor) *SessionManager {
	return &SessionManager{
		Provider:        provider,
		ToolExecutor:    toolExecutor,
		ProtocolFactory: func() contracts.ProtocolEngine { return NewProtocolEngine() },
		Now:             func() time.Time { return time.Now().UTC() },
	}
}

func (m *SessionManager) CreateSession(ctx context.Context, spec contracts.SessionSpec) (contracts.SessionHandle, error) {
	if m == nil {
		return nil, contracts.NewRuntimeError(contracts.ErrSessionCreateCode, "session manager is unavailable", false)
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, contracts.WrapRuntimeError(contracts.ErrSessionCreateCode, "session creation context is not active", false, err)
		}
	}
	if strings.TrimSpace(string(spec.Kind)) == "" {
		spec.Kind = contracts.SessionKindMain
	}
	if spec.AllowedTools != nil {
		spec.AllowedTools = append([]string(nil), spec.AllowedTools...)
	}
	if spec.ExcludedTools != nil {
		spec.ExcludedTools = append([]string(nil), spec.ExcludedTools...)
	}
	if spec.Metadata != nil {
		metadataCopy := make(map[string]any, len(spec.Metadata))
		for key, value := range spec.Metadata {
			metadataCopy[key] = value
		}
		spec.Metadata = metadataCopy
	}
	id := strings.TrimSpace(spec.RunID)
	if id == "" {
		id = fmt.Sprintf("%s-%d", spec.Kind, atomic.AddUint64(&m.idSeq, 1))
	}

	if strings.TrimSpace(spec.Model) == "" {
		spec.Model = "auto"
	}

	handle := &managedSession{
		id:      id,
		spec:    spec,
		manager: m,
	}
	handle.baseCtx, handle.cancelBase = context.WithCancel(context.Background())
	return handle, nil
}

type managedSession struct {
	id      string
	spec    contracts.SessionSpec
	manager *SessionManager

	mu         sync.Mutex
	closed     bool
	busy       bool
	cancelBase context.CancelFunc
	baseCtx    context.Context
}

func (s *managedSession) ID() string {
	if s == nil {
		return ""
	}
	return s.id
}

func (s *managedSession) SendAndWait(ctx context.Context, input contracts.SessionInput) (contracts.SessionOutput, error) {
	if s == nil {
		return contracts.SessionOutput{}, contracts.NewRuntimeError(contracts.ErrSessionCreateCode, "session handle is unavailable", false)
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return contracts.SessionOutput{}, contracts.NewRuntimeError(contracts.ErrSessionCreateCode, "session is closed", false)
	}
	if s.busy {
		s.mu.Unlock()
		return contracts.SessionOutput{}, contracts.NewRuntimeError(contracts.ErrProtocolViolationCode, "session already has an in-flight request", false)
	}
	s.busy = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.busy = false
		s.mu.Unlock()
	}()

	if ctx == nil {
		ctx = context.Background()
	}
	opCtx, stop := s.linkCancellation(ctx)
	defer stop()

	if s.spec.Timeout > 0 {
		var cancel context.CancelFunc
		opCtx, cancel = context.WithTimeout(opCtx, s.spec.Timeout)
		defer cancel()
	}

	if err := opCtx.Err(); err != nil {
		return contracts.SessionOutput{}, err
	}

	manager := s.manager
	if manager == nil || manager.Provider == nil {
		return contracts.SessionOutput{}, contracts.NewRuntimeError(
			contracts.ErrSessionCreateCode,
			"provider adapter is required for session execution",
			false,
		)
	}

	request := contracts.ProviderRequest{
		RunID:          s.spec.RunID,
		AgentName:      s.spec.AgentName,
		Messages:       []contracts.Message{{Role: "user", Content: strings.TrimSpace(input.Prompt)}},
		StrictToolMode: s.spec.StrictToolMode,
	}
	turn, err := manager.Provider.StartTurn(opCtx, request)
	if err != nil {
		return contracts.SessionOutput{}, contracts.WrapRuntimeError(contracts.ErrProviderTimeoutCode, "failed to start session provider turn", true, err)
	}

	protocol := manager.protocol()
	usage := contracts.UsageSummary{}
	var contentBuilder strings.Builder

	for {
		results, chunkContent, turnErr := s.processTurn(opCtx, protocol, turn)
		usage.InputTokens += turnUsage(turn).InputTokens
		usage.OutputTokens += turnUsage(turn).OutputTokens
		usage.TotalTokens += turnUsage(turn).TotalTokens
		contentBuilder.WriteString(chunkContent)
		if turnErr != nil {
			return contracts.SessionOutput{}, turnErr
		}
		if turn.Done {
			break
		}

		nextTurn, continueErr := manager.Provider.ContinueTurn(opCtx, turn, results)
		if continueErr != nil {
			return contracts.SessionOutput{}, contracts.WrapRuntimeError(contracts.ErrProviderTimeoutCode, "failed to continue session provider turn", true, continueErr)
		}
		turn = nextTurn
	}

	return contracts.SessionOutput{
		Content: strings.TrimSpace(contentBuilder.String()),
		Usage:   usage,
		Done:    true,
	}, nil
}

func (s *managedSession) Cancel(reason string) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()
	if s.cancelBase != nil {
		s.cancelBase()
	}
	_ = reason
	return nil
}

func (s *managedSession) Close() error {
	return s.Cancel("closed")
}

func (s *managedSession) linkCancellation(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		select {
		case <-s.baseCtx.Done():
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

func (s *managedSession) processTurn(ctx context.Context, protocol contracts.ProtocolEngine, turn contracts.ProviderTurn) ([]contracts.ToolResult, string, error) {
	results := make([]contracts.ToolResult, 0)
	var contentBuilder strings.Builder
	for _, modelEvent := range turn.ModelEvents {
		if modelEvent.AssistantDelta != "" {
			contentBuilder.WriteString(modelEvent.AssistantDelta)
		}
		if protocol != nil {
			if _, err := protocol.ApplyModelEvent(ctx, modelEvent); err != nil {
				return nil, "", err
			}
		}
		for _, call := range modelEvent.ToolCalls {
			if s.manager.ToolExecutor == nil {
				return nil, "", contracts.NewRuntimeError(contracts.ErrProtocolViolationCode, "tool executor adapter is required for tool calls", false)
			}
			result, execErr := s.manager.ToolExecutor.ExecuteTool(ctx, call)
			if strings.TrimSpace(result.CallID) == "" {
				result.CallID = call.CallID
			}
			if strings.TrimSpace(result.Status) == "" {
				result.Status = "completed"
			}
			if execErr != nil {
				if strings.TrimSpace(result.Status) == "" || strings.EqualFold(result.Status, "completed") {
					result.Status = "failed"
				}
				if strings.TrimSpace(result.Error) == "" {
					result.Error = execErr.Error()
				}
			}
			if protocol != nil {
				if _, err := protocol.ApplyToolResult(ctx, result); err != nil {
					return nil, "", err
				}
			}
			results = append(results, result)
			if execErr != nil {
				return nil, "", contracts.WrapRuntimeError(contracts.ErrToolExecutionCode, "tool execution failed", false, execErr)
			}
		}
	}
	return results, contentBuilder.String(), nil
}

func turnUsage(turn contracts.ProviderTurn) contracts.UsageSummary {
	usage := contracts.UsageSummary{}
	for _, modelEvent := range turn.ModelEvents {
		usage.InputTokens += modelEvent.Usage.InputTokens
		usage.OutputTokens += modelEvent.Usage.OutputTokens
		usage.TotalTokens += modelEvent.Usage.TotalTokens
	}
	return usage
}

func (m *SessionManager) protocol() contracts.ProtocolEngine {
	if m == nil || m.ProtocolFactory == nil {
		return nil
	}
	return m.ProtocolFactory()
}

var _ contracts.SessionManager = (*SessionManager)(nil)
var _ contracts.SessionHandle = (*managedSession)(nil)
