package priority_test

import (
	"slices"
	"testing"

	"github.com/GhandyP/06-inbox-prioritization/internal/priority"
)

// These tests pin the ordering guarantee: the position of a result depends only
// on data the reader supplied and on the closed label set, never on the model
// and never on the order the results arrived in.

func msg(id string, hour int) priority.Message {
	return priority.Message{
		ID:         id,
		Sender:     "someone@example.com",
		Subject:    "subject " + id,
		Body:       "body " + id,
		ReceivedAt: at(hour),
	}
}

func res(id string, label priority.Label) priority.Result {
	return priority.Result{
		MessageID: id,
		Label:     label,
		Reasons:   []priority.Reason{{Kind: priority.KindBulkMail, Quote: "body " + id}},
	}
}

func idsOf(results []priority.Result) []string {
	out := make([]string, 0, len(results))
	for _, r := range results {
		out = append(out, r.MessageID)
	}
	return out
}

func TestLabelRankFollowsTheClosedOrder(t *testing.T) {
	for want, label := range priority.Labels() {
		if got := label.Rank(); got != want {
			t.Fatalf("rank of %q: want %d, got %d", label, want, got)
		}
	}
	if got := priority.Label("high").Rank(); got != -1 {
		t.Fatalf("rank of an unknown label: want -1, got %d", got)
	}
}

func TestOrderSortsByLabelRankFirst(t *testing.T) {
	messages := []priority.Message{msg("a", 12), msg("b", 12), msg("c", 12), msg("d", 12)}
	results := []priority.Result{
		res("d", priority.Ignore),
		res("c", priority.Later),
		res("b", priority.Today),
		res("a", priority.Urgent),
	}

	got := idsOf(priority.Order(results, messages))
	want := []string{"a", "b", "c", "d"}
	if !slices.Equal(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestOrderBreaksLabelTiesByReceivedAt(t *testing.T) {
	messages := []priority.Message{msg("a", 10), msg("b", 9), msg("c", 11)}
	results := []priority.Result{
		res("a", priority.Today),
		res("b", priority.Today),
		res("c", priority.Today),
	}

	got := idsOf(priority.Order(results, messages))
	want := []string{"b", "a", "c"}
	if !slices.Equal(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestOrderBreaksRemainingTiesByMessageID(t *testing.T) {
	messages := []priority.Message{msg("c", 10), msg("a", 10), msg("b", 10)}
	results := []priority.Result{
		res("c", priority.Urgent),
		res("a", priority.Urgent),
		res("b", priority.Urgent),
	}

	got := idsOf(priority.Order(results, messages))
	want := []string{"a", "b", "c"}
	if !slices.Equal(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestOrderIgnoresTheInputOrder(t *testing.T) {
	messages := []priority.Message{msg("a", 10), msg("b", 10), msg("c", 9), msg("d", 11)}
	forward := []priority.Result{
		res("a", priority.Ignore),
		res("b", priority.Today),
		res("c", priority.Urgent),
		res("d", priority.Later),
	}
	reversed := []priority.Result{
		res("d", priority.Later),
		res("c", priority.Urgent),
		res("b", priority.Today),
		res("a", priority.Ignore),
	}

	want := idsOf(priority.Order(forward, messages))
	got := idsOf(priority.Order(reversed, messages))
	if !slices.Equal(got, want) {
		t.Fatalf("input order changed the result: want %v, got %v", want, got)
	}
}

func TestOrderIsIdempotent(t *testing.T) {
	messages := []priority.Message{msg("a", 10), msg("b", 10), msg("c", 10), msg("d", 10)}
	results := []priority.Result{
		res("d", priority.Ignore),
		res("c", priority.Later),
		res("b", priority.Today),
		res("a", priority.Urgent),
	}

	once := priority.Order(results, messages)
	if len(once) != len(results) {
		t.Fatalf("want %d results, got %d", len(results), len(once))
	}
	twice := priority.Order(once, messages)
	if !slices.Equal(idsOf(once), idsOf(twice)) {
		t.Fatalf("ordering is not idempotent: %v then %v", idsOf(once), idsOf(twice))
	}
}

func TestOrderDoesNotMutateItsInput(t *testing.T) {
	messages := []priority.Message{msg("a", 12), msg("b", 12)}
	results := []priority.Result{res("b", priority.Ignore), res("a", priority.Urgent)}
	before := idsOf(results)

	priority.Order(results, messages)

	if after := idsOf(results); !slices.Equal(before, after) {
		t.Fatalf("input mutated: want %v, got %v", before, after)
	}
}

func TestOrderSortsUnknownLabelsLast(t *testing.T) {
	messages := []priority.Message{msg("a", 10), msg("b", 10)}
	results := []priority.Result{
		res("a", priority.Label("high")),
		res("b", priority.Ignore),
	}

	got := idsOf(priority.Order(results, messages))
	want := []string{"b", "a"}
	if !slices.Equal(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestOrderSortsResultsWithoutAMessageLast(t *testing.T) {
	messages := []priority.Message{msg("a", 10)}
	results := []priority.Result{
		res("ghost", priority.Urgent),
		res("a", priority.Ignore),
	}

	got := idsOf(priority.Order(results, messages))
	want := []string{"a", "ghost"}
	if !slices.Equal(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestOrderAcceptsAnEmptyBatch(t *testing.T) {
	if got := priority.Order(nil, nil); len(got) != 0 {
		t.Fatalf("want no results, got %v", idsOf(got))
	}
}

// The sort key is not unique for every possible input: two results that name the
// same message, carry the same label and see the same timestamp compare equal.
// Validation rules that out by rejecting duplicate result ids, so this only
// concerns unvalidated input. The sort is stable, so such a pair keeps its input
// order instead of an arbitrary one, and this test pins that rather than leaving
// it to the reader of the sort call.
func TestOrderKeepsExactTiesInInputOrder(t *testing.T) {
	messages := []priority.Message{msg("m", 9)}
	first := res("m", priority.Today)
	second := res("m", priority.Today)
	second.Reasons = []priority.Reason{{Kind: priority.KindDirectQuestion, Quote: "second"}}

	forward := priority.Order([]priority.Result{first, second}, messages)
	if got := forward[0].Reasons[0].Quote; got != "body m" {
		t.Fatalf("want the first input result to stay first, got %q", got)
	}

	backward := priority.Order([]priority.Result{second, first}, messages)
	if got := backward[0].Reasons[0].Quote; got != "second" {
		t.Fatalf("want the first input result to stay first, got %q", got)
	}
}
