package contracts

import (
	"context"
	"time"
)

type Runtime interface {
	Execute(ctx context.Context, spec RunSpec) (RunOutcome, error)
}

type ProtocolEngine interface {
	ApplyModelEvent(ctx context.Context, ev ModelEvent) ([]RuntimeAction, error)
	ApplyToolResult(ctx context.Context, res ToolResult) ([]RuntimeAction, error)
	NextActions(ctx context.Context) ([]RuntimeAction, error)
	State() ProtocolState
}

type ProviderAdapter interface {
	StartTurn(ctx context.Context, req ProviderRequest) (ProviderTurn, error)
	ContinueTurn(ctx context.Context, turn ProviderTurn, toolResults []ToolResult) (ProviderTurn, error)
}

type ToolExecutor interface {
	ExecuteTool(ctx context.Context, in ToolCall) (ToolResult, error)
}

type PersistenceAdapter interface {
	OnRunStarted(ctx context.Context, runID string, meta RunMeta) error
	OnRuntimeEvent(ctx context.Context, runID string, ev RuntimeEvent) error
	OnRunFinished(ctx context.Context, runID string, out RunOutcome) error
}

type ControlPlaneReporter interface {
	ReportUsage(ctx context.Context, runID string, usage UsageSummary) error
}

type EventSink interface {
	Publish(ctx context.Context, ev RuntimeEvent) error
}

type MemoryStore interface {
	LookupIndex(ctx context.Context, agentID string, memoryPath string) (MemoryIndex, error)
	ReadRange(ctx context.Context, memoryPath string, offset int64, length int64) ([]byte, error)
}

type AgentDefinitionLoader interface {
	Load(ctx context.Context, agentName string) (AgentDefinition, error)
}

type SubagentRuntime interface {
	RunSubagent(ctx context.Context, req SubagentRequest) (SubagentResult, error)
}

type RunSpec struct {
	RunID                  string
	SessionID              string
	SessionKind            SessionKind
	OrgID                  string
	UserID                 string
	AgentName              string
	Messages               []Message
	StrictToolMode         bool
	MaxToolCalls           int
	MemoryPolicy           MemoryPolicy
	ToolPolicy             ToolPolicy
	SubagentMaxConcurrency int
	SkillsPayload          map[string]any
	SessionMetadata        map[string]any
}

type MemoryPolicy struct {
	MaxContextTokens int
	MaxFiles         int
	MaxBytesPerFile  int64
	RetrievalMode    string
	CompactionMode   string
}

type Message struct {
	Role    string
	Content string
}

type ProviderRequest struct {
	RunID          string
	AgentName      string
	Messages       []Message
	StrictToolMode bool
}

type ProviderTurn struct {
	ModelEvents []ModelEvent
	Done        bool
}

type ModelEvent struct {
	AssistantDelta string
	ToolCalls      []ToolCall
	Done           bool
	Usage          UsageSummary
}

type ToolCall struct {
	CallID string
	Tool   string
	Args   map[string]any
}

type ToolResult struct {
	CallID string
	Status string
	Result any
	Error  string
	Meta   map[string]any
}

type RuntimeAction struct {
	Kind       string
	ToolCall   *ToolCall
	ToolResult *ToolResult
	Event      *RuntimeEvent
}

type RuntimeEvent struct {
	RunID      string
	Type       string
	Seq        int64
	CallID     string
	Tool       string
	Payload    map[string]any
	Usage      UsageSummary
	OccurredAt time.Time
}

type ProtocolState string

const (
	ProtocolStateIdle      ProtocolState = "idle"
	ProtocolStateRunning   ProtocolState = "running"
	ProtocolStateCompleted ProtocolState = "completed"
	ProtocolStateFailed    ProtocolState = "failed"
)

type RunMeta struct {
	OrgID     string
	UserID    string
	AgentName string
}

type RunOutcome struct {
	RunID     string
	Status    string
	Usage     UsageSummary
	Completed bool
	ErrorCode string
	Message   string
}

type UsageSummary struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

type AgentDefinition struct {
	Name        string         `json:"name"`
	Model       string         `json:"model"`
	System      string         `json:"system"`
	Tools       []string       `json:"tools"`
	MCPServers  []string       `json:"mcp_servers"`
	MultiAgent  bool           `json:"multiagent"`
	Description string         `json:"description"`
	Metadata    map[string]any `json:"metadata"`
}

type MemoryIndex struct {
	Version  string         `json:"version"`
	FilePath string         `json:"file_path"`
	Segments []IndexSegment `json:"segments"`
	Checksum string         `json:"checksum"`
}

type IndexSegment struct {
	SegmentID string `json:"segment_id"`
	Offset    int64  `json:"offset"`
	Length    int64  `json:"length"`
	Tokens    int    `json:"tokens"`
}

type ContextBlock struct {
	SegmentID string
	Content   []byte
	Tokens    int
}

type SubagentRequest struct {
	ParentRunID string
	AgentName   string
	Messages    []Message
	Policy      MemoryPolicy
	ToolPolicy  ToolPolicy
	SessionKind SessionKind
	Context     map[string]any
}

type SubagentResult struct {
	Outcome RunOutcome
	Events  []RuntimeEvent
}
