package runtime

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/resilis/monk-runtime-sdk/contracts"
)

type AgentRegistry struct {
	mu         sync.RWMutex
	definitions map[string]contracts.AgentDefinition
}

func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		definitions: make(map[string]contracts.AgentDefinition),
	}
}

func (r *AgentRegistry) Register(ctx context.Context, def contracts.AgentDefinition) error {
	if err := contextActive(ctx); err != nil {
		return err
	}
	normalized, err := normalizeAgentDefinition(def)
	if err != nil {
		return err
	}
	if err := r.Validate(ctx, normalized); err != nil {
		return err
	}

	key := canonicalAgentName(normalized.Name)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.definitions[key]; exists {
		return contracts.NewRuntimeError(contracts.ErrAgentRegistryCode, "duplicate agent name \""+normalized.Name+"\"", false)
	}
	r.definitions[key] = normalized
	return nil
}

func (r *AgentRegistry) Validate(ctx context.Context, def contracts.AgentDefinition) error {
	if err := contextActive(ctx); err != nil {
		return err
	}
	_, err := normalizeAgentDefinition(def)
	return err
}

func (r *AgentRegistry) Resolve(ctx context.Context, name string) (contracts.AgentDefinition, error) {
	if err := contextActive(ctx); err != nil {
		return contracts.AgentDefinition{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	trimmedName := strings.TrimSpace(name)
	if trimmedName != "" {
		resolved, ok := r.definitions[canonicalAgentName(trimmedName)]
		if !ok {
			return contracts.AgentDefinition{}, contracts.NewRuntimeError(contracts.ErrAgentRegistryCode, "agent \""+trimmedName+"\" not found", false)
		}
		return cloneAgentDefinition(resolved), nil
	}

	if orchestrator, ok := r.definitions[canonicalAgentName("orchestrator")]; ok {
		return cloneAgentDefinition(orchestrator), nil
	}

	if len(r.definitions) == 0 {
		return contracts.AgentDefinition{}, contracts.NewRuntimeError(contracts.ErrAgentRegistryCode, "agent registry is empty", false)
	}

	orderedKeys := make([]string, 0, len(r.definitions))
	for key := range r.definitions {
		orderedKeys = append(orderedKeys, key)
	}
	sort.Strings(orderedKeys)
	return cloneAgentDefinition(r.definitions[orderedKeys[0]]), nil
}

func (r *AgentRegistry) List(ctx context.Context) ([]contracts.AgentDefinition, error) {
	if err := contextActive(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	definitions := make([]contracts.AgentDefinition, 0, len(r.definitions))
	for _, def := range r.definitions {
		definitions = append(definitions, cloneAgentDefinition(def))
	}
	r.mu.RUnlock()

	sort.SliceStable(definitions, func(i, j int) bool {
		left := canonicalAgentName(definitions[i].Name)
		right := canonicalAgentName(definitions[j].Name)
		if left == right {
			return definitions[i].Name < definitions[j].Name
		}
		return left < right
	})
	return definitions, nil
}

func contextActive(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return contracts.WrapRuntimeError(contracts.ErrAgentRegistryCode, "agent registry context is not active", false, err)
	}
	return nil
}

func normalizeAgentDefinition(def contracts.AgentDefinition) (contracts.AgentDefinition, error) {
	normalized := contracts.AgentDefinition{
		Name:        strings.TrimSpace(def.Name),
		Model:       strings.TrimSpace(def.Model),
		System:      strings.TrimSpace(def.System),
		MultiAgent:  def.MultiAgent,
		Description: strings.TrimSpace(def.Description),
	}
	if normalized.Name == "" {
		return contracts.AgentDefinition{}, contracts.NewRuntimeError(contracts.ErrAgentSchemaCode, "agent name is required", false)
	}

	normalized.Tools = dedupeNonEmptyStrings(def.Tools)
	normalized.MCPServers = dedupeNonEmptyStrings(def.MCPServers)
	if def.Metadata != nil {
		normalized.Metadata = make(map[string]any, len(def.Metadata))
		for key, value := range def.Metadata {
			normalized.Metadata[key] = value
		}
	}
	return normalized, nil
}

func canonicalAgentName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func dedupeNonEmptyStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	result := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		key := canonicalAgentName(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func cloneAgentDefinition(def contracts.AgentDefinition) contracts.AgentDefinition {
	cloned := contracts.AgentDefinition{
		Name:        def.Name,
		Model:       def.Model,
		System:      def.System,
		MultiAgent:  def.MultiAgent,
		Description: def.Description,
	}
	if len(def.Tools) > 0 {
		cloned.Tools = append([]string(nil), def.Tools...)
	}
	if len(def.MCPServers) > 0 {
		cloned.MCPServers = append([]string(nil), def.MCPServers...)
	}
	if len(def.Metadata) > 0 {
		cloned.Metadata = make(map[string]any, len(def.Metadata))
		for key, value := range def.Metadata {
			cloned.Metadata[key] = value
		}
	}
	return cloned
}

var _ contracts.AgentRegistry = (*AgentRegistry)(nil)