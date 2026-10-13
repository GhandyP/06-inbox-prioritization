package classifier_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/GhandyP/06-inbox-prioritization/internal/classifier"
	"github.com/GhandyP/06-inbox-prioritization/internal/priority"
)

// The classifier proposes; it never decides. Everything a model returns is
// checked by internal/priority before a caller sees it, and these tests pin that
// the check cannot be skipped.

type stubResponder struct {
	payload   map[string]any
	err       error
	calls     int
	prompts   []string
	responded bool
}

func (s *stubResponder) Respond(_ context.Context, prompt string) (map[string]any, error) {
	s.calls++
	s.responded = true
	s.prompts = append(s.prompts, prompt)
	if s.err != nil {
		return nil, s.err
	}
	return s.payload, nil
}

func batch() []priority.Message {
	return []priority.Message{
		{
			ID:         "m1",
			Sender:     "ana@example.com",
			Subject:    "Invoice 8841 due Friday",
			Body:       "Please confirm before Friday.",
			ReceivedAt: time.Date(2026, time.October, 13, 9, 0, 0, 0, time.UTC),
		},
		{
			ID:         "m2",
			Sender:     "news@example.com",
			Subject:    "Weekly newsletter",
			Body:       "Links you may enjoy.",
			ReceivedAt: time.Date(2026, time.October, 13, 10, 0, 0, 0, time.UTC),
		},
	}
}

func reason(kind, quote string) map[string]any {
	return map[string]any{"kind": kind, "quote": quote}
}

func validPayload() map[string]any {
	return map[string]any{
		"results": []any{
			map[string]any{
				"message_id": "m1",
				"label":      "urgent",
				"reasons":    []any{reason("deadline_language", "due Friday")},
			},
			map[string]any{
				"message_id": "m2",
				"label":      "ignore",
				"reasons":    []any{reason("bulk_mail", "Weekly newsletter")},
			},
		},
	}
}

func TestClassifyReturnsValidatedResults(t *testing.T) {
	stub := &stubResponder{payload: validPayload()}

	results, err := classifier.NewWithResponder(stub).Classify(context.Background(), batch())
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}
	if results[0].MessageID != "m1" || results[0].Label != priority.Urgent {
		t.Errorf("unexpected first result: %+v", results[0])
	}
	if len(results[0].Reasons) != 1 || results[0].Reasons[0].Kind != priority.KindDeadlineLanguage {
		t.Errorf("unexpected reasons: %+v", results[0].Reasons)
	}
}

func TestClassifyRejectsAnUnknownLabel(t *testing.T) {
	payload := validPayload()
	payload["results"].([]any)[0].(map[string]any)["label"] = "high"

	_, err := classifier.NewWithResponder(&stubResponder{payload: payload}).Classify(context.Background(), batch())
	if err == nil {
		t.Fatal("an unknown label must be refused")
	}
	if !strings.Contains(err.Error(), string(priority.RuleUnknownLabel)) {
		t.Fatalf("the error does not name the broken rule: %v", err)
	}
}

func TestClassifyRejectsAQuoteThatIsNotInTheMessage(t *testing.T) {
	payload := validPayload()
	payload["results"].([]any)[0].(map[string]any)["reasons"] = []any{
		reason("deadline_language", "legal action will follow"),
	}

	_, err := classifier.NewWithResponder(&stubResponder{payload: payload}).Classify(context.Background(), batch())
	if err == nil {
		t.Fatal("a fabricated quote must be refused")
	}
	if !strings.Contains(err.Error(), string(priority.RuleQuoteNotFound)) {
		t.Fatalf("the error does not name the broken rule: %v", err)
	}
}

func TestClassifyRejectsAMissingResult(t *testing.T) {
	payload := validPayload()
	payload["results"] = payload["results"].([]any)[:1]

	_, err := classifier.NewWithResponder(&stubResponder{payload: payload}).Classify(context.Background(), batch())
	if err == nil {
		t.Fatal("an unclassified message must be refused")
	}
	if !strings.Contains(err.Error(), string(priority.RuleMissingResult)) {
		t.Fatalf("the error does not name the broken rule: %v", err)
	}
}

func TestClassifyRejectsAnExtraField(t *testing.T) {
	payload := validPayload()
	payload["results"].([]any)[0].(map[string]any)["confidence"] = 0.9

	_, err := classifier.NewWithResponder(&stubResponder{payload: payload}).Classify(context.Background(), batch())
	if err == nil {
		t.Fatal("output that does not match the requested shape must be refused")
	}
}

func TestClassifyRejectsAMalformedPayload(t *testing.T) {
	_, err := classifier.NewWithResponder(&stubResponder{payload: map[string]any{"nope": true}}).
		Classify(context.Background(), batch())
	if err == nil {
		t.Fatal("a payload without results must be refused")
	}
}

func TestClassifyPropagatesAResponderFailure(t *testing.T) {
	want := errors.New("model unavailable")

	_, err := classifier.NewWithResponder(&stubResponder{err: want}).Classify(context.Background(), batch())
	if !errors.Is(err, want) {
		t.Fatalf("want the responder error, got %v", err)
	}
}

func TestClassifyNeverCallsTheModelForAnEmptyBatch(t *testing.T) {
	stub := &stubResponder{payload: validPayload()}

	results, err := classifier.NewWithResponder(stub).Classify(context.Background(), nil)
	if err != nil {
		t.Fatalf("an empty batch is not an error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("want no results, got %d", len(results))
	}
	if stub.calls != 0 {
		t.Fatalf("the model was called %d times for an empty batch", stub.calls)
	}
}

func TestClassifyMakesOneCallPerBatch(t *testing.T) {
	stub := &stubResponder{payload: validPayload()}

	if _, err := classifier.NewWithResponder(stub).Classify(context.Background(), batch()); err != nil {
		t.Fatalf("classify: %v", err)
	}
	if stub.calls != 1 {
		t.Fatalf("want one model call per batch, got %d", stub.calls)
	}
}

func TestPromptCarriesEveryMessage(t *testing.T) {
	stub := &stubResponder{payload: validPayload()}

	if _, err := classifier.NewWithResponder(stub).Classify(context.Background(), batch()); err != nil {
		t.Fatalf("classify: %v", err)
	}
	if len(stub.prompts) != 1 {
		t.Fatalf("want one prompt, got %d", len(stub.prompts))
	}

	prompt := stub.prompts[0]
	for _, want := range []string{"m1", "m2", "ana@example.com", "Invoice 8841 due Friday", "Weekly newsletter"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt does not carry %q", want)
		}
	}
}

func TestPromptSpellsOutTheClosedVocabulary(t *testing.T) {
	stub := &stubResponder{payload: validPayload()}

	if _, err := classifier.NewWithResponder(stub).Classify(context.Background(), batch()); err != nil {
		t.Fatalf("classify: %v", err)
	}

	prompt := stub.prompts[0]
	for _, label := range priority.Labels() {
		if !strings.Contains(prompt, string(label)) {
			t.Errorf("the prompt does not offer the label %q", label)
		}
	}
	for _, kind := range priority.Kinds() {
		if !strings.Contains(prompt, string(kind)) {
			t.Errorf("the prompt does not offer the kind %q", kind)
		}
	}
}

func TestNewRequiresAnAPIKey(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "")

	if _, err := classifier.New(context.Background(), classifier.Config{}); err == nil {
		t.Fatal("want an error when GOOGLE_API_KEY is unset")
	}
}

func TestModelNameHonoursTheEnvironment(t *testing.T) {
	t.Setenv("ADK_MODEL", "gemini-pro-latest")
	if got := classifier.ModelName(); got != "gemini-pro-latest" {
		t.Fatalf("want the ADK_MODEL override, got %q", got)
	}

	t.Setenv("ADK_MODEL", "")
	if got := classifier.ModelName(); got != classifier.DefaultModel {
		t.Fatalf("want the default, got %q", got)
	}
}
