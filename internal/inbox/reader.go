// Package inbox reads messages from a source without ever writing to it.
//
// The only way to get messages out of this package is to read them. There is no
// method that could archive, delete, reply to, label, or mark a message, and no
// scope granted anywhere in this repository would allow one.
package inbox

import (
	"context"

	"github.com/GhandyP/06-inbox-prioritization/internal/priority"
)

// Reader supplies the messages to prioritize.
//
// Implementations must be read-only. The interface is deliberately narrow: a
// mutation would have to arrive as a new method, which is a visible change
// rather than something an adapter can slip in.
type Reader interface {
	Messages(ctx context.Context) ([]priority.Message, error)
}
