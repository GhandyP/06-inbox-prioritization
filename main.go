// Command 06-inbox-prioritization prioritizes a batch of inbox messages.
//
// It reads from a JSON fixture or from Gmail read-only, asks a schema-constrained
// Google ADK agent for a label and the evidence behind it, and prints the batch
// in deterministic priority order. It never archives, replies to, deletes, or
// marks anything.
//
// See odd/tasks/inbox-prioritization-v1.md for the specs and the task list.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/GhandyP/06-inbox-prioritization/internal/classifier"
	"github.com/GhandyP/06-inbox-prioritization/internal/cli"
	"github.com/GhandyP/06-inbox-prioritization/internal/inbox"
	"github.com/GhandyP/06-inbox-prioritization/internal/inbox/gmail"
	"github.com/GhandyP/06-inbox-prioritization/internal/priority"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli.Run(ctx, os.Args[1:], deps(), os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func deps() cli.Deps {
	return cli.Deps{
		Read:     readMessages,
		Classify: classifyMessages,
	}
}

// readMessages opens the selected source. Both branches are read-only, and
// neither can be constructed without going through a Reader.
func readMessages(ctx context.Context, options cli.Options) ([]priority.Message, error) {
	if options.Gmail {
		client, err := gmail.New(ctx, gmail.Config{
			Query: options.Query,
			Limit: options.Limit,
		})
		if err != nil {
			return nil, err
		}
		return client.Messages(ctx)
	}

	return inbox.Fixture{Path: options.Input}.Messages(ctx)
}

func classifyMessages(ctx context.Context, options cli.Options, messages []priority.Message) ([]priority.Result, error) {
	agent, err := classifier.New(ctx, classifier.Config{ModelName: options.Model})
	if err != nil {
		return nil, err
	}
	return agent.Classify(ctx, messages)
}
