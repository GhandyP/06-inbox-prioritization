package priority

import (
	"fmt"
	"strings"
)

// Validate checks that results classify messages under the contract.
//
// It returns nil when the batch is valid. Otherwise it returns a *BatchError
// carrying every violation found, so one run reports every problem rather than
// stopping at the first.
//
// The contract is:
//
//   - every message in the batch has exactly one result, and no result names a
//     message that is not in the batch
//   - every label is in the closed set returned by Labels
//   - every reason kind is in the closed set returned by Kinds
//   - every result carries at least one reason
//   - every reason quote is a non-blank literal substring of that message's
//     subject or body, after folding whitespace
func Validate(messages []Message, results []Result) error {
	byID := make(map[string]Message, len(messages))
	for _, message := range messages {
		byID[message.ID] = message
	}

	var violations []Violation
	seen := make(map[string]bool, len(results))

	for _, result := range results {
		if seen[result.MessageID] {
			violations = append(violations, Violation{
				Rule:      RuleDuplicateResult,
				MessageID: result.MessageID,
				Detail:    "already classified earlier in this batch",
			})
		}
		seen[result.MessageID] = true

		if !isKnownLabel(result.Label) {
			violations = append(violations, Violation{
				Rule:      RuleUnknownLabel,
				MessageID: result.MessageID,
				Detail:    fmt.Sprintf("label %q is not in %v", result.Label, Labels()),
			})
		}

		if len(result.Reasons) == 0 {
			violations = append(violations, Violation{
				Rule:      RuleNoReasons,
				MessageID: result.MessageID,
				Detail:    "a label without evidence is not accepted",
			})
		}

		message, known := byID[result.MessageID]
		if !known {
			violations = append(violations, Violation{
				Rule:      RuleUnknownMessage,
				MessageID: result.MessageID,
				Detail:    "no such message in the batch",
			})
			continue
		}

		for _, reason := range result.Reasons {
			violations = append(violations, checkReason(reason, message, result.MessageID)...)
		}
	}

	for _, message := range messages {
		if !seen[message.ID] {
			violations = append(violations, Violation{
				Rule:      RuleMissingResult,
				MessageID: message.ID,
				Detail:    "message was not classified",
			})
		}
	}

	if len(violations) == 0 {
		return nil
	}
	return &BatchError{Violations: violations}
}

func checkReason(reason Reason, message Message, messageID string) []Violation {
	var violations []Violation

	if !isKnownKind(reason.Kind) {
		violations = append(violations, Violation{
			Rule:      RuleUnknownKind,
			MessageID: messageID,
			Detail:    fmt.Sprintf("reason kind %q is not in %v", reason.Kind, Kinds()),
		})
	}

	quote := foldWhitespace(reason.Quote)
	if quote == "" {
		return append(violations, Violation{
			Rule:      RuleEmptyQuote,
			MessageID: messageID,
			Detail:    fmt.Sprintf("reason kind %q carries no quote", reason.Kind),
		})
	}

	if !quoteAppears(message, quote) {
		violations = append(violations, Violation{
			Rule:      RuleQuoteNotFound,
			MessageID: messageID,
			Detail:    fmt.Sprintf("quote %q is not in the subject or the body", quote),
		})
	}

	return violations
}

// quoteAppears reports whether a folded quote is a literal substring of the
// folded subject or body. Comparison stays case-sensitive: folding case would
// widen the gap between the message and the evidence, and models reproduce case
// reliably when they quote.
func quoteAppears(message Message, foldedQuote string) bool {
	return containsFolded(message.Subject, foldedQuote) ||
		containsFolded(message.Body, foldedQuote)
}

func containsFolded(text, foldedQuote string) bool {
	return len(foldedQuote) > 0 && strings.Contains(foldWhitespace(text), foldedQuote)
}

// foldWhitespace collapses every run of Unicode whitespace to a single space and
// trims the ends. Models reproduce words reliably but mangle line breaks and
// indentation, so folding buys tolerance where it costs nothing and keeps the
// comparison strict on case and wording.
func foldWhitespace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func isKnownLabel(label Label) bool {
	for _, known := range Labels() {
		if label == known {
			return true
		}
	}
	return false
}

func isKnownKind(kind Kind) bool {
	for _, known := range Kinds() {
		if kind == known {
			return true
		}
	}
	return false
}
