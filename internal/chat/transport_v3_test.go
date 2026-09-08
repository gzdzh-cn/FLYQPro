package chat

import (
	"testing"
	"time"
)

func TestSelectV3TransportRequiresFivePercentGainAndIntegrity(t *testing.T) {
	tcp := TransportABResult{Kind: TransportTLSTCP, TotalTime: 10 * time.Second, FirstByte: 100 * time.Millisecond, Recovered: true, SHA256Verified: true}
	if got := SelectV3Transport(tcp, TransportABResult{Kind: TransportQUIC, TotalTime: 9 * time.Second, FirstByte: 100 * time.Millisecond, Recovered: true, SHA256Verified: true}); got != TransportQUIC {
		t.Fatalf("expected QUIC after >=5%% gain, got %s", got)
	}
	if got := SelectV3Transport(tcp, TransportABResult{Kind: TransportQUIC, TotalTime: 9*time.Second + 600*time.Millisecond, FirstByte: 100 * time.Millisecond, Recovered: true, SHA256Verified: true}); got != TransportTLSTCP {
		t.Fatalf("expected TCP for sub-5%% gain, got %s", got)
	}
	if got := SelectV3Transport(tcp, TransportABResult{Kind: TransportQUIC, TotalTime: 8 * time.Second, FirstByte: 101 * time.Millisecond, Recovered: true, SHA256Verified: true}); got != TransportTLSTCP {
		t.Fatalf("expected TCP when first byte regresses, got %s", got)
	}
}
