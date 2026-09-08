package chat

import (
	"context"
	"crypto/tls"
	"testing"
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
