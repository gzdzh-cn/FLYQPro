package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"flyqpro/internal/chat"
)

func TestSharedRootCountsIncludesNestedEntries(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "documents", "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		filepath.Join(root, "top.txt"),
		filepath.Join(root, "documents", "readme.txt"),
		filepath.Join(root, "documents", "nested", "data.bin"),
	} {
		if err := os.WriteFile(name, []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	files, folders := sharedRootCounts(root)
	if files != 3 || folders != 2 {
		t.Fatalf("expected 3 files and 2 folders, got %d files and %d folders", files, folders)
	}
}

func TestSharedRootCountsContextStopsWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	files, folders := sharedRootCountsContext(ctx, t.TempDir())
	if files != 0 || folders != 0 {
		t.Fatalf("expected canceled scan to return no counts, got %d files and %d folders", files, folders)
	}
}

func TestSharedRootCountsCanExcludeHiddenEntries(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".private"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		filepath.Join(root, ".hidden.txt"),
		filepath.Join(root, ".private", "nested.txt"),
		filepath.Join(root, "visible.txt"),
	} {
		if err := os.WriteFile(name, []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	files, folders := sharedRootCountsProgressContext(context.Background(), root, nil, false)
	if files != 1 || folders != 0 {
		t.Fatalf("expected hidden entries excluded, got %d files and %d folders", files, folders)
	}
	files, folders = sharedRootCountsProgressContext(context.Background(), root, nil, true)
	if files != 3 || folders != 1 {
		t.Fatalf("expected hidden entries included, got %d files and %d folders", files, folders)
	}
}

func TestSharedStatsLatestGenerationWins(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	if err := os.WriteFile(filepath.Join(second, "latest.txt"), []byte("latest"), 0o600); err != nil {
		t.Fatal(err)
	}

	service := &ChatService{engine: chat.NewEngine()}
	service.startSharedStats(first, true)
	service.startSharedStats(second, true)
	defer service.clearSharedStats()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		service.sharedStatsMu.Lock()
		stats := service.sharedStats
		service.sharedStatsMu.Unlock()
		if stats.root == filepath.Clean(second) && stats.ready {
			if stats.fileCount != 1 || stats.folderCount != 0 {
				t.Fatalf("unexpected latest stats: %#v", stats)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for latest shared stats")
}

func TestSharedStatusReturnsWhileStatsLoadInBackground(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	service := &ChatService{engine: chat.NewEngine()}
	service.startSharedStats(root, true)
	defer service.clearSharedStats()

	status := service.sharedStatus(chat.Profile{SharedEnabled: true, SharedRootPath: root}, root)
	if !status.StatsLoading && !status.StatsReady {
		t.Fatalf("expected stats to be loading or already ready, got %#v", status)
	}
}
