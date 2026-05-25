package runtime

import (
	"context"
	"testing"

	"github.com/resilis/monk-runtime-sdk/contracts"
)

type fixedResponseProvider struct {
	response string
}

func (p fixedResponseProvider) StartTurn(context.Context, contracts.ProviderRequest) (contracts.ProviderTurn, error) {
	return contracts.ProviderTurn{
		ModelEvents: []contracts.ModelEvent{{AssistantDelta: p.response, Done: true}},
		Done:        true,
	}, nil
}

func (p fixedResponseProvider) ContinueTurn(context.Context, contracts.ProviderTurn, []contracts.ToolResult) (contracts.ProviderTurn, error) {
	return contracts.ProviderTurn{Done: true}, nil
}

type noopToolExecutor struct{}

func (noopToolExecutor) ExecuteTool(context.Context, contracts.ToolCall) (contracts.ToolResult, error) {
	return contracts.ToolResult{Status: "completed"}, nil
}

func TestEngineSessionManagerBackfillUsesCurrentProviderAndToolExecutor(t *testing.T) {
	engine := NewEngine(nil, nil, nil, nil, nil, nil)
	_ = engine.SessionManager()

	engine.Provider = fixedResponseProvider{response: "ok"}
	engine.ToolExecutor = noopToolExecutor{}

	sessionManager, ok := engine.SessionManager().(*SessionManager)
	if !ok {
		t.Fatalf("expected *SessionManager, got %T", engine.SessionManager())
	}
	if sessionManager.Provider == nil {
		t.Fatal("expected provider to be synchronized into session manager")
	}
	if sessionManager.ToolExecutor == nil {
		t.Fatal("expected tool executor to be synchronized into session manager")
	}

	session, err := sessionManager.CreateSession(context.Background(), contracts.SessionSpec{
		RunID: "run-sync",
		Kind:  contracts.SessionKindSubagent,
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	out, err := session.SendAndWait(context.Background(), contracts.SessionInput{Prompt: "hello"})
	if err != nil {
		t.Fatalf("SendAndWait: %v", err)
	}
	if out.Content != "ok" {
		t.Fatalf("expected provider-backed content, got %q", out.Content)
	}
}
