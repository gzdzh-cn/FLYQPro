package chat

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"time"

	"github.com/quic-go/quic-go"
)

// v3 is intentionally a clean break: file bytes never travel through the
// JSON/TCP control protocol.

var ErrTransportClosed = errors.New("dzhgo v3 transport closed")

type LinkType string

const (
	LinkUnknown  LinkType = "unknown"
	LinkWiFi     LinkType = "wifi"
	LinkEthernet LinkType = "ethernet"
)

type LinkProfile struct {
	Type      LinkType
	SpeedMbps int
	RTT       time.Duration
}

type TransferTuningV3 struct{ FrameBytes, WindowBytes, Streams int }

func TuneV3(p LinkProfile, size int64) TransferTuningV3 {
	if size < 8<<20 {
		return TransferTuningV3{256 << 10, 4 << 20, 1}
	}
	if p.Type == LinkEthernet && p.SpeedMbps >= 1000 {
		return TransferTuningV3{1 << 20, 32 << 20, 4}
	}
	if p.Type == LinkEthernet {
		return TransferTuningV3{256 << 10, 4 << 20, 2}
	}
	return TransferTuningV3{512 << 10, 16 << 20, 2}
}

type QUICTransport struct{ conn *quic.Conn }

// V3DataTransport exposes one bidirectional byte stream to the existing v3
// state machine. TLS/TCP and QUIC must both enter through this boundary so ACK,
// checkpoint and hash semantics cannot diverge by transport.
type V3DataTransport interface {
	Kind() V3TransportKind
	OpenStream(context.Context) (net.Conn, error)
	Close() error
}

type V3TransportKind string

const (
	TransportTLSTCP V3TransportKind = "tls-tcp"
	TransportQUIC   V3TransportKind = "quic"
)

type TransportABResult struct {
	Kind           V3TransportKind
	TotalTime      time.Duration
	FirstByte      time.Duration
	Recovered      bool
	SHA256Verified bool
	// Samples/Successes are populated by the experiment runner. A QUIC
	// candidate is never promoted from a single lucky run.
	Samples   int
	Successes int
}

// SelectV3Transport keeps TCP/TLS as the conservative default. QUIC becomes
// eligible only when the same scheduler run proves a 5% total-time gain
// without increasing first-byte latency or weakening recovery/integrity.
func SelectV3Transport(tcp, quic TransportABResult) V3TransportKind {
	if !tcp.SHA256Verified || !quic.SHA256Verified || !tcp.Recovered || !quic.Recovered || tcp.TotalTime <= 0 || quic.TotalTime <= 0 {
		return TransportTLSTCP
	}
	if quic.Samples < 5 || quic.Successes != quic.Samples || tcp.Samples < 5 || tcp.Successes != tcp.Samples {
		return TransportTLSTCP
	}
	if quic.FirstByte > tcp.FirstByte*105/100 || quic.TotalTime > tcp.TotalTime*95/100 {
		return TransportTLSTCP
	}
	return TransportQUIC
}

func ListenQUIC(addr string, config *tls.Config) (net.PacketConn, *quic.Listener, error) {
	if config == nil {
		return nil, nil, errors.New("nil TLS config")
	}
	config = config.Clone()
	config.MinVersion = tls.VersionTLS13
	config.NextProtos = []string{v3DataALPN}
	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		return nil, nil, err
	}
	// File mutation frames are not replay-safe. Keep 0-RTT disabled until the
	// application handshake can prove idempotency for every resumed request.
	l, err := quic.Listen(pc, config, &quic.Config{Allow0RTT: false, MaxIncomingStreams: 64, MaxIncomingUniStreams: 16})
	if err != nil {
		_ = pc.Close()
		return nil, nil, err
	}
	return pc, l, nil
}

func DialQUIC(ctx context.Context, addr string, config *tls.Config) (*QUICTransport, error) {
	if config == nil {
		return nil, errors.New("nil TLS config")
	}
	config = config.Clone()
	config.MinVersion = tls.VersionTLS13
	config.NextProtos = []string{v3DataALPN}
	c, err := quic.DialAddr(ctx, addr, config, &quic.Config{EnableDatagrams: true, Allow0RTT: false, MaxIncomingStreams: 64})
	if err != nil {
		return nil, err
	}
	return &QUICTransport{conn: c}, nil
}

func (t *QUICTransport) openQUICStream(ctx context.Context) (*quic.Stream, error) {
	if t == nil || t.conn == nil {
		return nil, ErrTransportClosed
	}
	return t.conn.OpenStreamSync(ctx)
}
func (t *QUICTransport) Kind() V3TransportKind { return TransportQUIC }
func (t *QUICTransport) OpenStream(ctx context.Context) (net.Conn, error) {
	stream, err := t.openQUICStream(ctx)
	if err != nil {
		return nil, err
	}
	return &quicV3StreamConn{Stream: stream, conn: t.conn}, nil
}
func (t *QUICTransport) Close() error { return t.CloseWithError(0, "") }

type quicV3StreamConn struct {
	*quic.Stream
	conn *quic.Conn
}

func (c *quicV3StreamConn) LocalAddr() net.Addr  { return c.conn.LocalAddr() }
func (c *quicV3StreamConn) RemoteAddr() net.Addr { return c.conn.RemoteAddr() }
func (t *QUICTransport) AcceptStream(ctx context.Context) (*quic.Stream, error) {
	if t == nil || t.conn == nil {
		return nil, ErrTransportClosed
	}
	return t.conn.AcceptStream(ctx)
}
func (t *QUICTransport) CloseWithError(code quic.ApplicationErrorCode, msg string) error {
	if t == nil || t.conn == nil {
		return nil
	}
	return t.conn.CloseWithError(code, msg)
}
