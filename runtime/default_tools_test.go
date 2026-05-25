package runtime

import (
	"context"
	"testing"

	"github.com/resilis/monk-runtime-sdk/contracts"
)

func TestDefaultToolSchemasStable(t *testing.T) {
	registry := NewDefaultToolRegistry()
	definitions, err := registry.ListDefaultDefinitions(context.Background())
	if err != nil {
		t.Fatalf("ListDefaultDefinitions: %v", err)
	}

	wantNames := []string{
		contracts.DefaultToolTerminal,
		contracts.DefaultToolFileRead,
		contracts.DefaultToolFileWrite,
		contracts.DefaultToolMemoryRead,
		contracts.DefaultToolMemoryWrite,
		contracts.DefaultToolSpawnSubagent,
	}
	if len(definitions) != len(wantNames) {
		t.Fatalf("expected %d default tool definitions, got %d", len(wantNames), len(definitions))
	}

	for idx, expectedName := range wantNames {
		if definitions[idx].Name != expectedName {
			t.Fatalf("definition index %d expected name %q, got %q", idx, expectedName, definitions[idx].Name)
		}
		if definitions[idx].Version != "v1" {
			t.Fatalf("definition %q expected version v1, got %q", definitions[idx].Name, definitions[idx].Version)
		}
		if definitions[idx].InputSchema == "" || definitions[idx].OutputSchema == "" {
			t.Fatalf("definition %q has empty schemas", definitions[idx].Name)
		}
		if !definitions[idx].NonOverridable {
			t.Fatalf("definition %q must be non-overridable", definitions[idx].Name)
		}
	}
}

func TestConsumerBackendCannotRedefineDefaultTools(t *testing.T) {
	toolRegistry := NewToolRegistry(NewDefaultToolRegistry())
	ctx := context.Background()

	canonical, ok := contracts.CanonicalDefaultToolDefinition(contracts.DefaultToolTerminal)
	if !ok {
		t.Fatal("expected canonical terminal tool definition")
	}

	if err := toolRegistry.RegisterConsumerTool(ctx, canonical); err != nil {
		t.Fatalf("expected canonical re-registration to be accepted, got %v", err)
	}

	redefined := canonical
	redefined.InputSchema = `{"type":"object","properties":{"command":{"type":"number"}}}`
	err := toolRegistry.RegisterConsumerTool(ctx, redefined)
	if err == nil {
		t.Fatal("expected immutable default tool redefinition to fail")
	}
	if !contracts.IsCode(err, contracts.ErrToolBackendCode) {
		t.Fatalf("expected %s, got %v", contracts.ErrToolBackendCode, err)
	}

	custom := contracts.ToolDefinition{Name: "project_custom", Version: "v1", InputSchema: "{}", OutputSchema: "{}"}
	if err := toolRegistry.RegisterConsumerTool(ctx, custom); err != nil {
		t.Fatalf("custom tool registration should succeed: %v", err)
	}
}
