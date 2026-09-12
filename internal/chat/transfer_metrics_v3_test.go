package chat

import "testing"

func TestTransferMetricsSnapshotRoundTrip(t *testing.T) {
	want := TransferMetricsSnapshotV1{MetricSeq: 3, CheckpointSeq: 9, DurableBytes: 4096, Speed: 1234.5, AverageSpeed: 1200, PeakSpeed: 2000, DiskWriteMs: 7, AckLatencyMs: 11, ChunkSize: 512 * 1024, WindowSize: 8, WindowBytes: 4 * 1024 * 1024, AckTargetBytes: 4 * 1024 * 1024, StreamCount: 2, ActiveStreams: 2}
	payload, err := encodeTransferMetricsSnapshot(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeTransferMetricsSnapshot(payload)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("snapshot mismatch: got %+v want %+v", got, want)
	}
}

func TestTransferMetricsSnapshotRejectsInvalid(t *testing.T) {
	if _, err := decodeTransferMetricsSnapshot([]byte(`{"metricSeq":0,"checkpointSeq":1,"durableBytes":1}`)); err == nil {
		t.Fatal("zero metric sequence accepted")
	}
	if _, err := decodeTransferMetricsSnapshot([]byte(`{"metricSeq":1,"checkpointSeq":1,"durableBytes":-1}`)); err == nil {
		t.Fatal("negative durable bytes accepted")
	}
}
