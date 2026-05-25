package contracts

import (
	"encoding/json"
	"testing"
)

func TestCanonicalDefaultToolsHaveStableSchemas(t *testing.T) {
	defs := CanonicalDefaultToolDefinitions()
	if len(defs) == 0 {
		t.Fatal("expected canonical default tool definitions")
	}

	for _, def := range defs {
		if def.InputSchema == "" {
			t.Fatalf("tool %q missing input schema", def.Name)
		}
		if def.OutputSchema == "" {
			t.Fatalf("tool %q missing output schema", def.Name)
		}

		var inputSchema map[string]any
		if err := json.Unmarshal([]byte(def.InputSchema), &inputSchema); err != nil {
			t.Fatalf("tool %q has invalid input schema: %v", def.Name, err)
		}
		if inputSchema["type"] != "object" {
			t.Fatalf("tool %q input schema must be an object", def.Name)
		}

		var outputSchema map[string]any
		if err := json.Unmarshal([]byte(def.OutputSchema), &outputSchema); err != nil {
			t.Fatalf("tool %q has invalid output schema: %v", def.Name, err)
		}
		if outputSchema["type"] != "object" {
			t.Fatalf("tool %q output schema must be an object", def.Name)
		}
	}
}
