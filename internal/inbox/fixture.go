package inbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/GhandyP/06-inbox-prioritization/internal/priority"
)

// Rule identifies which structural rule a fixture file broke.
type Rule string

const (
	RuleUnreadable    Rule = "unreadable"
	RuleMalformedJSON Rule = "malformed_json"
	RuleUnknownField  Rule = "unknown_field"
	RuleMissingID     Rule = "missing_id"
	RuleMissingTime   Rule = "missing_received_at"
	RuleDuplicateID   Rule = "duplicate_id"
)

// Error is a typed fixture failure. It names the file, and the message when the
// problem belongs to one, so a caller can point at the offending entry.
type Error struct {
	Rule   Rule
	Path   string
	ID     string
	Detail string
}

func (e *Error) Error() string {
	if e.ID != "" {
		return fmt.Sprintf("inbox: %s: %s: message %q: %s", e.Rule, e.Path, e.ID, e.Detail)
	}
	return fmt.Sprintf("inbox: %s: %s: %s", e.Rule, e.Path, e.Detail)
}

// Fixture reads a batch from a JSON file.
//
// This is the offline path: every test in this repository runs against it, so no
// test needs credentials, a token, or a network, and the same file can be
// replayed to reproduce a report exactly.
type Fixture struct {
	Path string
}

type document struct {
	Messages []priority.Message `json:"messages"`
}

// Messages reads the file.
//
// It checks structure only. Ids must be present and unique, received_at must be
// present because ordering depends on it, and unknown fields are rejected so a
// typo cannot silently become a zero value. It deliberately does not check
// content: a message with an empty sender or an empty subject is still a message
// to prioritize.
//
// The first problem aborts the read, because a structurally broken file is
// rejected wholesale rather than partially trusted.
func (f Fixture) Messages(ctx context.Context) ([]priority.Message, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	file, err := os.Open(f.Path)
	if err != nil {
		return nil, &Error{Rule: RuleUnreadable, Path: f.Path, Detail: err.Error()}
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()

	var doc document
	if err := decoder.Decode(&doc); err != nil {
		return nil, &Error{Rule: decodeRule(err), Path: f.Path, Detail: err.Error()}
	}

	if err := checkStructure(doc.Messages, f.Path); err != nil {
		return nil, err
	}

	return doc.Messages, nil
}

// decodeRule separates a typo from a broken file. The encoder reports an unknown
// field as a plain error and exports no sentinel for it, so that one case has to
// be recognised by its wording.
func decodeRule(err error) Rule {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &syntaxErr) || errors.As(err, &typeErr) {
		return RuleMalformedJSON
	}
	if strings.Contains(err.Error(), "unknown field") {
		return RuleUnknownField
	}
	return RuleMalformedJSON
}

func checkStructure(messages []priority.Message, path string) *Error {
	seen := make(map[string]bool, len(messages))
	for i, message := range messages {
		if message.ID == "" {
			return &Error{
				Rule:   RuleMissingID,
				Path:   path,
				Detail: fmt.Sprintf("message at index %d has no id, so a result could not be traced back to it", i),
			}
		}
		if seen[message.ID] {
			return &Error{
				Rule:   RuleDuplicateID,
				Path:   path,
				ID:     message.ID,
				Detail: "the same id appears more than once, so results cannot be matched to messages",
			}
		}
		seen[message.ID] = true

		if message.ReceivedAt.IsZero() {
			return &Error{
				Rule:   RuleMissingTime,
				Path:   path,
				ID:     message.ID,
				Detail: "received_at is missing; ordering depends on it and it is never inferred",
			}
		}
	}
	return nil
}
