package contracts

import "context"

type ToolDefinition struct {
	Name           string
	Version        string
	InputSchema    string
	OutputSchema   string
	SafetyClass    string
	NonOverridable bool
}

const (
	DefaultToolTerminal      = "terminal"
	DefaultToolFileRead      = "file_read"
	DefaultToolFileWrite     = "file_write"
	DefaultToolMemoryRead    = "memory_read"
	DefaultToolMemoryWrite   = "memory_write"
	DefaultToolSpawnSubagent = "spawn_subagent"
)

var canonicalDefaultToolNames = []string{
	DefaultToolTerminal,
	DefaultToolFileRead,
	DefaultToolFileWrite,
	DefaultToolMemoryRead,
	DefaultToolMemoryWrite,
	DefaultToolSpawnSubagent,
}

var canonicalDefaultToolDefinitions = map[string]ToolDefinition{
	DefaultToolTerminal: {
		Name:           DefaultToolTerminal,
		Version:        "v1",
		InputSchema:    `{"type":"object","properties":{"command":{"type":"string","minLength":1},"cwd":{"type":"string"},"timeout_ms":{"type":"integer","minimum":1}},"required":["command"],"additionalProperties":true}`,
		OutputSchema:   `{"type":"object","properties":{"exit_code":{"type":"integer"},"stdout":{"type":"string"},"stderr":{"type":"string"},"timed_out":{"type":"boolean"}},"required":["exit_code"],"additionalProperties":true}`,
		SafetyClass:    "privileged",
		NonOverridable: true,
	},
	DefaultToolFileRead: {
		Name:           DefaultToolFileRead,
		Version:        "v1",
		InputSchema:    `{"type":"object","properties":{"path":{"type":"string","minLength":1},"offset":{"type":"integer","minimum":0},"length":{"type":"integer","minimum":0}},"required":["path"],"additionalProperties":true}`,
		OutputSchema:   `{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"},"bytes_read":{"type":"integer","minimum":0}},"required":["content"],"additionalProperties":true}`,
		SafetyClass:    "filesystem-read",
		NonOverridable: true,
	},
	DefaultToolFileWrite: {
		Name:           DefaultToolFileWrite,
		Version:        "v1",
		InputSchema:    `{"type":"object","properties":{"path":{"type":"string","minLength":1},"content":{"type":"string"},"append":{"type":"boolean"},"bytes_length":{"type":"integer","minimum":0}},"required":["path","content"],"additionalProperties":true}`,
		OutputSchema:   `{"type":"object","properties":{"path":{"type":"string"},"bytes_written":{"type":"integer","minimum":0}},"required":["bytes_written"],"additionalProperties":true}`,
		SafetyClass:    "filesystem-write",
		NonOverridable: true,
	},
	DefaultToolMemoryRead: {
		Name:           DefaultToolMemoryRead,
		Version:        "v1",
		InputSchema:    `{"type":"object","properties":{"memory_path":{"type":"string","minLength":1},"offset":{"type":"integer","minimum":0},"length":{"type":"integer","minimum":0}},"required":["memory_path"],"additionalProperties":true}`,
		OutputSchema:   `{"type":"object","properties":{"memory_path":{"type":"string"},"content":{"type":"string"},"bytes_read":{"type":"integer","minimum":0}},"required":["content"],"additionalProperties":true}`,
		SafetyClass:    "memory-read",
		NonOverridable: true,
	},
	DefaultToolMemoryWrite: {
		Name:           DefaultToolMemoryWrite,
		Version:        "v1",
		InputSchema:    `{"type":"object","properties":{"memory_path":{"type":"string","minLength":1},"content":{"type":"string"},"append":{"type":"boolean"},"bytes_length":{"type":"integer","minimum":0}},"required":["memory_path","content"],"additionalProperties":true}`,
		OutputSchema:   `{"type":"object","properties":{"memory_path":{"type":"string"},"bytes_written":{"type":"integer","minimum":0}},"required":["bytes_written"],"additionalProperties":true}`,
		SafetyClass:    "memory-write",
		NonOverridable: true,
	},
	DefaultToolSpawnSubagent: {
		Name:           DefaultToolSpawnSubagent,
		Version:        "v1",
		InputSchema:    `{"type":"object","properties":{"agent_name":{"type":"string","minLength":1},"messages":{"type":"array"},"fanout":{"type":"integer","minimum":1},"timeout_ms":{"type":"integer","minimum":1}},"required":["agent_name"],"additionalProperties":true}`,
		OutputSchema:   `{"type":"object","properties":{"status":{"type":"string"},"outcome":{"type":"object"},"events":{"type":"array"}},"required":["status"],"additionalProperties":true}`,
		SafetyClass:    "delegation",
		NonOverridable: true,
	},
}

type DefaultToolRegistry interface {
	ListDefaultDefinitions(ctx context.Context) ([]ToolDefinition, error)
	GetDefaultDefinition(ctx context.Context, name string) (ToolDefinition, error)
	IsImmutable(name string) bool
}

func CanonicalDefaultToolNames() []string {
	names := make([]string, len(canonicalDefaultToolNames))
	copy(names, canonicalDefaultToolNames)
	return names
}

func CanonicalDefaultToolDefinitions() []ToolDefinition {
	defs := make([]ToolDefinition, 0, len(canonicalDefaultToolNames))
	for _, name := range canonicalDefaultToolNames {
		defs = append(defs, canonicalDefaultToolDefinitions[name])
	}
	return defs
}

func CanonicalDefaultToolDefinition(name string) (ToolDefinition, bool) {
	def, ok := canonicalDefaultToolDefinitions[name]
	return def, ok
}

func IsImmutableDefaultTool(name string) bool {
	def, ok := canonicalDefaultToolDefinitions[name]
	return ok && def.NonOverridable
}
