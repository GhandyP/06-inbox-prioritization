package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/GhandyP/06-inbox-prioritization/internal/cli"
	"github.com/GhandyP/06-inbox-prioritization/internal/priority"
)

// The command is exercised through its public entry point only: Run takes the
// command line and two writers, and returns an exit code. Nothing here needs a
// model, a credential, or a network.

func at(hour int) time.Time {
	return time.Date(2026, time.October, 13, hour, 0, 0, 0, time.UTC)
}

func batch() []priority.Message {
	return []priority.Message{
		{ID: "m1", Sender: "ana@example.com", Subject: "Invoice due Friday", Body: "Please confirm before Friday.", ReceivedAt: at(9)},
		{ID: "m2", Sender: "news@example.com", Subject: "Weekly newsletter", Body: "Links you may enjoy.", ReceivedAt: at(10)},
		{ID: "m3", Sender: "ana@example.com", Subject: "Thanks", Body: "Much appreciated.", ReceivedAt: at(11)},
	}
}

func results() []priority.Result {
	return []priority.Result{
		{MessageID: "m1", Label: priority.Urgent, Reasons: []priority.Reason{{Kind: priority.KindDeadlineLanguage, Quote: "due Friday"}}},
		{MessageID: "m2", Label: priority.Ignore, Reasons: []priority.Reason{{Kind: priority.KindBulkMail, Quote: "Weekly newsletter"}}},
		{MessageID: "m3", Label: priority.Later, Reasons: []priority.Reason{{Kind: priority.KindNoActionNeeded, Quote: "Much appreciated."}}},
	}
}

type run struct {
	code   int
	stdout string
	stderr string
}

// deps returns collaborators that never touch the network, and records what the
// command asked for.
func deps(t *testing.T, opts *cli.Options, classified *[]priority.Message) cli.Deps {
	t.Helper()
	return cli.Deps{
		Read: func(_ context.Context, got cli.Options) ([]priority.Message, error) {
			if opts != nil {
				*opts = got
			}
			return batch(), nil
		},
		Classify: func(_ context.Context, _ cli.Options, messages []priority.Message) ([]priority.Result, error) {
			if classified != nil {
				*classified = messages
			}
			return results(), nil
		},
	}
}

func execute(t *testing.T, args []string, deps cli.Deps) run {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := cli.Run(context.Background(), args, deps, &stdout, &stderr)
	return run{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func TestRunPrintsResultsInPriorityOrder(t *testing.T) {
	got := execute(t, []string{"-input", "testdata/messages_ok.json"}, deps(t, nil, nil))
	if got.code != 0 {
		t.Fatalf("exit code %d, stderr: %s", got.code, got.stderr)
	}

	urgent := strings.Index(got.stdout, "m1")
	later := strings.Index(got.stdout, "m3")
	ignore := strings.Index(got.stdout, "m2")
	if urgent < 0 || later < 0 || ignore < 0 {
		t.Fatalf("a message is missing from the report:\n%s", got.stdout)
	}
	if !(urgent < later && later < ignore) {
		t.Fatalf("want urgent, then later, then ignore:\n%s", got.stdout)
	}
}

func TestRunPrintsEveryEvidenceQuote(t *testing.T) {
	got := execute(t, []string{"-input", "testdata/messages_ok.json"}, deps(t, nil, nil))
	for _, quote := range []string{"due Friday", "Weekly newsletter", "Much appreciated."} {
		if !strings.Contains(got.stdout, quote) {
			t.Errorf("the report does not show the quote %q:\n%s", quote, got.stdout)
		}
	}
}

func TestRunLabelsEveryMessage(t *testing.T) {
	got := execute(t, []string{"-input", "testdata/messages_ok.json"}, deps(t, nil, nil))
	for _, id := range []string{"m1", "m2", "m3"} {
		if !strings.Contains(got.stdout, id) {
			t.Errorf("message %s is not in the report:\n%s", id, got.stdout)
		}
	}
}

func TestRunEmitsJSONWhenAsked(t *testing.T) {
	got := execute(t, []string{"-input", "testdata/messages_ok.json", "-json"}, deps(t, nil, nil))
	if got.code != 0 {
		t.Fatalf("exit code %d, stderr: %s", got.code, got.stderr)
	}

	var document struct {
		Results []priority.Result `json:"results"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &document); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, got.stdout)
	}
	if len(document.Results) != 3 {
		t.Fatalf("want 3 results, got %d", len(document.Results))
	}
	if document.Results[0].MessageID != "m1" {
		t.Errorf("JSON output is not in priority order: %+v", document.Results)
	}
}

func TestJSONAndTextAgreeOnOrder(t *testing.T) {
	asJSON := execute(t, []string{"-input", "x.json", "-json"}, deps(t, nil, nil))
	asText := execute(t, []string{"-input", "x.json"}, deps(t, nil, nil))

	var document struct {
		Results []priority.Result `json:"results"`
	}
	if err := json.Unmarshal([]byte(asJSON.stdout), &document); err != nil {
		t.Fatalf("stdout is not JSON: %v", err)
	}

	previous := -1
	for _, result := range document.Results {
		position := strings.Index(asText.stdout, result.MessageID)
		if position < 0 || position < previous {
			t.Fatalf("the two outputs disagree on order at %s", result.MessageID)
		}
		previous = position
	}
}

func TestRunRejectsNoSource(t *testing.T) {
	got := execute(t, nil, deps(t, nil, nil))
	if got.code != 2 {
		t.Fatalf("want exit code 2 for a usage error, got %d", got.code)
	}
	if !strings.Contains(got.stderr, "input") {
		t.Fatalf("the error does not say what is missing: %s", got.stderr)
	}
}

func TestRunRejectsBothSourcesAtOnce(t *testing.T) {
	got := execute(t, []string{"-input", "x.json", "-gmail"}, deps(t, nil, nil))
	if got.code != 2 {
		t.Fatalf("want exit code 2, got %d", got.code)
	}
	if !strings.Contains(got.stderr, "one source") {
		t.Fatalf("the error does not explain the conflict: %s", got.stderr)
	}
}

func TestRunRejectsAnUnknownFlag(t *testing.T) {
	got := execute(t, []string{"-input", "x.json", "-nope"}, deps(t, nil, nil))
	if got.code != 2 {
		t.Fatalf("want exit code 2, got %d", got.code)
	}
	if !strings.Contains(got.stderr, "nope") {
		t.Fatalf("the error does not name the flag: %s", got.stderr)
	}
}

func TestRunRejectsPositionalArguments(t *testing.T) {
	got := execute(t, []string{"-input", "x.json", "extra"}, deps(t, nil, nil))
	if got.code != 2 {
		t.Fatalf("want exit code 2, got %d", got.code)
	}
}

func TestRunRejectsALimitBelowOne(t *testing.T) {
	got := execute(t, []string{"-input", "x.json", "-limit", "0"}, deps(t, nil, nil))
	if got.code != 2 {
		t.Fatalf("want exit code 2, got %d", got.code)
	}
	if !strings.Contains(got.stderr, "limit") {
		t.Fatalf("the error does not name the flag: %s", got.stderr)
	}
}

func TestRunReportsAReadFailure(t *testing.T) {
	d := deps(t, nil, nil)
	d.Read = func(context.Context, cli.Options) ([]priority.Message, error) {
		return nil, errors.New("the mailbox is unreachable")
	}

	got := execute(t, []string{"-input", "x.json"}, d)
	if got.code != 1 {
		t.Fatalf("want exit code 1, got %d", got.code)
	}
	if !strings.Contains(got.stderr, "the mailbox is unreachable") {
		t.Fatalf("the cause is not reported: %s", got.stderr)
	}
	if got.stdout != "" {
		t.Errorf("a failed run wrote to stdout: %s", got.stdout)
	}
}

func TestRunReportsAContractFailure(t *testing.T) {
	d := deps(t, nil, nil)
	d.Classify = func(context.Context, cli.Options, []priority.Message) ([]priority.Result, error) {
		return nil, errors.New("model broke the contract: quote_not_found")
	}

	got := execute(t, []string{"-input", "x.json"}, d)
	if got.code != 1 {
		t.Fatalf("want exit code 1, got %d", got.code)
	}
	if !strings.Contains(got.stderr, "quote_not_found") {
		t.Fatalf("the broken rule is not reported: %s", got.stderr)
	}
}

func TestRunSucceedsOnAnEmptyBatch(t *testing.T) {
	d := deps(t, nil, nil)
	d.Read = func(context.Context, cli.Options) ([]priority.Message, error) { return nil, nil }

	got := execute(t, []string{"-input", "x.json"}, d)
	if got.code != 0 {
		t.Fatalf("exit code %d, stderr: %s", got.code, got.stderr)
	}
	if strings.Contains(got.stdout, "m1") {
		t.Errorf("an empty batch produced output: %s", got.stdout)
	}
}

func TestEmptyBatchNeverReachesTheClassifier(t *testing.T) {
	var classified []priority.Message
	d := deps(t, nil, &classified)
	d.Read = func(context.Context, cli.Options) ([]priority.Message, error) { return nil, nil }

	if got := execute(t, []string{"-input", "x.json"}, d); got.code != 0 {
		t.Fatalf("exit code %d, stderr: %s", got.code, got.stderr)
	}
	if classified != nil {
		t.Fatalf("the classifier was called with %d messages", len(classified))
	}
}

func TestRunPassesTheFlagsToTheSource(t *testing.T) {
	var seen cli.Options
	d := deps(t, &seen, nil)

	got := execute(t, []string{"-input", "somewhere.json", "-limit", "7", "-model", "gemini-pro-latest"}, d)
	if got.code != 0 {
		t.Fatalf("exit code %d, stderr: %s", got.code, got.stderr)
	}
	if seen.Input != "somewhere.json" {
		t.Errorf("input: got %q", seen.Input)
	}
	if seen.Limit != 7 {
		t.Errorf("limit: got %d", seen.Limit)
	}
	if seen.Model != "gemini-pro-latest" {
		t.Errorf("model: got %q", seen.Model)
	}
	if seen.Gmail {
		t.Error("gmail was selected without being asked for")
	}
}

func TestRunReadsFromGmailWhenAsked(t *testing.T) {
	var seen cli.Options
	d := deps(t, &seen, nil)

	got := execute(t, []string{"-gmail", "-query", "is:unread", "-limit", "3"}, d)
	if got.code != 0 {
		t.Fatalf("exit code %d, stderr: %s", got.code, got.stderr)
	}
	if !seen.Gmail {
		t.Error("gmail was not selected")
	}
	if seen.Query != "is:unread" {
		t.Errorf("query: got %q", seen.Query)
	}
}

func TestHelpExplainsEveryFlagAndExitsZero(t *testing.T) {
	got := execute(t, []string{"-h"}, deps(t, nil, nil))
	if got.code != 0 {
		t.Fatalf("want exit code 0 for help, got %d", got.code)
	}
	for _, flag := range []string{"-input", "-gmail", "-query", "-limit", "-json", "-model"} {
		if !strings.Contains(got.stdout, flag) {
			t.Errorf("help does not mention %s:\n%s", flag, got.stdout)
		}
	}
}

func TestHelpDoesNotReachTheNetwork(t *testing.T) {
	d := deps(t, nil, nil)
	d.Read = func(context.Context, cli.Options) ([]priority.Message, error) {
		t.Error("help must not read anything")
		return nil, nil
	}

	if got := execute(t, []string{"-h"}, d); got.code != 0 {
		t.Fatalf("want exit code 0, got %d", got.code)
	}
}
