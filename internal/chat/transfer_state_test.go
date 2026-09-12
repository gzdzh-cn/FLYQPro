package chat

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"testing"
)

func TestCanonicalTransferState(t *testing.T) {
	tests := map[string]TransferState{
		"pending":                 TransferQueued,
		"queued":                  TransferQueued,
		"awaiting_acceptance":     TransferQueued,
		"transferring":            TransferActive,
		"receiving":               TransferActive,
		"paused":                  TransferPausedLocal,
		"paused_peer":             TransferPausedPeer,
		"paused_network_unstable": TransferPausedNetwork,
		"completed":               TransferCompleted,
		"canceled":                TransferCancelled,
		"failed":                  TransferFailed,
		"future_state":            TransferUnknown,
	}
	for phase, want := range tests {
		if got := canonicalTransferState(phase); got != want {
			t.Fatalf("phase %q: got %q, want %q", phase, got, want)
		}
	}
}

func TestClassifyTransferError(t *testing.T) {
	tests := map[string]TransferErrorCode{
		"CERTIFICATE_CHANGED":  ErrCertificateChanged,
		"DEVICE_KEY_CHANGED":   ErrDeviceKeyChanged,
		"DEVICE_NOT_TRUSTED":   ErrDeviceNotTrusted,
		"FRIENDSHIP_REQUIRED":  ErrFriendshipRequired,
		"SESSION_NOT_READY":    ErrSessionNotReady,
		"CHECKSUM_MISMATCH":    ErrChunkVerifyFailed,
		"SOURCE_CHANGED":       ErrSourceFileChanged,
		"INSUFFICIENT_STORAGE": ErrInsufficientStorage,
	}
	for reason, want := range tests {
		if got := classifyTransferError(reason); got != want {
			t.Fatalf("reason %q: got %q, want %q", reason, got, want)
		}
	}
}

func TestPeerPoolSummaryReportsSlots(t *testing.T) {
	pool := NewPeerPool("peer-1", 2)
	if got := pool.Summary(); got.SlotCount != 2 || got.ActiveSlots != 0 || got.DeadSlots != 0 || got.SessionID == "" {
		t.Fatalf("unexpected initial pool summary: %+v", got)
	}
	slot, err := pool.Acquire(context.Background(), "task")
	if err != nil {
		t.Fatal(err)
	}
	if got := pool.Summary(); got.ActiveSlots != 1 {
		t.Fatalf("active slot not reported: %+v", got)
	}
	pool.Release(slot)
}

func TestTLSIdentityErrorsAreStructured(t *testing.T) {
	err := verifyPeerCertificateState(tls.ConnectionState{}, Peer{})
	typed, ok := err.(*TransferError)
	if !ok || typed.Code != ErrDeviceNotTrusted || typed.Retryable {
		t.Fatalf("unexpected TLS identity error: %#v", err)
	}
	identity, certificate := parallelTestIdentity(t)
	leaf, parseErr := x509.ParseCertificate(certificate.Certificate[0])
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	pinned := pinnedTestPeer(identity, certificate)
	pinned.CertificateFingerprint = sha256Hex([]byte("old-certificate"))
	err = verifyPeerCertificateState(tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf}}, pinned)
	typed, ok = err.(*TransferError)
	if !ok || typed.Code != ErrCertificateChanged || typed.Error() != string(ErrCertificateChanged) {
		t.Fatalf("certificate change was not classified safely: %#v", err)
	}
}

func TestClientTLSConfigRequiresPinnedPeerIdentity(t *testing.T) {
	engine := NewEngine()
	identity, certificate := parallelTestIdentity(t)
	engine.identity = identity
	complete := pinnedTestPeer(identity, certificate)
	missing := []Peer{
		{PublicKeyPEM: complete.PublicKeyPEM, CertificateFingerprint: complete.CertificateFingerprint},
		{DeviceID: complete.DeviceID, CertificateFingerprint: complete.CertificateFingerprint},
		{DeviceID: complete.DeviceID, PublicKeyPEM: complete.PublicKeyPEM},
	}
	for _, peer := range missing {
		if _, err := engine.clientTLSConfig(peer); classifyTransferError(errorString(err)) != ErrDeviceNotTrusted {
			t.Fatalf("TLS config with incomplete peer identity was accepted: peer=%+v err=%v", peer, err)
		}
	}
	if _, err := engine.clientTLSConfig(complete); err != nil {
		t.Fatalf("complete TLS identity rejected: %v", err)
	}
}

func TestInboundDataPeerAuthenticatesBeforeBusinessFrame(t *testing.T) {
	identity, certificate := parallelTestIdentity(t)
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	publicKeyDER, err := x509.MarshalPKIXPublicKey(leaf.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	peer := Peer{DeviceID: identity.DeviceID, Relation: PeerRelation, PublicKeyPEM: string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyDER})), CertificateFingerprint: sha256Hex(leaf.Raw)}
	engine := NewEngine()
	engine.peers[peer.DeviceID] = peer
	authenticated, err := engine.authenticateInboundDataPeer(tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf}})
	if err != nil || authenticated.DeviceID != peer.DeviceID {
		t.Fatalf("trusted inbound data peer rejected: peer=%+v err=%v", authenticated, err)
	}
	peer.CertificateFingerprint = sha256Hex([]byte("old-certificate"))
	engine.peers[peer.DeviceID] = peer
	if _, err := engine.authenticateInboundDataPeer(tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf}}); classifyTransferError(errorString(err)) != ErrCertificateChanged {
		t.Fatalf("changed certificate was not rejected: %v", err)
	}
}

func TestTransferProgressV1GoldenContract(t *testing.T) {
	data, err := os.ReadFile("../../protocol/transfer-progress-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var event map[string]any
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"eventVersion", "transferId", "messageId", "peerDeviceId", "sessionId", "generation", "state", "phase", "direction", "transferred", "total", "durableBytes", "retries", "errorCode", "retryable", "transport", "updatedAt"} {
		if _, ok := event[key]; !ok {
			t.Fatalf("golden transfer event missing %q", key)
		}
	}
	if event["eventVersion"] != float64(1) || event["state"] != string(TransferActive) {
		t.Fatalf("unexpected golden contract: %+v", event)
	}
}
