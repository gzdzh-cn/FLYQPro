package chat

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSharedEntriesPageReturnsBoundedResult(t *testing.T) {
	root := t.TempDir()
	for index := 0; index < 205; index++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("file-%03d.txt", index)), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	first, err := ListSharedEntriesPage(root, "", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Entries) != 100 || !first.HasMore || first.NextOffset != 100 {
		t.Fatalf("unexpected first page: %#v", first)
	}
	second, err := ListSharedEntriesPage(root, "", first.NextOffset, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Entries) != 100 || !second.HasMore || second.NextOffset != 200 {
		t.Fatalf("unexpected second page: %#v", second)
	}
	last, err := ListSharedEntriesPage(root, "", second.NextOffset, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(last.Entries) != 5 || last.HasMore {
		t.Fatalf("unexpected final page: %#v", last)
	}
}

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

func TestListSharedEntriesHonorsHiddenFileSetting(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".secret"), []byte("hidden"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "visible"), []byte("visible"), 0o600); err != nil {
		t.Fatal(err)
	}

	visible, err := ListSharedEntries(root, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(visible) != 1 || visible[0].Name != "visible" {
		t.Fatalf("expected only visible entry, got %#v", visible)
	}

	all, err := ListSharedEntries(root, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("expected hidden entry when enabled, got %#v", all)
	}
}

func TestSharedDirectoryDetailsIncludeDescendantFileSizes(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "readme.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "nested", "data.bin"), []byte("world!!"), 0o600); err != nil {
		t.Fatal(err)
	}
	entry, _, err := GetSharedEntry(root, "docs", true)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Size != int64(len("hello")+len("world!!")) {
		t.Fatalf("expected descendant size 12, got %d", entry.Size)
	}
}
