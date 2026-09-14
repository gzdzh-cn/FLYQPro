package chat

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func parallelTestIdentity(t *testing.T) (Identity, tls.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	identity := Identity{DeviceInfo: DeviceInfo{
		DeviceID:               sha256Hex(publicDER),
		PublicKeyPEM:           string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})),
		CertificateFingerprint: sha256Hex(der),
	}, PrivateKeyPEM: string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})), CertificatePEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))}
	cert, err := identity.TLSCertificate()
	if err != nil {
		t.Fatal(err)
	}
	return identity, cert
}

func pinnedTestPeer(identity Identity, certificate tls.Certificate) Peer {
	return Peer{
		DeviceID:               identity.DeviceID,
		PublicKeyPEM:           identity.PublicKeyPEM,
		CertificateFingerprint: sha256Hex(certificate.Certificate[0]),
	}
}

// Exercise production v3 TLS workers, durable ranges and final binary confirmation.
func TestV3ParallelTLSCompletesAllRanges(t *testing.T) {
	runParallelTLS(t, 36*1024*1024+17)
}

func TestV3LargeTLS(t *testing.T) {
	value := os.Getenv("FLYQPRO_TRANSFER_TEST_BYTES")
	if value == "" {
		t.Skip("set FLYQPRO_TRANSFER_TEST_BYTES for a large TCP/TLS transfer")
	}
	size64, err := strconv.ParseInt(value, 10, 63)
	if err != nil || size64 < 4 || int64(int(size64)) != size64 {
		t.Fatal("invalid FLYQPRO_TRANSFER_TEST_BYTES")
	}
	runs := 5
	if rawRuns := os.Getenv("FLYQPRO_TRANSFER_RUNS"); rawRuns != "" {
		parsed, parseErr := strconv.Atoi(rawRuns)
		if parseErr != nil || parsed <= 0 {
			t.Fatalf("invalid FLYQPRO_TRANSFER_RUNS %q", rawRuns)
		}
		runs = parsed
	}
	sizes := make([]int, runs)
	for index := range sizes {
		sizes[index] = int(size64)
	}
	runV3TLS(t, sizes, 4, false)
}

func runParallelTLS(t *testing.T, size int) {
	t.Helper()
	runV3TLS(t, []int{size}, 4, false)
}

func TestParallelLaunchAlwaysSchedulesRemainingRanges(t *testing.T) {
	for _, tc := range []struct {
		launched, completed int
		bytes, diskMs       int64
		want                int
	}{
		{1, 0, 8 * 1024 * 1024, 0, 2},
		{2, 0, 32 * 1024 * 1024, 0, 4},
		{1, 0, 32 * 1024 * 1024, 500, 1},
		{1, 1, 9 * 1024 * 1024, 500, 2},
		{2, 2, 18 * 1024 * 1024, 500, 3},
		{3, 3, 27 * 1024 * 1024, 500, 4},
	} {
		if got := parallelLaunchTarget(tc.launched, tc.completed, 4, tc.bytes, tc.diskMs, 0); got != tc.want {
			t.Fatalf("%+v: got %d", tc, got)
		}
	}
	if got := parallelLaunchTarget(1, 0, 4, 32*1024*1024, 0, 9*time.Second); got != 1 {
		t.Fatalf("high ACK latency must not expand streams: got %d", got)
	}
}

func TestParallelAckRejectsInvalidConfirmation(t *testing.T) {
	for _, tc := range []struct {
		name          string
		bytes         int64
		status, token string
		valid         bool
	}{
		{"valid", 8, "receiving", "token", true},
		{"final", 16, "stream-complete", "token", true},
		{"duplicate", 4, "receiving", "token", false},
		{"reordered", 3, "receiving", "token", false},
		{"beyond_sent", 17, "receiving", "token", false},
		{"premature_final", 8, "stream-complete", "token", false},
		{"missing_final", 16, "receiving", "token", false},
		{"wrong_token", 8, "receiving", "wrong", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var data bytes.Buffer
			if err := writeWire(&data, wireMessage{Type: "file_stream_ack", AttachmentID: "attachment", StreamID: 1, TransferToken: tc.token, StreamBytes: tc.bytes, Status: tc.status}); err != nil {
				t.Fatal(err)
			}
			_, _, err := readParallelStreamAck(newWireReader(&data), Message{AttachmentID: "attachment"}, "token", 1, 16, 4, 16)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
		})
	}
}

func TestParallelJoinCancellationInterruptsHandshake(t *testing.T) {
	identity, cert := parallelTestIdentity(t)
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	helloRead, serverDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		var hello wireMessage
		if newWireReader(conn).Decode(&hello) != nil {
			return
		}
		close(helloRead)
		_, _ = io.Copy(io.Discard, conn)
	}()
	sender := NewEngine()
	sender.identity = identity
	peer := pinnedTestPeer(identity, cert)
	peer.IP = "127.0.0.1"
	peer.Port = listener.Addr().(*net.TCPAddr).Port
	result := make(chan error, 1)
	go func() {
		session, _, err := sender.openParallelDataStream(ctx, peer, protocolDialects[0], Message{AttachmentID: "attachment"}, "token", 0, 1, 0, 1024, parallelChunkSize)
		if session != nil {
			session.close()
		}
		result <- err
	}()
	select {
	case <-helloRead:
	case <-time.After(3 * time.Second):
		t.Fatal("TLS hello did not arrive")
	}
	cancel()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("canceled join succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not interrupt TLS join")
	}
	select {
	case <-serverDone:
	case <-time.After(time.Second):
		t.Fatal("canceled TLS connection remains open")
	}
}

func TestParallelFailureAckInterruptsBlockedWrite(t *testing.T) {
	identity, cert := parallelTestIdentity(t)
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		stop := context.AfterFunc(ctx, func() { conn.Close() })
		defer stop()
		reader := newWireReader(conn)
		var hello, join wireMessage
		if reader.Decode(&hello) != nil {
			return
		}
		if writeWire(conn, wireMessage{Type: "hello_ack", Capabilities: []string{fileParallelCapability}, FriendshipState: "friend"}) != nil {
			return
		}
		if reader.Decode(&join) != nil {
			return
		}
		if writeWire(conn, wireMessage{Type: "file_stream_join_ack", AttachmentID: join.AttachmentID, TransferToken: join.TransferToken, StreamID: join.StreamID, Status: "accepted"}) != nil {
			return
		}
		if _, err := readBinaryFileFrameHeader(reader.reader); err != nil {
			return
		}
		_ = writeWire(conn, wireMessage{Type: "file_stream_ack", AttachmentID: join.AttachmentID, TransferToken: join.TransferToken, StreamID: join.StreamID, Status: "failed", Reason: "INSUFFICIENT_STORAGE"})
		// Keep the connection open without consuming the remaining payload.
		<-ctx.Done()
	}()
	source, err := os.Create(filepath.Join(t.TempDir(), "source"))
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	const size = 64 * 1024 * 1024
	if err := source.Truncate(size); err != nil {
		t.Fatal(err)
	}
	sender := NewEngine()
	sender.identity = identity
	message := Message{AttachmentID: "attachment", AttachmentSize: size}
	sender.outgoing[message.AttachmentID] = &outgoingTransfer{session: newWireSession(nil)}
	peer := pinnedTestPeer(identity, cert)
	peer.IP = "127.0.0.1"
	peer.Port = listener.Addr().(*net.TCPAddr).Port
	updates := make(chan parallelStreamProgress, 32)
	done := make(chan struct{})
	go func() {
		defer close(done)
		sender.sendParallelStream(ctx, peer, protocolDialects[0], message, source, "token", 1, 0, 0, size, parallelChunkSize, updates)
	}()
	defer func() { cancel(); listener.Close(); <-done; <-serverDone }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("failure ACK did not interrupt the socket write")
	}
	close(updates)
	failed := false
	for update := range updates {
		failed = failed || update.err != nil
		if update.done || update.confirmed != 0 {
			t.Fatal("failed stream reported unconfirmed bytes as completed")
		}
	}
	if !failed {
		t.Fatal("failed stream did not report an error")
	}
}

type slowParallelReader struct {
	io.Reader
	delay time.Duration
}

func (r *slowParallelReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if n > 0 {
		time.Sleep(time.Duration(int64(r.delay) * int64(n) / (4 * 1024 * 1024)))
	}
	return n, err
}
