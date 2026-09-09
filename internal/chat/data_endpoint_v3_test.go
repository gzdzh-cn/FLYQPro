package chat

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"testing"
	"time"
)

func TestV3DataEndpointRequiresTLSConfig(t *testing.T) {
	if _, e := ListenV3Data("127.0.0.1:0", nil); e == nil {
		t.Fatal("nil config accepted")
	}
	if _, e := DialV3Data(context.Background(), "127.0.0.1:1", nil); e == nil {
		t.Fatal("nil config accepted")
	}
	_ = tls.VersionTLS13
}

func TestV3DialCancellationInterruptsTLSHandshake(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err == nil {
			defer conn.Close()
			_, _ = io.Copy(io.Discard, conn)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	if conn, err := DialV3Data(ctx, ln.Addr().String(), &tls.Config{InsecureSkipVerify: true}); err == nil {
		conn.Close()
		t.Fatal("stalled handshake succeeded")
	}
	if time.Since(started) > time.Second {
		t.Fatal("dial ignored context cancellation")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("canceled connection leaked")
	}
}

func TestV3CandidateFailureDoesNotHideReachableAddress(t *testing.T) {
	_, cert := parallelTestIdentity(t)
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err == nil {
			defer conn.Close()
			_, _ = io.Copy(io.Discard, conn)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, err := DialV3DataCandidates(ctx, []string{"127.0.0.2", "127.0.0.1", "127.0.0.1"}, ln.Addr().(*net.TCPAddr).Port, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	<-done
}
