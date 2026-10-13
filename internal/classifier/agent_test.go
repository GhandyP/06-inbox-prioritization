package classifier

import (
	"slices"
	"testing"

	"google.golang.org/adk/v2/agent/llmagent"

	"github.com/GhandyP/06-inbox-prioritization/internal/priority"
)

// These are white-box guard tests. The whole point of them is to assert a
// property of the agent configuration that a caller cannot observe from the
// outside, so they read the unexported builder directly.

func TestAgentConfigBindsNoTools(t *testing.T) {
	cfg := agentConfig(nil)

	if len(cfg.Tools) != 0 {
		t.Errorf("want no tools, got %d", len(cfg.Tools))
	}
	if len(cfg.Toolsets) != 0 {
		t.Errorf("want no toolsets, got %d", len(cfg.Toolsets))
	}
	if len(cfg.SubAgents) != 0 {
		t.Errorf("want no subagents, got %d", len(cfg.SubAgents))
	}
	if len(cfg.BeforeToolCallbacks) != 0 || len(cfg.AfterToolCallbacks) != 0 {
		t.Error("want no tool callbacks")
	}

	// ModeTask would make ADK register the finish_task tool, so the mode has to
	// stay at its zero value.
	var zero llmagent.Mode
	if cfg.Mode != zero {
		t.Errorf("want the zero mode, got %v", cfg.Mode)
	}
}

func TestAgentConfigDeclaresTheOutputSchema(t *testing.T) {
	cfg := agentConfig(nil)
	if cfg.OutputSchema == nil {
		t.Fatal("want an output schema")
	}

	results := cfg.OutputSchema.Properties["results"]
	if results == nil || results.Items == nil {
		t.Fatal("want a results array in the schema")
	}

	wantLabels := make([]string, 0, 4)
	for _, label := range priority.Labels() {
		wantLabels = append(wantLabels, string(label))
	}
	label := results.Items.Properties["label"]
	if label == nil {
		t.Fatal("want a label property")
	}
	if !slices.Equal(label.Enum, wantLabels) {
		t.Fatalf("label enum: want %v, got %v", wantLabels, label.Enum)
	}

	reasons := results.Items.Properties["reasons"]
	if reasons == nil || reasons.Items == nil {
		t.Fatal("want a reasons array in the schema")
	}

	wantKinds := make([]string, 0, 5)
	for _, kind := range priority.Kinds() {
		wantKinds = append(wantKinds, string(kind))
	}
	kind := reasons.Items.Properties["kind"]
	if kind == nil {
		t.Fatal("want a kind property")
	}
	if !slices.Equal(kind.Enum, wantKinds) {
		t.Fatalf("kind enum: want %v, got %v", wantKinds, kind.Enum)
	}

	if results.Items.Properties["message_id"] == nil {
		t.Error("want a message_id property")
	}
	if reasons.Items.Properties["quote"] == nil {
		t.Error("want a quote property")
	}
}

func TestOutputSchemaRequiresEveryPropertyItDeclares(t *testing.T) {
	cfg := agentConfig(nil)

	if !slices.Contains(cfg.OutputSchema.Required, "results") {
		t.Errorf("results is not required: %v", cfg.OutputSchema.Required)
	}

	item := cfg.OutputSchema.Properties["results"].Items
	for _, want := range []string{"message_id", "label", "reasons"} {
		if !slices.Contains(item.Required, want) {
			t.Errorf("%s is not required: %v", want, item.Required)
		}
	}

	reason := item.Properties["reasons"].Items
	for _, want := range []string{"kind", "quote"} {
		if !slices.Contains(reason.Required, want) {
			t.Errorf("%s is not required: %v", want, reason.Required)
		}
	}
}
