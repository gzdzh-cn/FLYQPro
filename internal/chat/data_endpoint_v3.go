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

type tlsTCPV3DataTransport struct {
	hosts  []string
	port   int
	config *tls.Config
}

func newTLSTCPV3DataTransport(hosts []string, port int, config *tls.Config) V3DataTransport {
	return &tlsTCPV3DataTransport{hosts: append([]string(nil), hosts...), port: port, config: config.Clone()}
}

func (*tlsTCPV3DataTransport) Kind() V3TransportKind { return TransportTLSTCP }
func (t *tlsTCPV3DataTransport) OpenStream(ctx context.Context) (net.Conn, error) {
	return DialV3DataCandidates(ctx, t.hosts, t.port, t.config)
}
func (*tlsTCPV3DataTransport) Close() error { return nil }

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
	dialer := tls.Dialer{NetDialer: &net.Dialer{Timeout: 5 * time.Second}, Config: cfg}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// DialV3DataCandidates races the addresses advertised by a peer and keeps the
// first successful TLS handshake. This handles peers with simultaneous WiFi
// and Ethernet interfaces without relying on discovery order.
func DialV3DataCandidates(ctx context.Context, hosts []string, port int, config *tls.Config) (net.Conn, error) {
	conn, _, err := DialV3DataCandidatesPreferred(ctx, hosts, port, "", config)
	return conn, err
}

// DialV3DataCandidatesPreferred gives the last known-good address a short head
// start, then races every fallback. A stale cache therefore costs at most the
// fallback delay rather than a complete dial timeout.
func DialV3DataCandidatesPreferred(ctx context.Context, hosts []string, port int, preferred string, config *tls.Config) (net.Conn, string, error) {
	if len(hosts) == 0 || port <= 0 {
		return nil, "", errors.New("no v3 data candidates")
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
		return nil, "", errors.New("no v3 data candidates")
	}
	if preferred != "" {
		for i, host := range unique {
			if host == preferred {
				copy(unique[1:i+1], unique[0:i])
				unique[0] = host
				break
			}
		}
	}
	probeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	type result struct {
		conn net.Conn
		host string
		err  error
	}
	results := make(chan result, len(unique))
	for index, host := range unique {
		index := index
		host := host
		go func() {
			if preferred != "" && index > 0 {
				timer := time.NewTimer(50 * time.Millisecond)
				defer timer.Stop()
				select {
				case <-probeCtx.Done():
					return
				case <-timer.C:
				}
			}
			conn, err := DialV3Data(probeCtx, net.JoinHostPort(host, strconv.Itoa(port)), config)
			select {
			case results <- result{conn: conn, host: host, err: err}:
			case <-probeCtx.Done():
				if conn != nil {
					_ = conn.Close()
				}
			}
		}()
	}
	var lastErr error
	for range unique {
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case item := <-results:
			if item.err == nil && item.conn != nil {
				cancel()
				return item.conn, item.host, nil
			}
			lastErr = item.err
		}
	}
	if lastErr == nil {
		lastErr = errors.New("all v3 data candidates failed")
	}
	return nil, "", lastErr
}
