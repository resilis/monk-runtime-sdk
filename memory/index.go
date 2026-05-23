package memory

import (
	"fmt"
	"sort"

	"github.com/resilis/monk-runtime-sdk/contracts"
)

func ValidateIndex(index contracts.MemoryIndex) error {
	if index.Version == "" {
		return contracts.NewRuntimeError(contracts.ErrMemoryIndexCode, "memory index version is required", false)
	}
	if index.FilePath == "" {
		return contracts.NewRuntimeError(contracts.ErrMemoryIndexCode, "memory index file_path is required", false)
	}
	for i, segment := range index.Segments {
		if segment.SegmentID == "" {
			return contracts.NewRuntimeError(contracts.ErrMemoryIndexCode, fmt.Sprintf("segment[%d] segment_id is required", i), false)
		}
		if segment.Offset < 0 {
			return contracts.NewRuntimeError(contracts.ErrMemoryIndexCode, fmt.Sprintf("segment[%d] offset must be >= 0", i), false)
		}
		if segment.Length <= 0 {
			return contracts.NewRuntimeError(contracts.ErrMemoryIndexCode, fmt.Sprintf("segment[%d] length must be > 0", i), false)
		}
	}
	return nil
}

func FindSegment(index contracts.MemoryIndex, segmentID string) (contracts.IndexSegment, bool) {
	for _, segment := range index.Segments {
		if segment.SegmentID == segmentID {
			return segment, true
		}
	}
	return contracts.IndexSegment{}, false
}

func SortedSegments(index contracts.MemoryIndex) []contracts.IndexSegment {
	segments := make([]contracts.IndexSegment, len(index.Segments))
	copy(segments, index.Segments)
	sort.SliceStable(segments, func(i int, j int) bool {
		if segments[i].Offset == segments[j].Offset {
			return segments[i].SegmentID < segments[j].SegmentID
		}
		return segments[i].Offset < segments[j].Offset
	})
	return segments
}
