package runtime

import (
	"context"
	"strings"

	"github.com/resilis/monk-runtime-sdk/contracts"
)

type DefaultToolRegistry struct{}

func NewDefaultToolRegistry() DefaultToolRegistry {
	return DefaultToolRegistry{}
}

func (r DefaultToolRegistry) ListDefaultDefinitions(ctx context.Context) ([]contracts.ToolDefinition, error) {
	if err := contextActive(ctx); err != nil {
		return nil, err
	}
	definitions := contracts.CanonicalDefaultToolDefinitions()
	cloned := make([]contracts.ToolDefinition, len(definitions))
	copy(cloned, definitions)
	return cloned, nil
}

func (r DefaultToolRegistry) GetDefaultDefinition(ctx context.Context, name string) (contracts.ToolDefinition, error) {
	if err := contextActive(ctx); err != nil {
		return contracts.ToolDefinition{}, err
	}
	definition, ok := contracts.CanonicalDefaultToolDefinition(strings.TrimSpace(name))
	if !ok {
		return contracts.ToolDefinition{}, contracts.NewRuntimeError(
			contracts.ErrToolBackendCode,
			"default tool definition not found",
			false,
		)
	}
	return definition, nil
}

func (r DefaultToolRegistry) IsImmutable(name string) bool {
	return contracts.IsImmutableDefaultTool(strings.TrimSpace(name))
}

var _ contracts.DefaultToolRegistry = DefaultToolRegistry{}
