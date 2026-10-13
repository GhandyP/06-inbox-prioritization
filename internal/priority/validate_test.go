package priority_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/GhandyP/06-inbox-prioritization/internal/priority"
)

// This file is the executable form of the contract. It exercises only the
// package's public interface, so it cannot pass by reaching into internals.

func at(hour int) time.Time {
	return time.Date(2026, time.October, 13, hour, 0, 0, 0, time.UTC)
}

func sampleMessages() []priority.Message {
	return []priority.Message{
		{
			ID:         "m1",
			Sender:     "ana@example.com",
			Subject:    "Invoice 8841 due Friday",
			Body:       "Please confirm the amount before Friday.",
			ReceivedAt: at(9),
		},
		{
			ID:         "m2",
			Sender:     "news@example.com",
			Subject:    "Weekly newsletter",
			Body:       "Ten links you may enjoy.",
			ReceivedAt: at(10),
		},
	}
}

func validResults() []priority.Result {
	return []priority.Result{
		{
			MessageID: "m1",
			Label:     priority.Urgent,
			Reasons: []priority.Reason{
				{Kind: priority.KindDeadlineLanguage, Quote: "due Friday"},
			},
		},
		{
			MessageID: "m2",
			Label:     priority.Ignore,
			Reasons: []priority.Reason{
				{Kind: priority.KindBulkMail, Quote: "Weekly newsletter"},
			},
		},
	}
}

func violationsOf(t *testing.T, err error) []priority.Violation {
	t.Helper()
	if err == nil {
		t.Fatal("want a *priority.BatchError, got nil")
	}
	var batch *priority.BatchError
	if !errors.As(err, &batch) {
		t.Fatalf("want a *priority.BatchError, got %T: %v", err, err)
	}
	if len(batch.Violations) == 0 {
		t.Fatal("BatchError carries no violations")
	}
	return batch.Violations
}

func hasViolation(violations []priority.Violation, rule priority.Rule, messageID string) bool {
	for _, v := range violations {
		if v.Rule == rule && v.MessageID == messageID {
			return true
		}
	}
	return false
}

func mustHave(t *testing.T, violations []priority.Violation, rule priority.Rule, messageID string) {
	t.Helper()
	if !hasViolation(violations, rule, messageID) {
		t.Fatalf("want rule %q for message %q, got %+v", rule, messageID, violations)
	}
}

func TestValidateAcceptsAValidBatch(t *testing.T) {
	if err := priority.Validate(sampleMessages(), validResults()); err != nil {
		t.Fatalf("valid batch rejected: %v", err)
	}
}

func TestValidateAcceptsAnEmptyBatch(t *testing.T) {
	if err := priority.Validate(nil, nil); err != nil {
		t.Fatalf("empty batch rejected: %v", err)
	}
}

func TestLabelSetIsClosed(t *testing.T) {
	want := []priority.Label{"urgent", "today", "later", "ignore"}
	got := priority.Labels()
	if len(got) != len(want) {
		t.Fatalf("want %d labels, got %d (%v)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("label %d: want %q, got %q", i, want[i], got[i])
		}
	}
}

func TestValidateRejectsAnUnknownLabel(t *testing.T) {
	results := validResults()
	results[0].Label = "high"

	err := priority.Validate(sampleMessages(), results)
	mustHave(t, violationsOf(t, err), priority.RuleUnknownLabel, "m1")
}

func TestValidateRejectsAnUnknownKind(t *testing.T) {
	results := validResults()
	results[0].Reasons[0].Kind = "vibes"

	err := priority.Validate(sampleMessages(), results)
	mustHave(t, violationsOf(t, err), priority.RuleUnknownKind, "m1")
}

func TestValidateRejectsADuplicateResult(t *testing.T) {
	results := validResults()
	results = append(results, results[0])

	err := priority.Validate(sampleMessages(), results)
	mustHave(t, violationsOf(t, err), priority.RuleDuplicateResult, "m1")
}

func TestValidateRejectsAnUnknownMessageID(t *testing.T) {
	results := validResults()
	results = append(results, priority.Result{
		MessageID: "m404",
		Label:     priority.Later,
		Reasons: []priority.Reason{
			{Kind: priority.KindDirectQuestion, Quote: "amount"},
		},
	})

	err := priority.Validate(sampleMessages(), results)
	mustHave(t, violationsOf(t, err), priority.RuleUnknownMessage, "m404")
}

func TestValidateRejectsAMissingResult(t *testing.T) {
	err := priority.Validate(sampleMessages(), validResults()[:1])
	mustHave(t, violationsOf(t, err), priority.RuleMissingResult, "m2")
}

func TestValidateRejectsAResultWithoutReasons(t *testing.T) {
	results := validResults()
	results[1].Reasons = nil

	err := priority.Validate(sampleMessages(), results)
	mustHave(t, violationsOf(t, err), priority.RuleNoReasons, "m2")
}

func TestValidateRejectsAnEmptyQuote(t *testing.T) {
	results := validResults()
	results[0].Reasons[0].Quote = "   "

	err := priority.Validate(sampleMessages(), results)
	mustHave(t, violationsOf(t, err), priority.RuleEmptyQuote, "m1")
}

func TestValidateRejectsAQuoteThatIsNotInTheMessage(t *testing.T) {
	results := validResults()
	results[0].Reasons[0].Quote = "final notice before legal action"

	err := priority.Validate(sampleMessages(), results)
	mustHave(t, violationsOf(t, err), priority.RuleQuoteNotFound, "m1")
}

func TestValidateAcceptsAQuoteFromTheSubject(t *testing.T) {
	results := validResults()
	results[0].Reasons[0].Quote = "Invoice 8841"

	if err := priority.Validate(sampleMessages(), results); err != nil {
		t.Fatalf("subject quote rejected: %v", err)
	}
}

func TestValidateAcceptsAQuoteFromTheBody(t *testing.T) {
	results := validResults()
	results[0].Reasons[0].Quote = "confirm the amount"

	if err := priority.Validate(sampleMessages(), results); err != nil {
		t.Fatalf("body quote rejected: %v", err)
	}
}

func TestValidateAcceptsAQuoteWithFoldedWhitespace(t *testing.T) {
	messages := []priority.Message{
		{
			ID:         "m1",
			Sender:     "ana@example.com",
			Subject:    "Invoice",
			Body:       "Confirm the\n   amount before Friday.",
			ReceivedAt: at(9),
		},
	}
	results := []priority.Result{
		{
			MessageID: "m1",
			Label:     priority.Urgent,
			Reasons: []priority.Reason{
				{Kind: priority.KindDeadlineLanguage, Quote: "the amount before Friday"},
			},
		},
	}

	if err := priority.Validate(messages, results); err != nil {
		t.Fatalf("whitespace-folded quote rejected: %v", err)
	}
}

func TestValidateRejectsAQuoteWithDifferentCase(t *testing.T) {
	results := validResults()
	results[1].Reasons[0].Quote = "weekly newsletter"

	violations := violationsOf(t, priority.Validate(sampleMessages(), results))
	mustHave(t, violations, priority.RuleQuoteNotFound, "m2")
	if len(violations) != 1 {
		t.Fatalf("case sensitivity should be the only problem, got %+v", violations)
	}
}

// Folding whitespace relaxes the rule by exactly one thing: layout. It must not
// let a quote reach across the join between the subject and the body, because
// that would accept text that appears in neither field.
func TestValidateRejectsAQuoteThatSpansTheSubjectAndTheBody(t *testing.T) {
	messages := []priority.Message{
		{ID: "m1", Subject: "Quarterly", Body: "report follows"},
	}
	results := []priority.Result{
		{
			MessageID: "m1",
			Label:     priority.Today,
			Reasons:   []priority.Reason{{Kind: priority.KindDirectQuestion, Quote: "Quarterly report"}},
		},
	}

	err := priority.Validate(messages, results)
	mustHave(t, violationsOf(t, err), priority.RuleQuoteNotFound, "m1")
}

func TestValidateReportsEveryViolationNotJustTheFirst(t *testing.T) {
	results := validResults()
	results[0].Label = "high"
	results[0].Reasons[0].Quote = "not in the message"
	results = results[:1]

	violations := violationsOf(t, priority.Validate(sampleMessages(), results))
	mustHave(t, violations, priority.RuleUnknownLabel, "m1")
	mustHave(t, violations, priority.RuleQuoteNotFound, "m1")
	mustHave(t, violations, priority.RuleMissingResult, "m2")
}

func TestBatchErrorNamesEveryOffendingMessage(t *testing.T) {
	results := validResults()
	results[0].Label = "high"

	var batch *priority.BatchError
	if !errors.As(priority.Validate(sampleMessages(), results), &batch) {
		t.Fatal("want a *priority.BatchError")
	}
	if msg := batch.Error(); !strings.Contains(msg, "m1") {
		t.Fatalf("BatchError does not name the offending message: %s", msg)
	}
}
