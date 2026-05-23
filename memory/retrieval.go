package memory

import (
	"context"
	"sort"

	"github.com/resilis/monk-runtime-sdk/contracts"
)

type SegmentData struct {
	Segment contracts.IndexSegment
	Content []byte
}

type Retriever struct {
	store contracts.MemoryStore
}

func NewRetriever(store contracts.MemoryStore) *Retriever {
	return &Retriever{store: store}
}

func (r *Retriever) ReadSegment(ctx context.Context, agentID string, memoryPath string, segmentID string) (SegmentData, error) {
	if r.store == nil {
		return SegmentData{}, contracts.NewRuntimeError(
			contracts.ErrMemoryIndexCode,
			"memory store is required",
			false,
		)
	}

	index, err := r.store.LookupIndex(ctx, agentID, memoryPath)
	if err != nil {
		return SegmentData{}, contracts.WrapRuntimeError(
			contracts.ErrMemoryIndexCode,
			"failed to lookup memory index",
			false,
			err,
		)
	}
	if err := ValidateIndex(index); err != nil {
		return SegmentData{}, err
	}

	segment, ok := FindSegment(index, segmentID)
	if !ok {
		return SegmentData{}, contracts.NewRuntimeError(
			contracts.ErrMemoryIndexCode,
			"segment_id not found in index",
			false,
		)
	}

	content, err := r.store.ReadRange(ctx, memoryPath, segment.Offset, segment.Length)
	if err != nil {
		return SegmentData{}, contracts.WrapRuntimeError(
			contracts.ErrMemoryIndexCode,
			"failed to read indexed segment range",
			false,
			err,
		)
	}

	return SegmentData{Segment: segment, Content: content}, nil
}

func (r *Retriever) ReadSegments(ctx context.Context, agentID string, memoryPath string, segmentIDs []string) ([]SegmentData, error) {
	segments := make([]SegmentData, 0, len(segmentIDs))
	for _, segmentID := range segmentIDs {
		segment, err := r.ReadSegment(ctx, agentID, memoryPath, segmentID)
		if err != nil {
			return nil, err
		}
		segments = append(segments, segment)
	}

	sort.SliceStable(segments, func(i int, j int) bool {
		if segments[i].Segment.Offset == segments[j].Segment.Offset {
			return segments[i].Segment.SegmentID < segments[j].Segment.SegmentID
		}
		return segments[i].Segment.Offset < segments[j].Segment.Offset
	})

	return segments, nil
}
