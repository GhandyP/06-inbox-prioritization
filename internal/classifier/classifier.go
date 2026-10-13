// Package classifier asks a Google ADK agent to propose a priority label and
// evidence for every message in a batch.
//
// The package proposes; it does not decide. Everything a model returns passes
// through internal/priority before a caller sees it, so a label outside the closed
// vocabulary, a result with no justification, or a quote that does not appear in
// the message is refused rather than reported.
//
// The agent is bound with no tools at all. With no tools, no toolsets and no
// subagents, ADK registers none, so there is no tool the model could call to act
// on a mailbox even if a later edit tried to add one by accident.
package classifier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"

	"github.com/GhandyP/06-inbox-prioritization/internal/priority"
)

// DefaultModel is used when ADK_MODEL is not set.
const DefaultModel = "gemini-flash-latest"

const (
	defaultAppName = "inbox_prioritization"
	defaultUserID  = "local"
	agentName      = "inbox_prioritizer"
)

// ModelName returns the model to use, honouring the ADK_MODEL override.
func ModelName() string {
	if override := strings.TrimSpace(os.Getenv("ADK_MODEL")); override != "" {
		return override
	}
	return DefaultModel
}

// Config configures the ADK agent.
type Config struct {
	// AppName names the ADK application. Empty uses a fixed name.
	AppName string

	// UserID owns the session. Empty uses a fixed name.
	UserID string

	// ModelName overrides the model. Empty uses ModelName.
	ModelName string
}

// Responder runs one prompt against one fresh session and returns the final
// structured payload.
//
// ADKResponder is the real implementation. Tests supply their own, which is why
// this is exported: it is the seam that keeps the suite offline.
type Responder interface {
	Respond(ctx context.Context, prompt string) (map[string]any, error)
}

// Agent proposes results through a Responder and then validates them.
type Agent struct {
	responder Responder
}

// NewWithResponder builds an agent over a supplied Responder.
func NewWithResponder(responder Responder) *Agent {
	return &Agent{responder: responder}
}

// Classify proposes a classification for every message and refuses to return it
// unless the whole batch satisfies the contract.
//
// An empty batch never reaches the model.
func (a *Agent) Classify(ctx context.Context, messages []priority.Message) ([]priority.Result, error) {
	if len(messages) == 0 {
		return nil, nil
	}

	payload, err := a.responder.Respond(ctx, buildPrompt(messages))
	if err != nil {
		return nil, err
	}

	results, err := parseResults(payload)
	if err != nil {
		return nil, err
	}

	if err := priority.Validate(messages, results); err != nil {
		return nil, fmt.Errorf("classifier: the model broke the contract: %w", err)
	}
	return results, nil
}

// New builds the ADK-backed agent.
func New(ctx context.Context, cfg Config) (*Agent, error) {
	apiKey := strings.TrimSpace(os.Getenv("GOOGLE_API_KEY"))
	if apiKey == "" {
		return nil, errors.New("classifier: GOOGLE_API_KEY is not set; get a key from https://aistudio.google.com/apikey")
	}

	name := strings.TrimSpace(cfg.ModelName)
	if name == "" {
		name = ModelName()
	}

	llm, err := gemini.NewModel(ctx, name, &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("classifier: create the model %q: %w", name, err)
	}

	root, err := llmagent.New(agentConfig(llm))
	if err != nil {
		return nil, fmt.Errorf("classifier: create the agent: %w", err)
	}

	appName := strings.TrimSpace(cfg.AppName)
	if appName == "" {
		appName = defaultAppName
	}
	userID := strings.TrimSpace(cfg.UserID)
	if userID == "" {
		userID = defaultUserID
	}

	sessions := session.InMemoryService()
	run, err := runner.New(runner.Config{AppName: appName, Agent: root, SessionService: sessions})
	if err != nil {
		return nil, fmt.Errorf("classifier: create the runner: %w", err)
	}

	return NewWithResponder(&ADKResponder{
		runner:   run,
		sessions: sessions,
		appName:  appName,
		userID:   userID,
	}), nil
}

// agentConfig builds the ADK agent configuration.
//
// Tools, Toolsets, SubAgents and Mode are deliberately left at their zero values.
// With none of them ADK registers no tool at all, so the agent has no way to act
// on a mailbox. agent_test.go asserts that this stays true.
func agentConfig(llm model.LLM) llmagent.Config {
	return llmagent.Config{
		Name:         agentName,
		Description:  "Classifies inbox messages by urgency and cites the evidence for each label.",
		Model:        llm,
		Instruction:  instruction(),
		OutputSchema: outputSchema(),
	}
}

func instruction() string {
	return "You prioritize an inbox. For every supplied message choose exactly one label from the " +
		"closed set and give at least one reason whose quote is copied word for word from that " +
		"message. Never invent a due date, never infer who sent a message, and never propose an " +
		"action to take on the mailbox."
}

// outputSchema constrains decoding to the contract. An unknown label is not
// something to check for afterwards: with a constrained schema it cannot be
// produced in the first place.
func outputSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"results": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"message_id": {Type: genai.TypeString},
						"label":      {Type: genai.TypeString, Enum: labelNames()},
						"reasons": {
							Type: genai.TypeArray,
							Items: &genai.Schema{
								Type: genai.TypeObject,
								Properties: map[string]*genai.Schema{
									"kind":  {Type: genai.TypeString, Enum: kindNames()},
									"quote": {Type: genai.TypeString},
								},
								Required: []string{"kind", "quote"},
							},
						},
					},
					Required: []string{"message_id", "label", "reasons"},
				},
			},
		},
		Required: []string{"results"},
	}
}

type document struct {
	Results []priority.Result `json:"results"`
}

// parseResults decodes the structured payload into the contract.
//
// Unknown fields are refused rather than ignored. A constrained schema cannot
// produce one, so an extra field means the output did not come from the schema
// that was requested.
func parseResults(payload map[string]any) ([]priority.Result, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("classifier: re-encode the model output: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	var doc document
	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("classifier: the model output is not the requested shape: %w", err)
	}
	return doc.Results, nil
}

// buildPrompt renders the batch next to the closed vocabularies it has to answer
// with, so the model is told what it may say instead of being left to guess.
func buildPrompt(messages []priority.Message) string {
	var builder strings.Builder

	builder.WriteString("Prioritize every message below. Answer with one result per message.\n\n")
	builder.WriteString("Labels, most urgent first: ")
	builder.WriteString(strings.Join(labelNames(), ", "))
	builder.WriteString("\nReason kinds: ")
	builder.WriteString(strings.Join(kindNames(), ", "))
	builder.WriteString("\n\nRules:\n")
	builder.WriteString("- Every reason quote must be copied word for word from that message's subject or body.\n")
	builder.WriteString("- Never invent a due date and never restate a date the message does not contain.\n")
	builder.WriteString("- A message with nothing that needs action is `ignore`.\n\nMessages:\n")

	for _, message := range messages {
		builder.WriteString("\n--- id: ")
		builder.WriteString(message.ID)
		builder.WriteString("\nfrom: ")
		builder.WriteString(message.Sender)
		builder.WriteString("\nreceived_at: ")
		builder.WriteString(message.ReceivedAt.UTC().Format(time.RFC3339))
		builder.WriteString("\nsubject: ")
		builder.WriteString(message.Subject)
		builder.WriteString("\nbody: ")
		builder.WriteString(message.Body)
		builder.WriteString("\n")
	}

	return builder.String()
}

func labelNames() []string {
	labels := priority.Labels()
	names := make([]string, 0, len(labels))
	for _, label := range labels {
		names = append(names, string(label))
	}
	return names
}

func kindNames() []string {
	kinds := priority.Kinds()
	names := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		names = append(names, string(kind))
	}
	return names
}

// ADKResponder runs one prompt per batch through a fresh ADK session.
type ADKResponder struct {
	runner   *runner.Runner
	sessions session.Service
	appName  string
	userID   string
}

// Respond creates a session, runs the prompt, and returns the final structured
// payload.
//
// The session is created per call on purpose. A batch must not be able to see the
// previous batch in its history, so one batch cannot influence the next.
func (r *ADKResponder) Respond(ctx context.Context, prompt string) (map[string]any, error) {
	created, err := r.sessions.Create(ctx, &session.CreateRequest{
		AppName: r.appName,
		UserID:  r.userID,
	})
	if err != nil {
		return nil, fmt.Errorf("classifier: create the session: %w", err)
	}

	events := r.runner.Run(ctx, r.userID, created.Session.ID(),
		genai.NewContentFromText(prompt, genai.RoleUser), agent.RunConfig{})

	for event, err := range events {
		if err != nil {
			return nil, fmt.Errorf("classifier: run the agent: %w", err)
		}
		if !event.IsFinalResponse() {
			continue
		}
		payload, ok := event.Output.(map[string]any)
		if !ok {
			return nil, errors.New("classifier: the agent returned no structured output")
		}
		return payload, nil
	}

	return nil, errors.New("classifier: the agent produced no final response")
}
