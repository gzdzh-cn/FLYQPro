package chat

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSharedPathRejectsEscape(t *testing.T) {
	root := t.TempDir()
	for _, relative := range []string{"../outside.txt", "/tmp/outside.txt", `..\outside.txt`} {
		if _, _, err := resolveSharedPath(root, relative, false); err == nil {
			t.Fatalf("expected path %q to be rejected", relative)
		}
	}
}

func TestSharedFolderOperationsStayWithinRoot(t *testing.T) {
	root := t.TempDir()
	if _, err := CreateSharedFolder(root, "", "docs"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "note.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := ListSharedEntries(root, "docs")
	if err != nil || len(entries) != 1 || entries[0].Name != "note.txt" {
		t.Fatalf("unexpected entries: %#v, %v", entries, err)
	}
	if err := RenameSharedEntry(root, "docs/note.txt", "renamed.txt"); err != nil {
		t.Fatal(err)
	}
	if err := CopySharedEntry(root, "docs/renamed.txt", ""); err != nil {
		t.Fatal(err)
	}
	if err := DeleteSharedEntry(root, "docs"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs")); !os.IsNotExist(err) {
		t.Fatalf("expected docs to be deleted, got %v", err)
	}
}
