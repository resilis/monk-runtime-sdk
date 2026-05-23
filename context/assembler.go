package context

import (
	stdctx "context"

	"github.com/resilis/monk-runtime-sdk/contracts"
	sdkmemory "github.com/resilis/monk-runtime-sdk/memory"
)

type AssembleInput struct {
	AgentID    string
	MemoryPath string
	SegmentIDs []string
	Policy     contracts.MemoryPolicy
}

type AssembleOutput struct {
	Blocks     []contracts.ContextBlock
	TokenCount int
}

type Assembler struct {
	store contracts.MemoryStore
}

func NewAssembler(store contracts.MemoryStore) *Assembler {
	return &Assembler{store: store}
}

func (a *Assembler) Assemble(ctx stdctx.Context, input AssembleInput) (AssembleOutput, error) {
	retriever := sdkmemory.NewRetriever(a.store)
	segments, err := retriever.ReadSegments(ctx, input.AgentID, input.MemoryPath, input.SegmentIDs)
	if err != nil {
		return AssembleOutput{}, err
	}

	maxFiles := input.Policy.MaxFiles
	if maxFiles <= 0 {
		maxFiles = len(segments)
	}

	maxTokens := input.Policy.MaxContextTokens
	if maxTokens <= 0 {
		maxTokens = int(^uint(0) >> 1)
	}

	output := AssembleOutput{Blocks: make([]contracts.ContextBlock, 0, len(segments))}
	for _, segment := range segments {
		if len(output.Blocks) >= maxFiles {
			break
		}
		if output.TokenCount+segment.Segment.Tokens > maxTokens {
			break
		}
		output.Blocks = append(output.Blocks, contracts.ContextBlock{
			SegmentID: segment.Segment.SegmentID,
			Content:   segment.Content,
			Tokens:    segment.Segment.Tokens,
		})
		output.TokenCount += segment.Segment.Tokens
	}

	return output, nil
}
