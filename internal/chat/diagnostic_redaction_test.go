package chat

import (
	"fmt"
	"strings"
	"syscall"
	"testing"
)

func TestDiagnosticRedactionRemovesSecretsAndAbsolutePaths(t *testing.T) {
	raw := "open /Users/alice/Documents/private.txt: token=secret-value password: hunter2 Authorization: Bearer bearer-secret C:\\Users\\alice\\private.txt -----BEGIN PRIVATE KEY----- abc -----END PRIVATE KEY-----"
	redacted := redactDiagnosticText(raw)
	for _, forbidden := range []string{"/Users/alice", `C:\Users\alice`, "secret-value", "hunter2", "bearer-secret", "BEGIN PRIVATE KEY", " abc "} {
		if strings.Contains(redacted, forbidden) {
			t.Fatalf("diagnostic leaked %q: %s", forbidden, redacted)
		}
	}
	if !strings.Contains(redacted, "<path>") || !strings.Contains(redacted, "<redacted>") {
		t.Fatalf("diagnostic did not preserve safe placeholders: %s", redacted)
	}
}

func TestTransferErrorInfoRecognizesWrappedDiskFull(t *testing.T) {
	code, retryable := transferErrorInfo(fmt.Errorf("checkpoint: %w", syscall.ENOSPC))
	if code != ErrInsufficientStorage || !retryable {
		t.Fatalf("wrapped ENOSPC classified as code=%s retryable=%v", code, retryable)
	}
}
