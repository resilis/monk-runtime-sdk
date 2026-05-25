package contracts

import (
	"math"
	"path/filepath"
	"testing"
)

func TestMergeToolPolicyDenyWins(t *testing.T) {
	base := ToolPolicy{
		AllowTools: []string{DefaultToolTerminal, DefaultToolFileRead},
	}
	consumer := ToolPolicy{
		DenyTools: []string{DefaultToolTerminal},
	}

	merged := MergeToolPolicy(base, consumer)

	if !deniedByName(merged.DenyTools, DefaultToolTerminal) {
		t.Fatalf("expected %q to be in deny list", DefaultToolTerminal)
	}
	if allowedByName(merged.AllowTools, DefaultToolTerminal) {
		t.Fatalf("expected %q to be removed from allow list", DefaultToolTerminal)
	}
	if !allowedByName(merged.AllowTools, DefaultToolFileRead) {
		t.Fatalf("expected %q to remain in allow list", DefaultToolFileRead)
	}
}

func TestEvaluateToolPolicyFailClosedWithoutExplicitAllow(t *testing.T) {
	err := EvaluateToolPolicy(ToolPolicy{}, DefaultToolTerminal, map[string]any{"command": "echo hi"})
	if !IsCode(err, ErrPolicyDeniedCode) {
		t.Fatalf("expected policy denied, got: %v", err)
	}
}

func TestEvaluateToolPolicyPathBoundarySafeContainment(t *testing.T) {
	safePrefix := filepath.Join("sandbox", "safe")
	policy := ToolPolicy{
		AllowTools:          []string{DefaultToolFileRead},
		AllowedPathPrefixes: []string{safePrefix},
	}

	outsidePath := filepath.Join("sandbox", "safe-other", "file.txt")
	err := EvaluateToolPolicy(policy, DefaultToolFileRead, map[string]any{"path": outsidePath})
	if !IsCode(err, ErrPolicyDeniedCode) {
		t.Fatalf("expected deny for outside path, got: %v", err)
	}

	insidePath := filepath.Join("sandbox", "safe", "file.txt")
	err = EvaluateToolPolicy(policy, DefaultToolFileRead, map[string]any{"path": insidePath})
	if err != nil {
		t.Fatalf("expected inside path to be allowed, got: %v", err)
	}
}

func TestEvaluateToolPolicyRejectsInvalidNumericArgs(t *testing.T) {
	policy := ToolPolicy{
		AllowTools:          []string{DefaultToolMemoryRead},
		AllowedPathPrefixes: []string{"mem"},
		MaxReadBytes:        4096,
	}

	err := EvaluateToolPolicy(policy, DefaultToolMemoryRead, map[string]any{
		"memory_path": filepath.Join("mem", "segment"),
		"length":      1.5,
	})
	if !IsCode(err, ErrPolicyDeniedCode) {
		t.Fatalf("expected deny for fractional length, got: %v", err)
	}
}

func TestReadInt64ArgStrictParsing(t *testing.T) {
	tests := []struct {
		name   string
		raw    any
		want   int64
		wantOK bool
	}{
		{name: "int value", raw: int(42), want: 42, wantOK: true},
		{name: "integer float", raw: float64(42), want: 42, wantOK: true},
		{name: "fractional float", raw: float64(42.25), wantOK: false},
		{name: "string invalid", raw: "42", wantOK: false},
		{name: "uint64 overflow", raw: uint64(math.MaxInt64) + 1, wantOK: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := map[string]any{"value": tc.raw}
			got, ok := readInt64Arg(args, "value")
			if ok != tc.wantOK {
				t.Fatalf("ok mismatch: want %v, got %v", tc.wantOK, ok)
			}
			if ok && got != tc.want {
				t.Fatalf("value mismatch: want %d, got %d", tc.want, got)
			}
		})
	}
}

func TestReadIntArgStrictParsing(t *testing.T) {
	args := map[string]any{"fanout": float64(3.75)}
	if _, ok := readIntArg(args, "fanout"); ok {
		t.Fatal("expected fractional float fanout to be rejected")
	}
}
