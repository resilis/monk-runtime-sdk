package runtime

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/resilis/monk-runtime-sdk/contracts"
)

type ToolPolicyEvaluator struct{}

func NewToolPolicyEvaluator() ToolPolicyEvaluator {
	return ToolPolicyEvaluator{}
}

func (e ToolPolicyEvaluator) Merge(base contracts.ToolPolicy, consumer contracts.ToolPolicy) contracts.ToolPolicy {
	merged := contracts.MergeToolPolicy(base, consumer)
	merged.AllowedPathPrefixes = mergeAllowedPathPrefixes(base.AllowedPathPrefixes, consumer.AllowedPathPrefixes)
	return merged
}

func (e ToolPolicyEvaluator) Evaluate(policy contracts.ToolPolicy, tool string, args map[string]any) error {
	return contracts.EvaluateToolPolicy(policy, tool, args)
}

func DefaultToolPolicy() contracts.ToolPolicy {
	return contracts.ToolPolicy{
		AllowTools: []string{
			contracts.DefaultToolTerminal,
			contracts.DefaultToolFileRead,
			contracts.DefaultToolFileWrite,
			contracts.DefaultToolMemoryRead,
			contracts.DefaultToolMemoryWrite,
			contracts.DefaultToolSpawnSubagent,
		},
		AllowedPathPrefixes: nil,
		AllowedCommands:     nil,
		MaxReadBytes:        1 << 20,
		MaxWriteBytes:       1 << 20,
		MaxSubagentFanout:   1,
	}
}

var _ contracts.PolicyEvaluator = ToolPolicyEvaluator{}

func mergeAllowedPathPrefixes(base []string, consumer []string) []string {
	basePrefixes := normalizePathPrefixes(base)
	consumerPrefixes := normalizePathPrefixes(consumer)

	if len(basePrefixes) == 0 {
		return consumerPrefixes
	}
	if len(consumerPrefixes) == 0 {
		return basePrefixes
	}

	merged := make(map[string]struct{})
	for _, basePrefix := range basePrefixes {
		for _, consumerPrefix := range consumerPrefixes {
			if pathPrefixContains(basePrefix, consumerPrefix) {
				merged[consumerPrefix] = struct{}{}
			}
			if pathPrefixContains(consumerPrefix, basePrefix) {
				merged[basePrefix] = struct{}{}
			}
		}
	}

	return sortedPathPrefixSet(merged)
}

func normalizePathPrefixes(prefixes []string) []string {
	normalized := make(map[string]struct{}, len(prefixes))
	for _, prefix := range prefixes {
		clean := filepath.Clean(strings.TrimSpace(prefix))
		if clean == "" || clean == "." {
			continue
		}
		normalized[clean] = struct{}{}
	}

	return sortedPathPrefixSet(normalized)
}

func sortedPathPrefixSet(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for prefix := range set {
		out = append(out, prefix)
	}
	sort.Strings(out)
	return out
}

func pathPrefixContains(prefix string, path string) bool {
	cleanPrefix := filepath.Clean(prefix)
	cleanPath := filepath.Clean(path)

	rel, err := filepath.Rel(cleanPrefix, cleanPath)
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
