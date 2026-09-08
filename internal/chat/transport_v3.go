package chat

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
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

type V3Frame struct {
	TransferID string
	StreamID   uint32
	Offset     uint64
	Length     uint32
	Hash       [32]byte
}

func (f V3Frame) MarshalBinary() []byte {
	b := make([]byte, 4+len(f.TransferID)+4+8+4+32)
	binary.BigEndian.PutUint32(b, uint32(len(f.TransferID)))
	n := 4
	copy(b[n:], f.TransferID)
	n += len(f.TransferID)
	binary.BigEndian.PutUint32(b[n:], f.StreamID)
	n += 4
	binary.BigEndian.PutUint64(b[n:], f.Offset)
	n += 8
	binary.BigEndian.PutUint32(b[n:], f.Length)
	n += 4
	copy(b[n:], f.Hash[:])
	return b
}

type QUICTransport struct{ conn *quic.Conn }

func ListenQUIC(addr string, config *tls.Config) (net.PacketConn, *quic.Listener, error) {
	if config == nil {
		return nil, nil, errors.New("nil TLS config")
	}
	config = config.Clone()
	config.MinVersion = tls.VersionTLS13
	config.NextProtos = []string{"dzhgo/3"}
	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		return nil, nil, err
	}
	l, err := quic.Listen(pc, config, &quic.Config{Allow0RTT: true, MaxIncomingStreams: 64, MaxIncomingUniStreams: 16})
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
	config.NextProtos = []string{"dzhgo/3"}
	c, err := quic.DialAddr(ctx, addr, config, &quic.Config{EnableDatagrams: true, Allow0RTT: true, MaxIncomingStreams: 64})
	if err != nil {
		return nil, err
	}
	return &QUICTransport{conn: c}, nil
}

func (t *QUICTransport) OpenStream(ctx context.Context) (*quic.Stream, error) {
	if t == nil || t.conn == nil {
		return nil, ErrTransportClosed
	}
	return t.conn.OpenStreamSync(ctx)
}
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

func WriteFrame(w io.Writer, f V3Frame, payload []byte) error {
	h := f.MarshalBinary()
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(h)+len(payload)))
	if _, e := w.Write(n[:]); e != nil {
		return e
	}
	if _, e := w.Write(h); e != nil {
		return e
	}
	_, e := w.Write(payload)
	return e
}
