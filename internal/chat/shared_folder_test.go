package chat

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"flyqpro/internal/service/db"
)

func openSharedFolderTestDatabase(t *testing.T) context.Context {
	t.Helper()
	root := t.TempDir()
	t.Setenv("GOFLY_DB_PATH", filepath.Join(root, "chat.db"))
	t.Setenv("FLYQPRO_DATA_DIR", filepath.Join(root, "data"))
	ctx := context.Background()
	if err := db.Open(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close(ctx) })
	if err := EnsureDefaults(ctx, filepath.Join(root, "attachments")); err != nil {
		t.Fatal(err)
	}
	return ctx
}

func TestSharedFolderMigrationIsIdempotent(t *testing.T) {
	ctx := openSharedFolderTestDatabase(t)
	legacyRoot := t.TempDir()
	expectedRoot, err := ValidateSharedRoot(legacyRoot)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := GetProfile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	profile.SharedRootPath = legacyRoot
	if err := SaveProfile(ctx, profile); err != nil {
		t.Fatal(err)
	}

	if _, err := GetProfile(ctx); err != nil {
		t.Fatal(err)
	}
	folders, err := ListSharedFolders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 1 || folders[0].RootPath != expectedRoot || folders[0].Name != filepath.Base(expectedRoot) {
		t.Fatalf("unexpected migrated folders: %#v", folders)
	}
	if _, err := GetProfile(ctx); err != nil {
		t.Fatal(err)
	}
	folders, err = ListSharedFolders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 1 || folders[0].ID == "" {
		t.Fatalf("migration was not idempotent: %#v", folders)
	}

	if err := RemoveSharedFolder(ctx, folders[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := GetProfile(ctx); err != nil {
		t.Fatal(err)
	}
	folders, err = ListSharedFolders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 0 {
		t.Fatalf("removed legacy folder was recreated: %#v", folders)
	}
	if _, err := os.Stat(legacyRoot); err != nil {
		t.Fatalf("removing configuration deleted the source directory: %v", err)
	}
}

func TestSharedFolderOperationsRejectDuplicatesAndKeepSources(t *testing.T) {
	ctx := openSharedFolderTestDatabase(t)
	firstRoot := t.TempDir()
	secondRoot := t.TempDir()

	first, err := AddSharedFolder(ctx, firstRoot)
	if err != nil {
		t.Fatal(err)
	}
	second, err := AddSharedFolder(ctx, secondRoot)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID {
		t.Fatal("shared folder IDs must be unique")
	}
	if _, err := AddSharedFolder(ctx, firstRoot); err == nil {
		t.Fatal("expected duplicate shared folder path to be rejected")
	}

	if err := RemoveSharedFolder(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(firstRoot); err != nil {
		t.Fatalf("removing configuration deleted the first source directory: %v", err)
	}
	if err := RemoveSharedFolder(ctx, first.ID); err == nil {
		t.Fatal("expected removing an unknown shared folder to fail")
	}
	folders, err := ListSharedFolders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 1 || folders[0].ID != second.ID {
		t.Fatalf("unexpected remaining folders: %#v", folders)
	}
}
