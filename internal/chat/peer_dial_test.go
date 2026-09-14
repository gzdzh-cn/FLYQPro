package chat

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"testing"
	"time"
)

func TestPeerDialHostsDeduplicatesAdvertisedAddresses(t *testing.T) {
	peer := Peer{IP: " 192.0.2.10 ", LocalAddresses: []string{"192.0.2.10", "", "192.0.2.11", "192.0.2.11"}}
	hosts := peerDialHosts(peer)
	if len(hosts) != 2 || hosts[0] != "192.0.2.10" || hosts[1] != "192.0.2.11" {
		t.Fatalf("unexpected dial hosts: %#v", hosts)
	}
}

func TestPeerDialUsesReachableLocalAddress(t *testing.T) {
	_, certificate := parallelTestIdentity(t)
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS13})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	serverDone := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverDone <- acceptErr
			return
		}
		defer conn.Close()
		serverDone <- conn.(*tls.Conn).Handshake()
	}()

	peer := Peer{
		IP:             "127.0.0.2",
		LocalAddresses: []string{"127.0.0.1", "127.0.0.1"},
		Port:           listener.Addr().(*net.TCPAddr).Port,
	}
	conn, err := dialPeerTLS(context.Background(), peer, &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS13}, time.Second)
	if err != nil {
		t.Fatalf("reachable advertised address was hidden by stale primary: %v", err)
	}
	_ = conn.Close()
	if err := <-serverDone; err != nil {
		t.Fatalf("server handshake failed: %v", err)
	}
}

func TestPeerDialCancellationInterruptsTLSHandshake(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			defer conn.Close()
			_, _ = io.Copy(io.Discard, conn)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	peer := Peer{IP: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port}
	started := time.Now()
	if conn, err := dialPeerTLS(ctx, peer, &tls.Config{InsecureSkipVerify: true}, 5*time.Second); err == nil {
		_ = conn.Close()
		t.Fatal("stalled TLS handshake succeeded")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("dial ignored context cancellation: %s", elapsed)
	}
	select {
	case <-serverDone:
	case <-time.After(time.Second):
		t.Fatal("canceled control connection leaked")
	}
}
