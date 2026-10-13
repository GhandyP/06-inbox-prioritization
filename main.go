// Command 06-inbox-prioritization prioritizes a batch of inbox messages.
//
// The CLI is wired incrementally by the tasks described in
// odd/tasks/inbox-prioritization-v1.md. Until T7 lands, this command reports
// that it is not wired yet instead of pretending to work.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "06-inbox-prioritization: not wired yet; see odd/tasks/inbox-prioritization-v1.md")
	os.Exit(2)
}
