package chat

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"
)

func peerDialHosts(peer Peer) []string {
	hosts := make([]string, 0, 1+len(peer.LocalAddresses))
	seen := make(map[string]struct{}, 1+len(peer.LocalAddresses))
	for _, host := range append([]string{peer.IP}, peer.LocalAddresses...) {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}
		if _, exists := seen[host]; exists {
			continue
		}
		seen[host] = struct{}{}
		hosts = append(hosts, host)
	}
	return hosts
}

func peerHasDialAddress(peer Peer) bool {
	return peer.Port > 0 && len(peerDialHosts(peer)) > 0
}

// dialPeerTLS races all control-plane addresses advertised by discovery. A
// stale primary address must not hide a reachable WiFi or Ethernet address.
func dialPeerTLS(ctx context.Context, peer Peer, config *tls.Config, timeout time.Duration) (*tls.Conn, error) {
	if config == nil {
		return nil, errors.New("nil TLS config")
	}
	hosts := peerDialHosts(peer)
	if len(hosts) == 0 || peer.Port <= 0 {
		return nil, errors.New("好友地址不可用")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	dialCtx := ctx
	cancel := func() {}
	if timeout > 0 {
		dialCtx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		dialCtx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	type dialResult struct {
		conn *tls.Conn
		err  error
	}
	results := make(chan dialResult)
	for _, host := range hosts {
		host := host
		go func() {
			cfg := config.Clone()
			dialer := tls.Dialer{
				NetDialer: &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second},
				Config:    cfg,
			}
			raw, err := dialer.DialContext(dialCtx, "tcp", net.JoinHostPort(host, strconv.Itoa(peer.Port)))
			var conn *tls.Conn
			if raw != nil {
				conn, _ = raw.(*tls.Conn)
				if conn == nil {
					_ = raw.Close()
					err = errors.New("TLS 连接类型无效")
				}
			}
			select {
			case results <- dialResult{conn: conn, err: err}:
			case <-dialCtx.Done():
				if conn != nil {
					_ = conn.Close()
				}
			}
		}()
	}

	var lastErr error
	for range hosts {
		select {
		case <-dialCtx.Done():
			if lastErr != nil {
				return nil, lastErr
			}
			return nil, dialCtx.Err()
		case result := <-results:
			if result.err == nil && result.conn != nil {
				cancel()
				return result.conn, nil
			}
			lastErr = result.err
		}
	}
	if lastErr == nil {
		lastErr = errors.New("所有好友地址均不可用")
	}
	return nil, lastErr
}
