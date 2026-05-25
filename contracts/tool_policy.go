package contracts

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"strings"
)

type ToolPolicy struct {
	AllowTools          []string
	DenyTools           []string
	AllowedPathPrefixes []string
	DeniedPathPrefixes  []string
	AllowedCommands     []string
	DeniedCommands      []string
	MaxReadBytes        int64
	MaxWriteBytes       int64
	MaxSubagentFanout   int
}

type ToolBackendCapabilities struct {
	CanExecuteTerminal bool
	CanReadFiles       bool
	CanWriteFiles      bool
	CanReadMemory      bool
	CanWriteMemory     bool
	CanSpawnSubagent   bool
}

type ToolBackend interface {
	Capabilities() ToolBackendCapabilities
	Execute(ctx context.Context, tool string, args map[string]any) (ToolResult, error)
}

type PolicyEvaluator interface {
	Merge(base ToolPolicy, consumer ToolPolicy) ToolPolicy
	Evaluate(policy ToolPolicy, tool string, args map[string]any) error
}

func MergeToolPolicy(base ToolPolicy, consumer ToolPolicy) ToolPolicy {
	allowTools, hasAllow := mergeAllowList(base.AllowTools, consumer.AllowTools)
	allowPaths, _ := mergeAllowList(base.AllowedPathPrefixes, consumer.AllowedPathPrefixes)
	allowCmds, _ := mergeAllowList(base.AllowedCommands, consumer.AllowedCommands)

	merged := ToolPolicy{
		AllowTools:          allowTools,
		DenyTools:           mergeDenyList(base.DenyTools, consumer.DenyTools),
		AllowedPathPrefixes: allowPaths,
		DeniedPathPrefixes:  mergeDenyList(base.DeniedPathPrefixes, consumer.DeniedPathPrefixes),
		AllowedCommands:     allowCmds,
		DeniedCommands:      mergeDenyList(base.DeniedCommands, consumer.DeniedCommands),
		MaxReadBytes:        mergeMaxInt64(base.MaxReadBytes, consumer.MaxReadBytes),
		MaxWriteBytes:       mergeMaxInt64(base.MaxWriteBytes, consumer.MaxWriteBytes),
		MaxSubagentFanout:   mergeMaxInt(base.MaxSubagentFanout, consumer.MaxSubagentFanout),
	}

	if hasAllow {
		merged.AllowTools = filterDenied(merged.AllowTools, merged.DenyTools)
	}
	merged.AllowedPathPrefixes = filterDenied(merged.AllowedPathPrefixes, merged.DeniedPathPrefixes)
	merged.AllowedCommands = filterDenied(merged.AllowedCommands, merged.DeniedCommands)

	return merged
}

func EvaluateToolPolicy(policy ToolPolicy, tool string, args map[string]any) error {
	toolName := strings.TrimSpace(tool)
	if toolName == "" {
		return NewRuntimeError(ErrPolicyDeniedCode, "tool name is required", false)
	}

	if deniedByName(policy.DenyTools, toolName) {
		return NewRuntimeError(ErrPolicyDeniedCode, fmt.Sprintf("tool %q denied by policy", toolName), false)
	}
	if !allowedByName(policy.AllowTools, toolName) {
		return NewRuntimeError(ErrPolicyDeniedCode, fmt.Sprintf("tool %q is not explicitly allowed", toolName), false)
	}

	if path, ok := readStringArg(args, "path"); ok {
		clean := filepath.Clean(path)
		if deniedByPathPrefix(policy.DeniedPathPrefixes, clean) {
			return NewRuntimeError(ErrPolicyDeniedCode, fmt.Sprintf("path %q denied by policy", clean), false)
		}
		if !allowedByPathPrefix(policy.AllowedPathPrefixes, clean) {
			return NewRuntimeError(ErrPolicyDeniedCode, fmt.Sprintf("path %q not in allowed prefixes", clean), false)
		}
	}

	if memoryPath, ok := readStringArg(args, "memory_path"); ok {
		clean := filepath.Clean(memoryPath)
		if deniedByPathPrefix(policy.DeniedPathPrefixes, clean) {
			return NewRuntimeError(ErrPolicyDeniedCode, fmt.Sprintf("memory path %q denied by policy", clean), false)
		}
		if !allowedByPathPrefix(policy.AllowedPathPrefixes, clean) {
			return NewRuntimeError(ErrPolicyDeniedCode, fmt.Sprintf("memory path %q not in allowed prefixes", clean), false)
		}
	}

	if command, ok := readStringArg(args, "command"); ok {
		trimmed := strings.TrimSpace(command)
		if deniedByPrefix(policy.DeniedCommands, trimmed) {
			return NewRuntimeError(ErrPolicyDeniedCode, fmt.Sprintf("command %q denied by policy", trimmed), false)
		}
		if !allowedByPrefix(policy.AllowedCommands, trimmed) {
			return NewRuntimeError(ErrPolicyDeniedCode, fmt.Sprintf("command %q not in allowed commands", trimmed), false)
		}
	}

	if hasArg(args, "length") {
		length, ok := readInt64Arg(args, "length")
		if !ok || length < 0 {
			return NewRuntimeError(ErrPolicyDeniedCode, "length must be a non-negative integer", false)
		}
		if policy.MaxReadBytes > 0 && length > policy.MaxReadBytes {
			return NewRuntimeError(ErrPolicyDeniedCode, fmt.Sprintf("read length %d exceeds max_read_bytes %d", length, policy.MaxReadBytes), false)
		}
	}

	if hasArg(args, "bytes_length") {
		length, ok := readInt64Arg(args, "bytes_length")
		if !ok || length < 0 {
			return NewRuntimeError(ErrPolicyDeniedCode, "bytes_length must be a non-negative integer", false)
		}
		if policy.MaxWriteBytes > 0 && length > policy.MaxWriteBytes {
			return NewRuntimeError(ErrPolicyDeniedCode, fmt.Sprintf("write length %d exceeds max_write_bytes %d", length, policy.MaxWriteBytes), false)
		}
	}

	if hasArg(args, "fanout") {
		fanout, ok := readIntArg(args, "fanout")
		if !ok || fanout < 0 {
			return NewRuntimeError(ErrPolicyDeniedCode, "fanout must be a non-negative integer", false)
		}
		if policy.MaxSubagentFanout > 0 && fanout > policy.MaxSubagentFanout {
			return NewRuntimeError(ErrPolicyDeniedCode, fmt.Sprintf("fanout %d exceeds max_subagent_fanout %d", fanout, policy.MaxSubagentFanout), false)
		}
	}

	return nil
}

func mergeAllowList(base []string, consumer []string) ([]string, bool) {
	baseSet := normalizeSet(base)
	consumerSet := normalizeSet(consumer)
	baseHas := len(baseSet) > 0
	consumerHas := len(consumerSet) > 0

	switch {
	case baseHas && consumerHas:
		intersection := make([]string, 0, len(baseSet))
		for key := range baseSet {
			if _, ok := consumerSet[key]; ok {
				intersection = append(intersection, key)
			}
		}
		return stableFromSet(intersection), true
	case baseHas:
		return stableFromSet(mapKeys(baseSet)), true
	case consumerHas:
		return stableFromSet(mapKeys(consumerSet)), true
	default:
		return nil, false
	}
}

func mergeDenyList(base []string, consumer []string) []string {
	set := normalizeSet(base)
	for key := range normalizeSet(consumer) {
		set[key] = struct{}{}
	}
	return stableFromSet(mapKeys(set))
}

func mergeMaxInt64(base int64, consumer int64) int64 {
	if base > 0 && consumer > 0 {
		if consumer < base {
			return consumer
		}
		return base
	}
	if base > 0 {
		return base
	}
	if consumer > 0 {
		return consumer
	}
	return 0
}

func mergeMaxInt(base int, consumer int) int {
	if base > 0 && consumer > 0 {
		if consumer < base {
			return consumer
		}
		return base
	}
	if base > 0 {
		return base
	}
	if consumer > 0 {
		return consumer
	}
	return 0
}

func filterDenied(allowed []string, denied []string) []string {
	if len(allowed) == 0 {
		return nil
	}
	deniedSet := normalizeSet(denied)
	filtered := make([]string, 0, len(allowed))
	for _, value := range allowed {
		if _, blocked := deniedSet[strings.ToLower(strings.TrimSpace(value))]; blocked {
			continue
		}
		filtered = append(filtered, value)
	}
	return filtered
}

func normalizeSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}
	return set
}

func mapKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	return keys
}

func stableFromSet(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	if len(out) < 2 {
		return out
	}
	for i := 0; i < len(out)-1; i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func deniedByName(deny []string, value string) bool {
	needle := strings.ToLower(strings.TrimSpace(value))
	for _, denyValue := range deny {
		if strings.ToLower(strings.TrimSpace(denyValue)) == needle {
			return true
		}
	}
	return false
}

func allowedByName(allow []string, value string) bool {
	needle := strings.ToLower(strings.TrimSpace(value))
	for _, allowValue := range allow {
		if strings.ToLower(strings.TrimSpace(allowValue)) == needle {
			return true
		}
	}
	return false
}

func deniedByPrefix(deny []string, value string) bool {
	needle := strings.ToLower(strings.TrimSpace(value))
	for _, denyPrefix := range deny {
		prefix := strings.ToLower(strings.TrimSpace(denyPrefix))
		if prefix == "" {
			continue
		}
		if strings.HasPrefix(needle, prefix) {
			return true
		}
	}
	return false
}

func deniedByPathPrefix(deny []string, value string) bool {
	needle := filepath.Clean(strings.TrimSpace(value))
	for _, denyPrefix := range deny {
		prefix := filepath.Clean(strings.TrimSpace(denyPrefix))
		if prefix == "" || prefix == "." {
			continue
		}
		if pathWithinPrefix(needle, prefix) {
			return true
		}
	}
	return false
}

func allowedByPathPrefix(allow []string, value string) bool {
	needle := filepath.Clean(strings.TrimSpace(value))
	for _, allowPrefix := range allow {
		prefix := filepath.Clean(strings.TrimSpace(allowPrefix))
		if prefix == "" || prefix == "." {
			continue
		}
		if pathWithinPrefix(needle, prefix) {
			return true
		}
	}
	return false
}

func pathWithinPrefix(value string, prefix string) bool {
	cleanValue := filepath.Clean(value)
	cleanPrefix := filepath.Clean(prefix)
	rel, err := filepath.Rel(cleanPrefix, cleanValue)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func allowedByPrefix(allow []string, value string) bool {
	needle := strings.ToLower(strings.TrimSpace(value))
	for _, allowPrefix := range allow {
		prefix := strings.ToLower(strings.TrimSpace(allowPrefix))
		if prefix == "" {
			continue
		}
		if strings.HasPrefix(needle, prefix) {
			return true
		}
	}
	return false
}

func readStringArg(args map[string]any, key string) (string, bool) {
	if args == nil {
		return "", false
	}
	raw, ok := args[key]
	if !ok {
		return "", false
	}
	value, ok := raw.(string)
	if !ok {
		return "", false
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}
	return trimmed, true
}

func readIntArg(args map[string]any, key string) (int, bool) {
	if args == nil {
		return 0, false
	}
	raw, ok := args[key]
	if !ok {
		return 0, false
	}
	switch value := raw.(type) {
	case int:
		return value, true
	case int64:
		if value < int64(minIntValue()) || value > int64(maxIntValue()) {
			return 0, false
		}
		return int(value), true
	case int32:
		return int(value), true
	case int16:
		return int(value), true
	case int8:
		return int(value), true
	case uint:
		if value > uint(maxIntValue()) {
			return 0, false
		}
		return int(value), true
	case uint64:
		if value > uint64(maxIntValue()) {
			return 0, false
		}
		return int(value), true
	case uint32:
		return int(value), true
	case uint16:
		return int(value), true
	case uint8:
		return int(value), true
	case float64:
		if !isIntegerFloat(value) {
			return 0, false
		}
		if value < float64(minIntValue()) || value > float64(maxIntValue()) {
			return 0, false
		}
		return int(value), true
	default:
		return 0, false
	}
}

func readInt64Arg(args map[string]any, key string) (int64, bool) {
	if args == nil {
		return 0, false
	}
	raw, ok := args[key]
	if !ok {
		return 0, false
	}
	switch value := raw.(type) {
	case int:
		return int64(value), true
	case int64:
		return value, true
	case int32:
		return int64(value), true
	case int16:
		return int64(value), true
	case int8:
		return int64(value), true
	case uint:
		if uint64(value) > math.MaxInt64 {
			return 0, false
		}
		return int64(value), true
	case uint64:
		if value > math.MaxInt64 {
			return 0, false
		}
		return int64(value), true
	case uint32:
		return int64(value), true
	case uint16:
		return int64(value), true
	case uint8:
		return int64(value), true
	case float64:
		if !isIntegerFloat(value) {
			return 0, false
		}
		if value < math.MinInt64 || value > math.MaxInt64 {
			return 0, false
		}
		return int64(value), true
	default:
		return 0, false
	}
}

func hasArg(args map[string]any, key string) bool {
	if args == nil {
		return false
	}
	_, ok := args[key]
	return ok
}

func isIntegerFloat(value float64) bool {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return false
	}
	return math.Trunc(value) == value
}

func maxIntValue() int {
	return int(^uint(0) >> 1)
}

func minIntValue() int {
	return -maxIntValue() - 1
}
