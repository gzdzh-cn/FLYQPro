package chat

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestProductionTLSConfigurationIsCentralized prevents a future production
// dial path from silently reintroducing trust-all TLS. Test fixtures are
// intentionally excluded; they may use self-signed certificates locally.
func TestProductionTLSConfigurationIsCentralized(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to locate package source")
	}
	root := filepath.Dir(filename)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(root, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		contents := string(data)
		if !strings.Contains(contents, "InsecureSkipVerify") {
			continue
		}
		if entry.Name() != "engine.go" {
			t.Errorf("production TLS trust override found outside engine.go: %s", entry.Name())
			continue
		}
		if !strings.Contains(contents, "VerifyConnection") {
			t.Errorf("engine.go trust override is missing VerifyConnection")
		}
	}
}
