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
