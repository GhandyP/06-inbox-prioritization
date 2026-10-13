// Package cli wires a reader, the classifier and the contract into one command.
//
// Run takes the command line and two writers and returns an exit code, so the
// whole command is exercised through a single entry point without a model, a
// credential, or a network.
//
// Nothing here can act on a mailbox. The only source it can open is a read-only
// Reader, and the only thing it writes is a report.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/GhandyP/06-inbox-prioritization/internal/priority"
)

// Exit codes.
const (
	ExitOK    = 0
	ExitError = 1
	ExitUsage = 2
)

// defaultLimit is the cap used when -limit is not given.
const defaultLimit = 25

// Options are the parsed flags.
type Options struct {
	// Input is the path to a JSON fixture. It and Gmail are exclusive.
	Input string

	// Gmail selects the read-only Gmail reader instead of a fixture.
	Gmail bool

	// Query narrows the Gmail read.
	Query string

	// Limit caps how many messages are read.
	Limit int

	// Model overrides the classifier model.
	Model string

	// JSON emits JSON instead of text.
	JSON bool
}

// Deps are the collaborators the command needs.
//
// Both fields are required. main builds the real ones; tests supply offline ones,
// which is what keeps the suite free of credentials and network calls.
type Deps struct {
	// Read returns the messages for one run.
	Read func(ctx context.Context, opts Options) ([]priority.Message, error)

	// Classify proposes results for a batch.
	Classify func(ctx context.Context, opts Options, messages []priority.Message) ([]priority.Result, error)
}

// Run executes one command line and returns the exit code.
//
// The report is written only after the batch has passed the contract, so a
// caller never sees a label that failed validation.
func Run(ctx context.Context, args []string, deps Deps, stdout, stderr io.Writer) int {
	options, err := parse(args, stdout)
	switch {
	case errors.Is(err, flag.ErrHelp):
		return ExitOK
	case err != nil:
		fmt.Fprintf(stderr, "%v\n\n%s", err, usage())
		return ExitUsage
	}
	if err := check(options); err != nil {
		fmt.Fprintf(stderr, "%v\n\n%s", err, usage())
		return ExitUsage
	}

	messages, err := deps.Read(ctx, options)
	if err != nil {
		fmt.Fprintf(stderr, "read the messages: %v\n", err)
		return ExitError
	}

	// An empty batch never reaches the model: there is nothing to classify, and
	// a call would cost a request and a session for no result.
	if len(messages) == 0 {
		if err := reportEmpty(stdout, options); err != nil {
			fmt.Fprintf(stderr, "write the report: %v\n", err)
			return ExitError
		}
		return ExitOK
	}

	results, err := deps.Classify(ctx, options, messages)
	if err != nil {
		fmt.Fprintf(stderr, "classify the batch: %v\n", err)
		return ExitError
	}

	if err := render(stdout, options, messages, results); err != nil {
		fmt.Fprintf(stderr, "write the report: %v\n", err)
		return ExitError
	}
	return ExitOK
}

func parse(args []string, stdout io.Writer) (Options, error) {
	var options Options

	flags := flag.NewFlagSet("06-inbox-prioritization", flag.ContinueOnError)
	// The flag package must not print on its own: help belongs on stdout and
	// failures belong on stderr, and only Run knows which is which.
	flags.SetOutput(io.Discard)
	flags.Usage = func() {}

	flags.StringVar(&options.Input, "input", "", "path to a JSON fixture of messages (offline source)")
	flags.BoolVar(&options.Gmail, "gmail", false, "read from Gmail instead of a fixture (read-only)")
	flags.StringVar(&options.Query, "query", "", "Gmail search query, for example 'is:unread' (gmail only)")
	flags.IntVar(&options.Limit, "limit", defaultLimit, "read at most this many messages")
	flags.StringVar(&options.Model, "model", "", "model name; defaults to ADK_MODEL or gemini-flash-latest")
	flags.BoolVar(&options.JSON, "json", false, "emit JSON instead of a text report")

	showHelp := func(w io.Writer) {
		fmt.Fprint(w, "Prioritize an inbox without touching it.\n\n")
		fmt.Fprint(w, "Classifies every message into a closed priority label and prints the quoted\n")
		fmt.Fprint(w, "evidence behind each label. It never archives, replies to, or deletes anything.\n\n")
		fmt.Fprintf(w, "Usage:\n  %s -input <file> [flags]\n  %s -gmail [flags]\n\n", flags.Name(), flags.Name())
		fmt.Fprint(w, "Flags:\n")

		previous := flags.Output()
		flags.SetOutput(w)
		flags.PrintDefaults()
		flags.SetOutput(previous)

		fmt.Fprint(w, "\nExactly one source is required: -input or -gmail.\n")
	}

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			showHelp(stdout)
			return Options{}, flag.ErrHelp
		}
		return Options{}, err
	}
	if flags.NArg() > 0 {
		return Options{}, fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	return options, nil
}

func check(options Options) error {
	switch {
	case options.Input != "" && options.Gmail:
		return errors.New("choose one source, not both: -input or -gmail")
	case options.Input == "" && !options.Gmail:
		return errors.New("a source is required: pass -input <file> or -gmail")
	}
	if options.Limit < 1 {
		return fmt.Errorf("limit must be at least 1, got %d", options.Limit)
	}
	return nil
}

func usage() string {
	return "Usage:\n  06-inbox-prioritization -input <file> [flags]\n  06-inbox-prioritization -gmail [flags]\n\nRun with -h for the full list of flags.\n"
}

func reportEmpty(w io.Writer, options Options) error {
	if options.JSON {
		return renderJSON(w, nil)
	}
	_, err := fmt.Fprintln(w, "No messages to prioritize.")
	return err
}

func render(w io.Writer, options Options, messages []priority.Message, results []priority.Result) error {
	// The model does not decide the order; the contract does.
	ordered := priority.Order(results, messages)
	if options.JSON {
		return renderJSON(w, ordered)
	}
	return renderText(w, messages, ordered)
}

func renderJSON(w io.Writer, results []priority.Result) error {
	if results == nil {
		results = []priority.Result{}
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(map[string]any{"results": results})
}

// renderText groups results by label in priority order and prints each message
// with the quote that justified its label, so a reviewer can check the claim
// instead of trusting it.
func renderText(w io.Writer, messages []priority.Message, results []priority.Result) error {
	byID := make(map[string]priority.Message, len(messages))
	for _, message := range messages {
		byID[message.ID] = message
	}

	current := priority.Label("")
	for _, result := range results {
		if result.Label != current {
			current = result.Label
			if _, err := fmt.Fprintf(w, "\n%s\n", strings.ToUpper(string(current))); err != nil {
				return err
			}
		}

		message := byID[result.MessageID]
		if _, err := fmt.Fprintf(w, "  %s  %s\n", message.ID, display(message.Subject)); err != nil {
			return err
		}
		line := fmt.Sprintf("      from %s, received %s\n",
			display(message.Sender), message.ReceivedAt.UTC().Format(time.RFC3339))
		if _, err := fmt.Fprint(w, line); err != nil {
			return err
		}

		for _, reason := range result.Reasons {
			if _, err := fmt.Fprintf(w, "      %s: %q\n", reason.Kind, reason.Quote); err != nil {
				return err
			}
		}
	}
	return nil
}

func display(value string) string {
	if strings.TrimSpace(value) == "" {
		return "(unknown)"
	}
	return value
}
