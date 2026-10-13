package inbox_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GhandyP/06-inbox-prioritization/internal/inbox"
	"github.com/GhandyP/06-inbox-prioritization/internal/priority"
)

// The fixture adapter is the offline path: every test in this repository runs
// against it, so no test needs credentials, a token, or a network.

var (
	_ inbox.Reader = inbox.Fixture{}
	_ inbox.Reader = (*inbox.Fixture)(nil)
)

func testdata(name string) string {
	return filepath.Join("..", "..", "testdata", name)
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "messages.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp fixture: %v", err)
	}
	return path
}

func assertRule(t *testing.T, err error, want inbox.Rule) *inbox.Error {
	t.Helper()
	if err == nil {
		t.Fatalf("want a *inbox.Error with rule %q, got nil", want)
	}
	var fixtureErr *inbox.Error
	if !errors.As(err, &fixtureErr) {
		t.Fatalf("want a *inbox.Error, got %T: %v", err, err)
	}
	if fixtureErr.Rule != want {
		t.Fatalf("want rule %q, got %q (%v)", want, fixtureErr.Rule, err)
	}
	return fixtureErr
}

func TestFixtureReadsEveryFieldInOrder(t *testing.T) {
	messages, err := inbox.Fixture{Path: testdata("messages_ok.json")}.Messages(context.Background())
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if len(messages) != 5 {
		t.Fatalf("want 5 messages, got %d", len(messages))
	}

	first := messages[0]
	if first.ID != "m-001" {
		t.Errorf("id: want m-001, got %q", first.ID)
	}
	if first.Sender != "ana.ruiz@example.com" {
		t.Errorf("sender: got %q", first.Sender)
	}
	if first.Subject != "Invoice 8841 due Friday" {
		t.Errorf("subject: got %q", first.Subject)
	}
	if first.Body == "" {
		t.Error("body is empty")
	}
	want := time.Date(2026, time.October, 13, 9, 12, 0, 0, time.UTC)
	if !first.ReceivedAt.Equal(want) {
		t.Errorf("received_at: want %s, got %s", want, first.ReceivedAt)
	}

	ids := make([]string, 0, len(messages))
	for _, m := range messages {
		ids = append(ids, m.ID)
	}
	wantIDs := []string{"m-001", "m-002", "m-003", "m-004", "m-005"}
	for i := range wantIDs {
		if ids[i] != wantIDs[i] {
			t.Fatalf("file order not preserved: want %v, got %v", wantIDs, ids)
		}
	}
}

func TestFixtureReadsAnEmptyBatch(t *testing.T) {
	messages, err := inbox.Fixture{Path: testdata("messages_empty.json")}.Messages(context.Background())
	if err != nil {
		t.Fatalf("read empty fixture: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("want no messages, got %d", len(messages))
	}
}

func TestFixtureAcceptsMessagesWithoutMetadata(t *testing.T) {
	messages, err := inbox.Fixture{Path: testdata("messages_missing_metadata.json")}.Messages(context.Background())
	if err != nil {
		t.Fatalf("a message without a sender or subject is structurally valid: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("want 2 messages, got %d", len(messages))
	}
	if messages[0].Sender != "" || messages[0].Subject != "" {
		t.Errorf("want empty metadata preserved, got %+v", messages[0])
	}
}

func TestFixtureRejectsMalformedJSON(t *testing.T) {
	path := writeTemp(t, `{"messages": [`)
	_, err := inbox.Fixture{Path: path}.Messages(context.Background())
	assertRule(t, err, inbox.RuleMalformedJSON)
}

func TestFixtureRejectsAnUnknownField(t *testing.T) {
	path := writeTemp(t, `{"messages":[{"id":"m1","sender":"a@b.c","subject":"s","body":"b","recieved_at":"2026-10-13T09:00:00Z"}]}`)
	_, err := inbox.Fixture{Path: path}.Messages(context.Background())
	assertRule(t, err, inbox.RuleUnknownField)
}

func TestFixtureRejectsAMessageWithoutAnID(t *testing.T) {
	path := writeTemp(t, `{"messages":[{"id":"","sender":"a@b.c","subject":"s","body":"b","received_at":"2026-10-13T09:00:00Z"}]}`)
	_, err := inbox.Fixture{Path: path}.Messages(context.Background())
	if got := assertRule(t, err, inbox.RuleMissingID); got.Detail == "" {
		t.Error("the error does not say which message is at fault")
	}
}

func TestFixtureRejectsAMessageWithoutAReceivedAt(t *testing.T) {
	path := writeTemp(t, `{"messages":[{"id":"m1","sender":"a@b.c","subject":"s","body":"b"}]}`)
	_, err := inbox.Fixture{Path: path}.Messages(context.Background())
	if got := assertRule(t, err, inbox.RuleMissingTime); got.ID != "m1" {
		t.Errorf("want the offending id m1, got %q", got.ID)
	}
}

func TestFixtureRejectsADuplicateID(t *testing.T) {
	path := writeTemp(t, `{"messages":[
		{"id":"m1","sender":"a@b.c","subject":"s","body":"b","received_at":"2026-10-13T09:00:00Z"},
		{"id":"m1","sender":"d@e.f","subject":"t","body":"c","received_at":"2026-10-13T10:00:00Z"}
	]}`)
	_, err := inbox.Fixture{Path: path}.Messages(context.Background())
	if got := assertRule(t, err, inbox.RuleDuplicateID); got.ID != "m1" {
		t.Errorf("want the offending id m1, got %q", got.ID)
	}
}

func TestFixtureRejectsAMissingFile(t *testing.T) {
	_, err := inbox.Fixture{Path: filepath.Join(t.TempDir(), "absent.json")}.Messages(context.Background())
	assertRule(t, err, inbox.RuleUnreadable)
}

func TestErrorsNameTheFixturePath(t *testing.T) {
	path := writeTemp(t, `{"messages": [`)
	_, err := inbox.Fixture{Path: path}.Messages(context.Background())
	if got := err.Error(); !strings.Contains(got, filepath.Base(path)) {
		t.Fatalf("error does not name the file: %s", got)
	}
}

func TestFixtureHonoursContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := inbox.Fixture{Path: testdata("messages_ok.json")}.Messages(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestCommittedFixturesParse(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "messages_*.json"))
	if err != nil {
		t.Fatalf("glob testdata: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no committed fixtures found")
	}
	for _, path := range paths {
		if _, err := (inbox.Fixture{Path: path}).Messages(context.Background()); err != nil {
			t.Errorf("committed fixture %s does not parse: %v", filepath.Base(path), err)
		}
	}
}

func TestFixtureOutputFeedsTheContract(t *testing.T) {
	messages, err := inbox.Fixture{Path: testdata("messages_ok.json")}.Messages(context.Background())
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	// No results yet, so every message must be reported as unclassified. This
	// proves the two packages compose without a conversion layer.
	err = priority.Validate(messages, nil)
	if err == nil {
		t.Fatal("want a missing-result error for every message")
	}
	var batch *priority.BatchError
	if !errors.As(err, &batch) {
		t.Fatalf("want a *priority.BatchError, got %T: %v", err, err)
	}
	if len(batch.Violations) != len(messages) {
		t.Fatalf("want %d violations, got %d", len(messages), len(batch.Violations))
	}
}
