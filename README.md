# 06-inbox-prioritization

[![CI](https://github.com/GhandyP/06-inbox-prioritization/actions/workflows/ci.yml/badge.svg)](https://github.com/GhandyP/06-inbox-prioritization/actions/workflows/ci.yml)

Prioritize an inbox: classify each message into a closed priority label and print the quoted evidence
behind it — without archiving, replying to, or deleting anything.

A small, complete Go example built on [Google ADK for Go](https://github.com/google/adk-go) v2 and
Gemini. It exists for the two hard parts of an LLM triage feature: keeping the model inside a closed
vocabulary, and keeping a mailbox out of reach.

## The two guarantees

**Evidence you can check.** Every label must come with at least one reason whose quote is a literal
substring of that message's subject or body. Validation rejects a label outside the closed set, a
result for a message that is not in the batch, a message with no result, a result with no reason, and
a quote that does not appear in the message.

This proves provenance, not entailment. It can prove that the quoted words exist in the message, and
it cannot prove that those words justify the label. That judgement stays with the reviewer, which is
why the report prints the quote under every label instead of summarising it away.

**Nothing can be touched.** The Gmail adapter requests exactly one scope,
`https://www.googleapis.com/auth/gmail.readonly`, and a test walks the module and fails if any wider
scope string appears anywhere in it. The ADK agent is bound with no tools, no toolsets and no
subagents, so there is no tool it could call, and a test asserts that this stays true. The reader
interface has one method and nothing that could archive, delete, reply to, label, or mark a message.

## Quick start

Requirements: Go 1.26.6 or newer, and a `GOOGLE_API_KEY` from
[Google AI Studio](https://aistudio.google.com/apikey).

```bash
export GOOGLE_API_KEY="your-key"

# Prioritize the committed sample batch.
go run . -input testdata/messages_ok.json

# The same report as JSON.
go run . -input testdata/messages_ok.json -json
```

The report groups messages by label in priority order and prints the quoted evidence under each one.
The labels below are the model's; this is the shape of the report:

```text
URGENT
  m-001  Invoice 8841 due Friday
      from ana.ruiz@example.com, received 2026-10-13T09:12:00Z
      deadline_language: "due Friday"

IGNORE
  m-002  Ten links you may have missed this week
      from newsletter@vendor.example.com, received 2026-10-13T09:30:00Z
      bulk_mail: "Weekly newsletter"
```

`go run . -input testdata/messages_empty.json` needs no key at all: an empty batch never reaches the
model.

## Reading from Gmail

The reader is read-only. You need an OAuth client of type **Desktop app** from
[Google Cloud](https://console.cloud.google.com/apis/credentials) with the Gmail API enabled, and its
JSON downloaded.

```bash
export GOOGLE_API_KEY="your-key"
go run . -gmail -query "is:unread newer_than:2d" -limit 20
```

The first run prints a consent URL and caches the token under your user configuration directory,
for example `~/.config/06-inbox-prioritization/token.json`. The token file is listed in `.gitignore`,
and `gmail.CheckTokenPath` refuses a token path that falls inside this project's repository, so a
stray `git add -A` cannot publish it.

`gmail.readonly` is the only scope requested. `google.ConfigFromJSON` pins it, and the granted scope
is read back from the token and checked, so a token that was granted more is refused rather than
trusted.

## Priority labels

| Label | Means |
| --- | --- |
| `urgent` | Needs a person now, and the message says why. |
| `today` | Should be dealt with today. |
| `later` | Real, but not today. |
| `ignore` | Nothing here needs action. |

The order is not the model's opinion. Results are sorted by label rank, then by the message's
`received_at`, then by message id, so two runs over the same batch agree.

There is no confidence score on purpose. A model-generated score would not be reproducible, and a
deterministic order is the point.

## Reason kinds

| Kind | Means |
| --- | --- |
| `deadline_language` | The message states or implies a deadline in its own words. |
| `vip_sender` | The sender is one the batch says to treat as a priority. |
| `direct_question` | The message asks the reader something. |
| `bulk_mail` | A newsletter, digest, or other bulk send. |
| `no_action_needed` | Nothing in the message asks for anything. |

## Commands and exit codes

```text
Usage:
  06-inbox-prioritization -input <file> [flags]
  06-inbox-prioritization -gmail [flags]
```

| Flag | Meaning |
| --- | --- |
| `-input` | Path to a JSON fixture of messages. |
| `-gmail` | Read from Gmail instead of a fixture. |
| `-query` | Gmail search query, for example `is:unread`. |
| `-limit` | Read at most this many messages. Defaults to 25. |
| `-model` | Model name. Defaults to `ADK_MODEL` or `gemini-flash-latest`. |
| `-json` | Emit JSON instead of a text report. |

Exactly one of `-input` and `-gmail` is required.

| Exit code | Meaning |
| --- | --- |
| 0 | A report was produced. |
| 1 | Reading or classification failed. Nothing was written to stdout. |
| 2 | The command line was wrong. |

## What this does not do

- It does not archive, delete, reply to, label, or mark anything. No such code path exists, and no
  scope that would permit one is ever requested.
- It does not infer a due date. If a message contains a date, that date appears as quoted evidence;
  nothing computes one.
- It does not prove that a quote justifies its label. See "Evidence you can check" above.
- It does not read attachments, watch the mailbox, or handle more than one account.

## Development

```bash
make check   # formatting, vet and tests: the same three steps CI runs
make test    # tests only
make demo    # run the CLI against the committed fixture
```

The whole suite runs offline. No test needs a credential, a token, or a network connection: the
fixture reader is the path every test takes, and the classifier is exercised through a `Responder`
seam.

## License

MIT. See [LICENSE](LICENSE).
