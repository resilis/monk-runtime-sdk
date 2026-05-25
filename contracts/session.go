package contracts

import (
	"context"
	"time"
)

type SessionKind string

const (
	SessionKindMain     SessionKind = "main"
	SessionKindSubagent SessionKind = "subagent"
	SessionKindSidecar  SessionKind = "sidecar"
)

type SessionSpec struct {
	RunID            string
	Kind             SessionKind
	AgentName        string
	WorkingDirectory string
	Model            string
	Prompt           string
	Timeout          time.Duration
	StrictToolMode   bool
	AllowedTools     []string
	ExcludedTools    []string
	SkillsPayload    map[string]any
	Metadata         map[string]any
}

type SessionInput struct {
	Prompt string
}

type SessionOutput struct {
	Content string
	Usage   UsageSummary
	Done    bool
}

type SessionHandle interface {
	ID() string
	SendAndWait(ctx context.Context, input SessionInput) (SessionOutput, error)
	Cancel(reason string) error
	Close() error
}

type SessionManager interface {
	CreateSession(ctx context.Context, spec SessionSpec) (SessionHandle, error)
}
