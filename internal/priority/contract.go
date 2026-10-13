// Package priority holds the classification contract for an inbox batch: the
// closed label and evidence vocabulary, and the validation that decides whether
// a batch of results may be trusted.
//
// Nothing in this package performs I/O and nothing in it calls a model. The
// classifier proposes; this package decides. That split is what keeps the
// ordering reproducible and the tests offline.
package priority

import (
	"fmt"
	"strings"
	"time"
)

// Label is the closed priority vocabulary. The set answers "when should this be
// dealt with", which is what prioritizing an inbox means; a severity scale such
// as urgent/normal/low would answer a different question.
type Label string

const (
	Urgent Label = "urgent"
	Today  Label = "today"
	Later  Label = "later"
	Ignore Label = "ignore"
)

// Labels returns the closed set in descending priority order. A fresh slice is
// returned on every call so a caller cannot mutate the vocabulary.
func Labels() []Label {
	return []Label{Urgent, Today, Later, Ignore}
}

// Kind names why a message got its label. The set is closed so the classifier
// cannot invent a justification category.
type Kind string

const (
	KindDeadlineLanguage Kind = "deadline_language"
	KindVipSender        Kind = "vip_sender"
	KindDirectQuestion   Kind = "direct_question"
	KindBulkMail         Kind = "bulk_mail"
	KindNoActionNeeded   Kind = "no_action_needed"
)

// Kinds returns the closed set of reason kinds in canonical order.
func Kinds() []Kind {
	return []Kind{
		KindDeadlineLanguage,
		KindVipSender,
		KindDirectQuestion,
		KindBulkMail,
		KindNoActionNeeded,
	}
}

// Message is one message to prioritize. ReceivedAt is supplied by the reader and
// is never inferred by the classifier.
type Message struct {
	ID         string
	Sender     string
	Subject    string
	Body       string
	ReceivedAt time.Time
}

// Reason is one piece of evidence for a label.
//
// Quote must be a literal substring of the message subject or body, compared
// after runs of whitespace are folded to single spaces. That makes evidence
// checkable instead of merely plausible. It proves provenance, not entailment:
// validation can prove the quoted words exist in the message, and it cannot
// prove those words justify the label. That judgement stays with the reviewer,
// which is why the output prints the quote next to every label.
type Reason struct {
	Kind  Kind
	Quote string
}

// Result is the classification of exactly one message.
type Result struct {
	MessageID string
	Label     Label
	Reasons   []Reason
}

// Rule identifies which contract rule a violation broke.
type Rule string

const (
	RuleUnknownLabel    Rule = "unknown_label"
	RuleUnknownKind     Rule = "unknown_kind"
	RuleDuplicateResult Rule = "duplicate_result"
	RuleUnknownMessage  Rule = "unknown_message"
	RuleMissingResult   Rule = "missing_result"
	RuleNoReasons       Rule = "no_reasons"
	RuleEmptyQuote      Rule = "empty_quote"
	RuleQuoteNotFound   Rule = "quote_not_found"
)

// Violation is one broken contract rule, named against one message.
type Violation struct {
	Rule      Rule
	MessageID string
	Detail    string
}

func (v Violation) String() string {
	return fmt.Sprintf("%s: message %q: %s", v.Rule, v.MessageID, v.Detail)
}

// BatchError carries every violation found in one validation pass, so a caller
// sees every problem in a single run instead of one problem per run.
type BatchError struct {
	Violations []Violation
}

func (e *BatchError) Error() string {
	summaries := make([]string, 0, len(e.Violations))
	for _, v := range e.Violations {
		summaries = append(summaries, v.String())
	}
	noun := "violations"
	if len(e.Violations) == 1 {
		noun = "violation"
	}
	return fmt.Sprintf("priority: %d contract %s: %s", len(e.Violations), noun, strings.Join(summaries, "; "))
}
