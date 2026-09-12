package chat

import (
	"encoding/json"
	"fmt"
)

// TransferMetricsSnapshotV1 is carried on the final ACK of a durable batch.
// Keeping the snapshot in the binary frame payload avoids a second control
// channel and gives both sides one authoritative receive-side measurement.
type TransferMetricsSnapshotV1 struct {
	MetricSeq      uint64  `json:"metricSeq"`
	CheckpointSeq  uint64  `json:"checkpointSeq"`
	DurableBytes   int64   `json:"durableBytes"`
	Speed          float64 `json:"speed"`
	AverageSpeed   float64 `json:"averageSpeed"`
	PeakSpeed      float64 `json:"peakSpeed"`
	DiskWriteMs    int64   `json:"diskWriteMs"`
	AckLatencyMs   int64   `json:"ackLatencyMs"`
	ChunkSize      int     `json:"chunkSize"`
	WindowSize     int     `json:"windowSize"`
	WindowBytes    int64   `json:"windowBytes"`
	AckTargetBytes int64   `json:"ackTargetBytes"`
	StreamCount    int     `json:"streamCount"`
	ActiveStreams  int     `json:"activeStreams"`
}

func encodeTransferMetricsSnapshot(snapshot TransferMetricsSnapshotV1) ([]byte, error) {
	return json.Marshal(snapshot)
}

func decodeTransferMetricsSnapshot(payload []byte) (TransferMetricsSnapshotV1, error) {
	var snapshot TransferMetricsSnapshotV1
	if len(payload) == 0 {
		return snapshot, fmt.Errorf("empty transfer metrics snapshot")
	}
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return snapshot, fmt.Errorf("invalid transfer metrics snapshot: %w", err)
	}
	if snapshot.MetricSeq == 0 || snapshot.CheckpointSeq == 0 || snapshot.DurableBytes < 0 {
		return snapshot, fmt.Errorf("invalid transfer metrics values")
	}
	return snapshot, nil
}
