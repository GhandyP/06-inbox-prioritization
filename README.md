# 06-inbox-prioritization

Prioritize an inbox: classify supplied messages by urgency and explain the evidence for each
priority, without archiving, replying to, or deleting messages.

Built on [Google ADK for Go](https://github.com/google/adk-go) v2 and Gemini.

> **Status: under construction.** The inherited prototype was removed and this repository is being
> rebuilt from scratch as a standalone project. See
> [`odd/tasks/inbox-prioritization-v1.md`](odd/tasks/inbox-prioritization-v1.md) for the plan, the
> specs, and the current task.

## Why the previous version did not run

The earlier README told you to run `go run .`. That did not work, and it is worth recording why:

- `main.go` imported `github.com/GhandyP/adk-go-enterprise-cookbook/internal/adkhelper`
- that package lives in a **private** repository, and the public import path does not resolve
- the repository had no `go.mod` at all

The prototype was copied out of its parent project without its module scaffold. This rewrite keeps
the project self-contained so the same thing cannot happen again.

## Run

Not available yet. The CLI is wired by T7; until then the binary reports that it is not wired.

## License

MIT. See [LICENSE](LICENSE).
