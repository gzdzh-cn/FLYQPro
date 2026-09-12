package chat

import (
	"context"
	"crypto/tls"
	"net"
	"testing"
	"time"
)

var _ V3DataTransport = (*tlsTCPV3DataTransport)(nil)
var _ V3DataTransport = (*QUICTransport)(nil)

func TestSelectV3TransportRequiresFivePercentGainAndIntegrity(t *testing.T) {
	tcp := TransportABResult{Kind: TransportTLSTCP, TotalTime: 10 * time.Second, FirstByte: 100 * time.Millisecond, Recovered: true, SHA256Verified: true, Samples: 5, Successes: 5}
	if got := SelectV3Transport(tcp, TransportABResult{Kind: TransportQUIC, TotalTime: 9 * time.Second, FirstByte: 100 * time.Millisecond, Recovered: true, SHA256Verified: true, Samples: 5, Successes: 5}); got != TransportQUIC {
		t.Fatalf("expected QUIC after >=5%% gain, got %s", got)
	}
	if got := SelectV3Transport(tcp, TransportABResult{Kind: TransportQUIC, TotalTime: 9*time.Second + 600*time.Millisecond, FirstByte: 100 * time.Millisecond, Recovered: true, SHA256Verified: true, Samples: 5, Successes: 5}); got != TransportTLSTCP {
		t.Fatalf("expected TCP for sub-5%% gain, got %s", got)
	}
	if got := SelectV3Transport(tcp, TransportABResult{Kind: TransportQUIC, TotalTime: 8 * time.Second, FirstByte: 106 * time.Millisecond, Recovered: true, SHA256Verified: true, Samples: 5, Successes: 5}); got != TransportTLSTCP {
		t.Fatalf("expected TCP when first byte regresses over 5%%, got %s", got)
	}
	if got := SelectV3Transport(tcp, TransportABResult{Kind: TransportQUIC, TotalTime: 8 * time.Second, FirstByte: 100 * time.Millisecond, Recovered: true, SHA256Verified: true, Samples: 4, Successes: 4}); got != TransportTLSTCP {
		t.Fatalf("expected TCP until five successful samples exist, got %s", got)
	}
	if got := SelectV3Transport(tcp, TransportABResult{Kind: TransportQUIC, TotalTime: 8 * time.Second, FirstByte: 100 * time.Millisecond, Recovered: true, SHA256Verified: true, Samples: 5, Successes: 4}); got != TransportTLSTCP {
		t.Fatalf("expected TCP when one QUIC sample failed, got %s", got)
	}
}

func TestTLSTCPDataTransportUsesSharedStreamContract(t *testing.T) {
	_, certificate := parallelTestIdentity(t)
	listener, err := ListenV3Data("127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{certificate}})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Listener.Close()
	done := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Listener.Accept()
		if acceptErr == nil {
			defer conn.Close()
			buffer := make([]byte, 1)
			_, acceptErr = conn.Read(buffer)
		}
		done <- acceptErr
	}()
	port := listener.Listener.Addr().(*net.TCPAddr).Port
	transport := newTLSTCPV3DataTransport([]string{"127.0.0.1"}, port, &tls.Config{InsecureSkipVerify: true})
	if transport.Kind() != TransportTLSTCP {
		t.Fatalf("unexpected kind: %s", transport.Kind())
	}
	conn, err := transport.OpenStream(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
