package sys

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestCombineAddonInsertStatements(t *testing.T) {
	statements := []string{
		"INSERT INTO addons_demo (id, name) VALUES ('1', 'first')",
		"INSERT INTO addons_demo (id, name) VALUES ('2', 'value contains VALUES')",
		"DELETE FROM addons_dict_info WHERE addonsName = 'demo'",
	}

	combined := combineAddonInsertStatements(statements)
	if len(combined) != 2 {
		t.Fatalf("expected 2 statements, got %d: %#v", len(combined), combined)
	}
	want := "INSERT INTO addons_demo (id, name) VALUES ('1', 'first'), ('2', 'value contains VALUES')"
	if combined[0] != want {
		t.Fatalf("unexpected merged insert:\nwant: %s\n got: %s", want, combined[0])
	}
	if combined[1] != statements[2] {
		t.Fatalf("non-insert statement must remain unchanged: %s", combined[1])
	}
}

func TestCombineAddonInsertStatementsRespectsRowLimit(t *testing.T) {
	statements := make([]string, 0, addonInsertBatchRows+1)
	for index := 0; index < addonInsertBatchRows+1; index++ {
		statements = append(statements, fmt.Sprintf("INSERT INTO addons_demo (id) VALUES (%d)", index))
	}

	combined := combineAddonInsertStatements(statements)
	if len(combined) != 2 {
		t.Fatalf("expected 2 bounded batches, got %d", len(combined))
	}
	if count := addonInsertValuesCount(splitInsertValuesForTest(t, combined[0])); count != addonInsertBatchRows {
		t.Fatalf("expected first batch to contain %d rows, got %d", addonInsertBatchRows, count)
	}
	if count := addonInsertValuesCount(splitInsertValuesForTest(t, combined[1])); count != 1 {
		t.Fatalf("expected second batch to contain 1 row, got %d", count)
	}
}

func TestAcquireAddonTaskLockCanBeCancelledWhileWaiting(t *testing.T) {
	name := "cancel-lock-test"
	release, err := acquireAddonTaskLock(context.Background(), name)
	if err != nil {
		t.Fatalf("acquire first lock: %v", err)
	}
	defer release()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, waitErr := acquireAddonTaskLock(ctx, name)
		result <- waitErr
	}()

	cancel()
	select {
	case waitErr := <-result:
		if !errors.Is(waitErr, context.Canceled) {
			t.Fatalf("expected context cancellation, got %v", waitErr)
		}
	case <-time.After(time.Second):
		t.Fatal("waiting lock did not respond to cancellation")
	}
}

func TestSplitAddonSQLContextStopsBeforeParsing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := splitAddonSQLContext(ctx, "INSERT INTO addons_demo (id) VALUES (1);")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestStreamAddonSQLStatementsPreservesQuotedSemicolons(t *testing.T) {
	content := "INSERT INTO addons_demo (name) VALUES ('a;b');\n" +
		"INSERT INTO addons_demo (name) VALUES ('中文');"
	reader := bufio.NewReader(strings.NewReader(content))
	var statements []string
	err := streamAddonSQLStatements(context.Background(), reader, func(statement string) error {
		statements = append(statements, statement)
		return nil
	})
	if err != nil {
		t.Fatalf("stream parser failed: %v", err)
	}
	if len(statements) != 2 {
		t.Fatalf("expected 2 statements, got %d: %#v", len(statements), statements)
	}
	if statements[0] != "INSERT INTO addons_demo (name) VALUES ('a;b')" {
		t.Fatalf("quoted semicolon was split incorrectly: %s", statements[0])
	}
	if statements[1] != "INSERT INTO addons_demo (name) VALUES ('中文')" {
		t.Fatalf("utf-8 statement was changed: %s", statements[1])
	}
}

func TestStreamAddonSQLStatementsCanBeCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := streamAddonSQLStatements(ctx, bufio.NewReader(strings.NewReader("INSERT INTO addons_demo VALUES (1);")), func(string) error {
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func splitInsertValuesForTest(t *testing.T, statement string) string {
	t.Helper()
	_, values, ok := splitAddonInsertStatement(statement)
	if !ok {
		t.Fatalf("expected INSERT statement: %s", statement)
	}
	return values
}
