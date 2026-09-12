package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestReopenUsesConfiguredDatabasePath(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	t.Setenv("GOFLY_DB_PATH", filepath.Join(root, "first.db"))
	if err := Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer Close(ctx)
	if _, err := DB().Exec(ctx, "CREATE TABLE isolation_sentinel(id INTEGER)"); err != nil {
		t.Fatal(err)
	}
	if err := Close(ctx); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOFLY_DB_PATH", filepath.Join(root, "second.db"))
	if err := Open(ctx); err != nil {
		t.Fatal(err)
	}
	value, err := DB().GetValue(ctx, "SELECT count(*) FROM sqlite_master WHERE name='isolation_sentinel'")
	if err != nil {
		t.Fatal(err)
	}
	if value.Int() != 0 {
		t.Fatal("reopened old database instead of configured path")
	}
}

func TestTransferResumeMigrationStateIsDurable(t *testing.T) {
	ctx := context.Background()
	t.Setenv("GOFLY_DB_PATH", filepath.Join(t.TempDir(), "migration.db"))
	if err := Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer Close(ctx)
	if err := SetTransferResumeMigrationStatus(ctx, "migrating", "attachment-1", ""); err != nil {
		t.Fatal(err)
	}
	row, err := DB().GetOne(ctx, `SELECT status, cursor, last_error FROM transfer_resume_migrations WHERE id=1`)
	if err != nil {
		t.Fatal(err)
	}
	if row["status"].String() != "migrating" || row["cursor"].String() != "attachment-1" || row["last_error"].String() != "" {
		t.Fatalf("unexpected migration state: %+v", row)
	}
	version, err := DB().GetValue(ctx, `SELECT count(*) FROM schema_migrations WHERE version=5`)
	if err != nil || version.Int() != 1 {
		t.Fatalf("schema version 5 not recorded: value=%v err=%v", version, err)
	}
}
