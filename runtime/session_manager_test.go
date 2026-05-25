package runtime

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/resilis/monk-runtime-sdk/contracts"
)

type testBlockingProvider struct {
	enterOnce sync.Once
	entered   chan struct{}
	release   chan struct{}
}

func (p *testBlockingProvider) StartTurn(ctx context.Context, req contracts.ProviderRequest) (contracts.ProviderTurn, error) {
	_ = req
	p.enterOnce.Do(func() { close(p.entered) })
	select {
	case <-p.release:
		return contracts.ProviderTurn{Done: true}, nil
	case <-ctx.Done():
		return contracts.ProviderTurn{}, ctx.Err()
	}
}

func (p *testBlockingProvider) ContinueTurn(ctx context.Context, turn contracts.ProviderTurn, toolResults []contracts.ToolResult) (contracts.ProviderTurn, error) {
	_ = ctx
	_ = turn
	_ = toolResults
	return contracts.ProviderTurn{Done: true}, nil
}

func TestSessionLifecycleSafety(t *testing.T) {
	t.Run("fails closed when provider is missing", func(t *testing.T) {
		manager := NewSessionManager(nil, nil)
		session, err := manager.CreateSession(context.Background(), contracts.SessionSpec{
			RunID: "run-fail-closed",
			Kind:  contracts.SessionKindSubagent,
		})
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}

		_, err = session.SendAndWait(context.Background(), contracts.SessionInput{Prompt: "ping"})
		if err == nil {
			t.Fatal("expected error when provider is missing")
		}
		if !contracts.IsCode(err, contracts.ErrSessionCreateCode) {
			t.Fatalf("expected %s, got %v", contracts.ErrSessionCreateCode, err)
		}
	})

	t.Run("times out safely", func(t *testing.T) {
		provider := &testBlockingProvider{
			entered: make(chan struct{}),
			release: make(chan struct{}),
		}
		manager := NewSessionManager(provider, nil)
		session, err := manager.CreateSession(context.Background(), contracts.SessionSpec{
			RunID:   "run-timeout",
			Kind:    contracts.SessionKindMain,
			Timeout: 20 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}

		_, err = session.SendAndWait(context.Background(), contracts.SessionInput{Prompt: "timeout"})
		if err == nil {
			t.Fatal("expected timeout error")
		}
		if !contracts.IsCode(err, contracts.ErrProviderTimeoutCode) {
			t.Fatalf("expected %s, got %v", contracts.ErrProviderTimeoutCode, err)
		}
	})

	t.Run("cancelled session fails closed", func(t *testing.T) {
		provider := &testBlockingProvider{
			entered: make(chan struct{}),
			release: make(chan struct{}),
		}
		close(provider.release)
		manager := NewSessionManager(provider, nil)
		session, err := manager.CreateSession(context.Background(), contracts.SessionSpec{
			RunID: "run-cancel",
			Kind:  contracts.SessionKindSidecar,
		})
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		if err := session.Cancel("cancelled in test"); err != nil {
			t.Fatalf("Cancel: %v", err)
		}

		_, err = session.SendAndWait(context.Background(), contracts.SessionInput{Prompt: "after-cancel"})
		if err == nil {
			t.Fatal("expected error after cancellation")
		}
		if !contracts.IsCode(err, contracts.ErrSessionCreateCode) {
			t.Fatalf("expected %s, got %v", contracts.ErrSessionCreateCode, err)
		}
	})

	t.Run("rejects concurrent requests", func(t *testing.T) {
		provider := &testBlockingProvider{
			entered: make(chan struct{}),
			release: make(chan struct{}),
		}
		manager := NewSessionManager(provider, nil)
		session, err := manager.CreateSession(context.Background(), contracts.SessionSpec{
			RunID: "run-concurrency",
			Kind:  contracts.SessionKindSubagent,
		})
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}

		errCh := make(chan error, 1)
		go func() {
			_, callErr := session.SendAndWait(context.Background(), contracts.SessionInput{Prompt: "first"})
			errCh <- callErr
		}()

		select {
		case <-provider.entered:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("first request did not reach provider")
		}

		_, err = session.SendAndWait(context.Background(), contracts.SessionInput{Prompt: "second"})
		if err == nil {
			t.Fatal("expected concurrent request error")
		}
		if !contracts.IsCode(err, contracts.ErrProtocolViolationCode) {
			t.Fatalf("expected %s, got %v", contracts.ErrProtocolViolationCode, err)
		}

		close(provider.release)
		select {
		case firstErr := <-errCh:
			if firstErr != nil {
				t.Fatalf("first request failed: %v", firstErr)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("first request did not complete")
		}
	})
}
