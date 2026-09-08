package chat

import "testing"

func TestTuneV3Profiles(t *testing.T) {
	if got := TuneV3(LinkProfile{Type: LinkEthernet, SpeedMbps: 1000}, 1<<30); got.Streams != 4 || got.FrameBytes != 1<<20 {
		t.Fatalf("gigabit tuning: %+v", got)
	}
	if got := TuneV3(LinkProfile{Type: LinkEthernet, SpeedMbps: 100}, 1<<30); got.Streams != 2 {
		t.Fatalf("fast ethernet tuning: %+v", got)
	}
	if got := TuneV3(LinkProfile{Type: LinkWiFi}, 1<<30); got.FrameBytes != 512<<10 {
		t.Fatalf("wifi tuning: %+v", got)
	}
	if got := TuneV3(LinkProfile{}, 1<<20); got.Streams != 1 {
		t.Fatalf("small file tuning: %+v", got)
	}
}

func TestV3FrameBinaryLayout(t *testing.T) {
	f := V3Frame{TransferID: "a", StreamID: 2, Offset: 3, Length: 4}
	b := f.MarshalBinary()
	if len(b) != 4+1+4+8+4+32 {
		t.Fatalf("unexpected frame size %d", len(b))
	}
}
