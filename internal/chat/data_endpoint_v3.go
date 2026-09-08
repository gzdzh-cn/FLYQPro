package chat

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"strconv"
	"time"
)

const v3DataALPN = "dzhgo/3-data"

type V3DataEndpoint struct {
	Listener net.Listener
	Addr     string
}

func ListenV3Data(addr string, config *tls.Config) (*V3DataEndpoint, error) {
	if config == nil {
		return nil, errors.New("nil TLS config")
	}
	cfg := config.Clone()
	cfg.MinVersion = tls.VersionTLS13
	cfg.NextProtos = []string{v3DataALPN}
	ln, err := tls.Listen("tcp", addr, cfg)
	if err != nil {
		return nil, err
	}
	return &V3DataEndpoint{Listener: ln, Addr: ln.Addr().String()}, nil
}
func DialV3Data(ctx context.Context, addr string, config *tls.Config) (net.Conn, error) {
	if config == nil {
		return nil, errors.New("nil TLS config")
	}
	cfg := config.Clone()
	cfg.MinVersion = tls.VersionTLS13
	cfg.NextProtos = []string{v3DataALPN}
	d := net.Dialer{}
	conn, err := tls.DialWithDialer(&d, "tcp", addr, cfg)
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if err := conn.HandshakeContext(ctx); err != nil {
		conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

// DialV3DataCandidates races the addresses advertised by a peer and keeps the
// first successful TLS handshake. This handles peers with simultaneous WiFi
// and Ethernet interfaces without relying on discovery order.
func DialV3DataCandidates(ctx context.Context, hosts []string, port int, config *tls.Config) (net.Conn, error) {
	if len(hosts) == 0 || port <= 0 {
		return nil, errors.New("no v3 data candidates")
	}
	unique := make([]string, 0, len(hosts))
	seen := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		if host == "" {
			continue
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		unique = append(unique, host)
	}
	if len(unique) == 0 {
		return nil, errors.New("no v3 data candidates")
	}
	probeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	type result struct {
		conn net.Conn
		err  error
	}
	results := make(chan result, len(unique))
	for _, host := range unique {
		host := host
		go func() {
			conn, err := DialV3Data(probeCtx, net.JoinHostPort(host, strconv.Itoa(port)), config)
			results <- result{conn: conn, err: err}
		}()
	}
	var lastErr error
	for range unique {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case item := <-results:
			if item.err == nil && item.conn != nil {
				cancel()
				return item.conn, nil
			}
			lastErr = item.err
		}
	}
	if lastErr == nil {
		lastErr = errors.New("all v3 data candidates failed")
	}
	return nil, lastErr
}
