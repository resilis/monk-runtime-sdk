package runtime

import (
	"path/filepath"
	"testing"

	"github.com/resilis/monk-runtime-sdk/contracts"
)

func TestToolPolicyDenyWins(t *testing.T) {
	evaluator := NewToolPolicyEvaluator()
	base := contracts.ToolPolicy{
		AllowTools:          []string{contracts.DefaultToolTerminal, contracts.DefaultToolFileRead, contracts.DefaultToolSpawnSubagent},
		AllowedPathPrefixes: []string{"/workspace", "/memories"},
		AllowedCommands:     []string{"go test", "rg", "cat"},
		MaxReadBytes:        4096,
		MaxWriteBytes:       1024,
		MaxSubagentFanout:   2,
	}
	consumer := contracts.ToolPolicy{
		DenyTools:           []string{contracts.DefaultToolTerminal},
		DeniedPathPrefixes:  []string{"/workspace/secrets"},
		DeniedCommands:      []string{"go test ./..."},
		MaxReadBytes:        512,
		MaxWriteBytes:       256,
		MaxSubagentFanout:   1,
		AllowedCommands:     []string{"go test ./...", "go test ./internal/orchestrator"},
		AllowedPathPrefixes: []string{"/workspace", "/workspace/secrets"},
	}

	merged := evaluator.Merge(base, consumer)

	if err := evaluator.Evaluate(merged, contracts.DefaultToolTerminal, map[string]any{"command": "go test ./internal/orchestrator"}); err == nil {
		t.Fatal("expected terminal tool to be denied by deny-wins merge")
	}

	if err := evaluator.Evaluate(merged, contracts.DefaultToolFileRead, map[string]any{"path": "/workspace/secrets/token.txt", "length": 10}); err == nil {
		t.Fatal("expected denied path prefix to win")
	}

	if err := evaluator.Evaluate(merged, contracts.DefaultToolFileRead, map[string]any{"path": "/workspace/main.go", "length": 1024}); err == nil {
		t.Fatal("expected max_read_bytes merged lower bound to apply")
	}

	if err := evaluator.Evaluate(merged, contracts.DefaultToolSpawnSubagent, map[string]any{"fanout": 2}); err == nil {
		t.Fatal("expected merged max_subagent_fanout to enforce lower bound")
	}
}

func TestToolPolicyMergePreservesUsableAllowedPathPrefixes(t *testing.T) {
	evaluator := NewToolPolicyEvaluator()
	base := contracts.ToolPolicy{
		AllowTools:          []string{contracts.DefaultToolFileRead},
		AllowedPathPrefixes: []string{"/workspace"},
	}
	consumer := contracts.ToolPolicy{
		AllowedPathPrefixes: []string{"/workspace"},
		DeniedPathPrefixes:  []string{"/workspace/secrets"},
	}

	merged := evaluator.Merge(base, consumer)

	if err := evaluator.Evaluate(merged, contracts.DefaultToolFileRead, map[string]any{"path": "/workspace/app/main.go", "length": 32}); err != nil {
		t.Fatalf("expected allowed workspace path to pass merged policy, got: %v", err)
	}

	if err := evaluator.Evaluate(merged, contracts.DefaultToolFileRead, map[string]any{"path": "/workspace/secrets/token.txt", "length": 16}); err == nil {
		t.Fatal("expected denied path prefix to block access")
	}

	if err := evaluator.Evaluate(merged, contracts.DefaultToolFileRead, map[string]any{"path": "/etc/passwd", "length": 16}); err == nil {
		t.Fatal("expected out-of-scope path to be denied")
	}
}

func TestDefaultToolPolicyFailClosedWithoutExplicitPathOrCommandAllowances(t *testing.T) {
	evaluator := NewToolPolicyEvaluator()
	policy := DefaultToolPolicy()

	if err := evaluator.Evaluate(policy, contracts.DefaultToolFileRead, map[string]any{"path": "/workspace/main.go", "length": 32}); err == nil {
		t.Fatal("expected default file_read policy to fail closed without explicit allowed path prefixes")
	}

	if err := evaluator.Evaluate(policy, contracts.DefaultToolTerminal, map[string]any{"command": "echo delegated"}); err == nil {
		t.Fatal("expected default terminal policy to fail closed without explicit allowed commands")
	}
}

func TestToolPolicyCanonicalizationBlocksPathEscape(t *testing.T) {
	evaluator := NewToolPolicyEvaluator()
	policy := contracts.ToolPolicy{
		AllowTools:          []string{contracts.DefaultToolFileRead},
		AllowedPathPrefixes: []string{"/workspace"},
		DeniedPathPrefixes:  []string{"/workspace/secrets"},
	}

	escapePath := filepath.Join("/workspace", "safe", "..", "secrets", "token.txt")
	err := evaluator.Evaluate(policy, contracts.DefaultToolFileRead, map[string]any{"path": escapePath, "length": 1})
	if err == nil {
		t.Fatal("expected canonicalized path traversal into denied prefix to be blocked")
	}
}
