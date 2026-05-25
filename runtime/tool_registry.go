package runtime

import (
	"context"
	"strings"
	"sync"

	"github.com/resilis/monk-runtime-sdk/contracts"
)

type ToolRegistry struct {
	defaults contracts.DefaultToolRegistry

	mu      sync.RWMutex
	customs map[string]contracts.ToolDefinition
}

func NewToolRegistry(defaults contracts.DefaultToolRegistry) *ToolRegistry {
	if defaults == nil {
		registry := NewDefaultToolRegistry()
		defaults = registry
	}
	return &ToolRegistry{
		defaults: defaults,
		customs:  make(map[string]contracts.ToolDefinition),
	}
}

func (r *ToolRegistry) RegisterConsumerTool(ctx context.Context, definition contracts.ToolDefinition) error {
	if err := contextActive(ctx); err != nil {
		return err
	}
	if r == nil {
		return contracts.NewRuntimeError(contracts.ErrToolBackendCode, "tool registry is unavailable", false)
	}

	normalizedName := strings.TrimSpace(definition.Name)
	if normalizedName == "" {
		return contracts.NewRuntimeError(contracts.ErrToolBackendCode, "tool name is required", false)
	}

	if r.defaults.IsImmutable(normalizedName) {
		defaultDefinition, err := r.defaults.GetDefaultDefinition(ctx, normalizedName)
		if err != nil {
			return err
		}
		if !toolDefinitionsEqual(defaultDefinition, definition) {
			return contracts.NewRuntimeError(
				contracts.ErrToolBackendCode,
				"consumer cannot redefine canonical default tool name or schema",
				false,
			)
		}
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	key := canonicalToolName(normalizedName)
	if _, exists := r.customs[key]; exists {
		return contracts.NewRuntimeError(contracts.ErrToolBackendCode, "consumer tool is already registered", false)
	}
	definition.Name = normalizedName
	r.customs[key] = definition
	return nil
}

func (r *ToolRegistry) Resolve(ctx context.Context, name string) (contracts.ToolDefinition, bool, error) {
	if err := contextActive(ctx); err != nil {
		return contracts.ToolDefinition{}, false, err
	}
	if r == nil {
		return contracts.ToolDefinition{}, false, contracts.NewRuntimeError(contracts.ErrToolBackendCode, "tool registry is unavailable", false)
	}

	normalized := strings.TrimSpace(name)
	if normalized == "" {
		return contracts.ToolDefinition{}, false, contracts.NewRuntimeError(contracts.ErrToolBackendCode, "tool name is required", false)
	}

	if definition, err := r.defaults.GetDefaultDefinition(ctx, normalized); err == nil {
		return definition, true, nil
	}

	r.mu.RLock()
	definition, ok := r.customs[canonicalToolName(normalized)]
	r.mu.RUnlock()
	if !ok {
		return contracts.ToolDefinition{}, false, contracts.NewRuntimeError(contracts.ErrToolBackendCode, "tool is not registered", false)
	}
	return definition, false, nil
}

func canonicalToolName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func toolDefinitionsEqual(left contracts.ToolDefinition, right contracts.ToolDefinition) bool {
	return strings.TrimSpace(left.Name) == strings.TrimSpace(right.Name) &&
		strings.TrimSpace(left.Version) == strings.TrimSpace(right.Version) &&
		strings.TrimSpace(left.InputSchema) == strings.TrimSpace(right.InputSchema) &&
		strings.TrimSpace(left.OutputSchema) == strings.TrimSpace(right.OutputSchema) &&
		strings.TrimSpace(left.SafetyClass) == strings.TrimSpace(right.SafetyClass) &&
		left.NonOverridable == right.NonOverridable
}
