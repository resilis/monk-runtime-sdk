package runtime

import (
	"context"
	"testing"

	"github.com/resilis/monk-runtime-sdk/contracts"
)

func TestAgentRegistryDuplicateRegistrationRejection(t *testing.T) {
	registry := NewAgentRegistry()
	ctx := context.Background()

	if err := registry.Register(ctx, contracts.AgentDefinition{Name: "orchestrator"}); err != nil {
		t.Fatalf("register orchestrator: %v", err)
	}

	err := registry.Register(ctx, contracts.AgentDefinition{Name: "  Orchestrator  "})
	if err == nil {
		t.Fatal("expected duplicate registration error")
	}
	if !contracts.IsCode(err, contracts.ErrAgentRegistryCode) {
		t.Fatalf("expected %s, got %v", contracts.ErrAgentRegistryCode, err)
	}
}

func TestAgentRegistryResolveRequiredAgentInvariants(t *testing.T) {
	t.Run("explicit missing agent fails closed", func(t *testing.T) {
		registry := NewAgentRegistry()
		ctx := context.Background()
		if err := registry.Register(ctx, contracts.AgentDefinition{Name: "orchestrator"}); err != nil {
			t.Fatalf("register orchestrator: %v", err)
		}

		_, err := registry.Resolve(ctx, "planner")
		if err == nil {
			t.Fatal("expected resolve error for missing agent")
		}
		if !contracts.IsCode(err, contracts.ErrAgentRegistryCode) {
			t.Fatalf("expected %s, got %v", contracts.ErrAgentRegistryCode, err)
		}
	})

	t.Run("empty resolve name prefers orchestrator when present", func(t *testing.T) {
		registry := NewAgentRegistry()
		ctx := context.Background()
		if err := registry.Register(ctx, contracts.AgentDefinition{Name: "reviewer"}); err != nil {
			t.Fatalf("register reviewer: %v", err)
		}
		if err := registry.Register(ctx, contracts.AgentDefinition{Name: "orchestrator"}); err != nil {
			t.Fatalf("register orchestrator: %v", err)
		}

		resolved, err := registry.Resolve(ctx, "")
		if err != nil {
			t.Fatalf("resolve default agent: %v", err)
		}
		if resolved.Name != "orchestrator" {
			t.Fatalf("expected orchestrator default, got %q", resolved.Name)
		}
	})

	t.Run("empty registry resolve fails", func(t *testing.T) {
		registry := NewAgentRegistry()
		_, err := registry.Resolve(context.Background(), "")
		if err == nil {
			t.Fatal("expected resolve error for empty registry")
		}
		if !contracts.IsCode(err, contracts.ErrAgentRegistryCode) {
			t.Fatalf("expected %s, got %v", contracts.ErrAgentRegistryCode, err)
		}
	})
}

func TestAgentRegistryDeterministicResolveAndListBehavior(t *testing.T) {
	registry := NewAgentRegistry()
	ctx := context.Background()

	if err := registry.Register(ctx, contracts.AgentDefinition{Name: "zeta"}); err != nil {
		t.Fatalf("register zeta: %v", err)
	}
	if err := registry.Register(ctx, contracts.AgentDefinition{Name: "beta"}); err != nil {
		t.Fatalf("register beta: %v", err)
	}
	if err := registry.Register(ctx, contracts.AgentDefinition{Name: "alpha"}); err != nil {
		t.Fatalf("register alpha: %v", err)
	}

	firstResolve, err := registry.Resolve(ctx, "")
	if err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	secondResolve, err := registry.Resolve(ctx, "")
	if err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	if firstResolve.Name != "alpha" || secondResolve.Name != "alpha" {
		t.Fatalf("expected deterministic default resolve to alpha, got %q and %q", firstResolve.Name, secondResolve.Name)
	}

	firstList, err := registry.List(ctx)
	if err != nil {
		t.Fatalf("first list: %v", err)
	}
	secondList, err := registry.List(ctx)
	if err != nil {
		t.Fatalf("second list: %v", err)
	}

	if len(firstList) != 3 || len(secondList) != 3 {
		t.Fatalf("expected 3 agents in both list calls, got %d and %d", len(firstList), len(secondList))
	}

	for i, expected := range []string{"alpha", "beta", "zeta"} {
		if firstList[i].Name != expected {
			t.Fatalf("first list[%d] expected %q, got %q", i, expected, firstList[i].Name)
		}
		if secondList[i].Name != expected {
			t.Fatalf("second list[%d] expected %q, got %q", i, expected, secondList[i].Name)
		}
	}
}