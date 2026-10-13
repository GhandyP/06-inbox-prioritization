# inbox-prioritization-v1 — standalone ADK inbox prioritizer

Public Go project: one Google ADK (Go v2) agent reads messages from Gmail (read-only) or a local
JSON fixture, classifies each one into a closed priority label with cited evidence, and returns a
deterministic ordering. Mailbox mutations stay out of v1 and out of the code.

This is a **rewrite from scratch**, not a port. The prototype copied from the private cookbook
(`internal/adkhelper` import, no `go.mod`) is discarded; its README and `main.go` are input, not spec.

Repo: `github.com/GhandyP/06-inbox-prioritization` (public, MIT). Work lands on `main`, one
work-unit commit per task.

## Specs

S1 — user (verbatim): "Dejar 06-inbox-prioritization". Repo name and Go module path stay
`06-inbox-prioritization` / `github.com/GhandyP/06-inbox-prioritization`.

S2 — user (verbatim): "la idea es volver a hacer el proyecto desde el crincipio de forma
independiente para que sea su propio proyecto usando odd". Full reset: the cookbook README and
`main.go` carry no authority over the new contract.

S3 — user (verbatim): "vuelve a comenzar". No inherited contract. The label set and the
date policy are designed fresh, so the disagreement between `guia-80-casos.md`, the case README,
and `main.go` is moot.

S4 — user (verbatim): "Sumar lectura Gmail read-only". v1 includes a read-only Gmail adapter.

S5 — user (verbatim): "Sí, arrancá con T1".

S6 — user (verbatim): "No todavía" (no Google Cloud OAuth client ID yet). T5 ships the adapter and
its documentation; the real OAuth flow is not exercised in this session.

S7 — design decision, authorized by S3. Closed label set, quoted verbatim from
`guia-80-casos.md:323-327`: labels `urgent|today|later|ignore`. Rationale: the set answers *when to
act*, which is what prioritizing an inbox means; `urgent|normal|low` measures severity instead.

S8 — design decision, authorized by S3. No model-generated `score`. An LLM score is not
reproducible, and the case requires deterministic ordering. Order derives from data already present
in the message: `labelRank(urgent=0, today=1, later=2, ignore=3)` -> `receivedAt` -> `messageID`.

S9 — design decision, authorized by S3. `Reason{Kind, Quote}`, where `Quote` must be a literal
substring of the message subject or body. No inferred due date and no `due_date` field. This proves
provenance, not entailment.

S10 — project goal, quoted verbatim from the project's own README: "Classify supplied messages by
urgency and explain the evidence for each priority without archiving, replying to, or deleting
messages."

S11 — non-goal, quoted verbatim from the project's own README: "Keep archive, delete, and reply
tools absent; require human approval before any future mailbox mutation." Hard rule: zero write
scopes. `gmail.readonly` is a single constant with a test; `gmail.modify` and `gmail.send` must not
appear anywhere in the tree.

Non-goals (v1): no mailbox mutation of any kind, no write scopes, no inferred due dates, no
model-generated score, no Gmail push/watch, no attachment parsing, no multi-account support.

Adjacent cookbook cases deliberately out of scope: `19-message-urgency-detection` (urgency
detection as such), `07-task-extraction` (turning deadlines into tasks),
`08-automated-email-follow-ups` (sending). This project only orders a supplied batch.

## Tasks

Commit identities are recorded as evidence once each task lands; they are never predicted.

| ID | Task | Specs | Route | Commit |
| --- | --- | --- | --- | --- |
| T0 | ODD tracking: this document | S2, S5 | inline | `52360ad` |
| T1 | Reset: drop the inherited prototype, MIT LICENSE, `.gitignore`, own `go.mod`, `main.go` that compiles without `internal/adkhelper` | S1, S2, S5 | inline | `3808223` |
| T2 | Contract + validation in `internal/priority` (closed label set, ID closure, literal-quote check, typed errors) with RED tests | S7, S9, S10 | inline | `ba05773` |
| T3 | Deterministic ordering in `internal/priority` (`labelRank` -> `receivedAt` -> `messageID`, reproducible ties) with RED tests | S8 | inline | `fdc21bc` |
| T4 | `internal/inbox`: `Message` + `Reader` interface + JSON fixture adapter + edge-case fixtures in `testdata/` with RED tests | S9, S10 | inline | `7fb75b6` |
| T5 | `internal/inbox/gmail`: OAuth installed-app adapter, single `gmail.readonly` scope, token cached outside the repo, scope test | S4, S6, S11 | inline + verifier | `7f8b3c3`, `9e54ae9` |
| T6 | `internal/classifier`: schema-constrained ADK Go v2 agent, one session per batch, no mutating tools, with RED tests | S7, S9 | inline | `7859490` |
| T7 | `internal/cli`: flags, wiring, text and JSON output, exit codes, with RED tests | S10 | inline | pending |
| T8 | Honest README rewrite, Makefile, CI workflow (build, vet, gofmt, test), CHANGELOG | S2, S11 | inline | pending |
| T9 | Independent verification of the whole candidate | S1-S11 | verifier | pending |

Every task closes with at least one work-unit commit on `main`. Tests and docs land alongside the
behavior they describe. Push and pull requests stay a user decision.

## Log

L1 — user (verbatim): "ok mira igual que antes es ver lo que se tiene y preparar un plan para poder hacer este proyecto con google adk para montarlo en mi github, primero que todo crea el repo publico de este proyecto y luego es lo demas"

L2 — exploration evidence (2026-10-13). The public repo **already existed**:
`gh repo view GhandyP/06-inbox-prioritization` reported `visibility: PUBLIC`, created
`2026-10-13T14:37:23Z`; local `main` equalled `origin/main` at `adda593`. Nothing was created.

L3 — exploration evidence. The prototype could not build. `main.go` imported
`github.com/GhandyP/adk-go-enterprise-cookbook/internal/adkhelper`, that package lives in
`/home/ghandy/Documents/P1_proyecticos/adk-go-enterprise-cookbook` whose remote is
`GhandyP/proyecticos` (private), `github.com/GhandyP/adk-go-enterprise-cookbook` does not resolve on
GitHub, and the repo had no `go.mod`. So the README section "Run the current prototype" was false.

L4 — exploration evidence. Three sources disagreed on the contract: `guia-80-casos.md:323-327`
asked for `urgent|today|later|ignore` plus `suggested_due_date`; the case README asked for
`urgent|normal|low` plus `score` and `sortKey`; and the live `main.go` instruction said
"urgent, today, later, or ignore" together with "do not invent due dates".

L5 — user answers (verbatim): "Dejar 06-inbox-prioritization"; "la idea es volver a hacer el
proyecto desde el crincipio de forma independiente para que sea su propio proyecto usando odd";
"vuelve a comenzar"; "Sumar lectura Gmail read-only"; "Sí, arrancá con T1"; "No todavía".

L6 — decision (from L5). Full rewrite, not a port. That makes L4 moot: the label set and date policy
are designed fresh under S7/S8/S9 instead of reconciled. Gmail read-only promotes this project to
high risk item 2 (credentials, OAuth, scopes), so T9 independent verification is mandatory and not
skippable.

L7 — evidence (pre-T1). `go mod init github.com/GhandyP/06-inbox-prioritization` succeeded and the
resulting module built cleanly under go1.26.4, so the numeric path segment is not an obstacle and S1
is technically sound. Module proxy confirms `google.golang.org/adk/v2` up to `v2.6.0`; `02` pins
`v2.2.0` and `04` moved to `v2.6.0`.

L8 — decision (T1). Dependencies arrive with the code that first imports them, so every commit stays
`go mod tidy`-clean: T1 establishes a dependency-free module, the Gmail and OAuth libraries land at
T5, and `google.golang.org/adk/v2` is pinned at T6 once `internal/classifier` imports it. `go.sum`
therefore appears at T5, not at T1.

L9 — decision (T1). Work lands directly on `main`, matching the user's established preference for
this project series ("Directo a main", recorded in `04/odd/tasks/adk-meeting-summary-v1.md` L3) and
the practice of both published siblings, `02-professional-email-drafting` and
`04-meeting-transcription-summary`. This overrides the general branch-first-on-default guidance
because a prior explicit user decision covers this series. Commits stay **local**: push and pull
request creation remain user decisions and were not requested.

L10 — decision (T1). The `go` directive is pinned to `1.26.6` up front rather than raised later. A
probe showed `google.golang.org/adk/v2@v2.6.0` requires `go >= 1.26.6`, so declaring the floor at T1
avoids a mid-project toolchain move and keeps `go-version-file: go.mod` stable in CI. The local
toolchain is go1.26.4 and auto-switches to the required toolchain, which was verified to work.

L11 — evidence (T1). Commit `3808223` on `main`. Checks: `go build ./...` exit 0, `go vet ./...`
exit 0, `gofmt -l .` clean, `go list -m` reports `go=1.26.6`, tracked files are `.gitignore`,
`LICENSE`, `README.md`, `go.mod`, `main.go`, `odd/tasks/inbox-prioritization-v1.md`. `go run .`
prints "06-inbox-prioritization: not wired yet; see odd/tasks/inbox-prioritization-v1.md" and exits
non-zero, so no commit claims a working CLI before T7. The old `README.md` claim that `go run .`
worked is replaced by a section documenting why it could not.

L12 — evidence (T2). Commit `ba05773`. RED was observed as
`no non-test Go files in .../internal/priority [build failed]`, exit 1. GREEN as
`ok github.com/GhandyP/06-inbox-prioritization/internal/priority` with 17 test functions; `go build
./...`, `go vet ./...` and `gofmt -l .` all clean. Two tests were tightened before GREEN rather than
after: the case-sensitivity test now changes exactly one quote and asserts a single violation, and
the `BatchError` test now asserts the offending message ID appears in the rendered error, so its
name is honest. Whitespace folding was chosen over byte-exact comparison and case comparison was
kept strict, so the rule tolerates mangled line breaks and refuses reworded evidence.

L13 — decision (T2/T3). `Labels()` and `Kinds()` return fresh slices so no caller can mutate the
closed vocabularies. `Label.Rank()` returns the index in `Labels()` and `-1` for an unknown label,
which is a query rather than a sort key; `Order` therefore maps an unknown label to `len(Labels())`
so unknown labels sort last instead of first.

L14 — evidence (T3). Commit `fdc21bc`. RED was observed as ten build errors
(`label.Rank undefined`, `undefined: priority.Order`) and exit 1. GREEN as
`ok github.com/GhandyP/06-inbox-prioritization/internal/priority` with 27 test functions passing
under `-count=1`; `go build ./...`, `go vet ./...` and `gofmt -l .` all clean. `Order` uses
`slices.SortStableFunc` with a `cmp.Compare` key so the comparison is total and the function never
mutates its input, which is asserted by a test.

L15 — evidence (T4). Commit `7fb75b6`. RED was observed as
`no non-test Go files in .../internal/inbox [build failed]`, exit 1. GREEN as
`ok github.com/GhandyP/06-inbox-prioritization/internal/inbox` with 13 test functions, 40 passing
across the module under `-count=1`; `go build ./...`, `go vet ./...` and `gofmt -l .` all clean. Two
sloppy helpers and a meaningless `errors.Is(err, err)` were removed from the test file before GREEN
rather than committed. Committed fixtures are `testdata/messages_ok.json`,
`testdata/messages_empty.json` and `testdata/messages_missing_metadata.json`; malformed-input cases
are written to `t.TempDir()` instead of being committed, and `TestCommittedFixturesParse` guards the
committed ones against rot. `priority.Message`, `Reason` and `Result` gained `json` tags here
because the fixture and the T7 output both need them.

L16 — evidence (T5). Commit `7f8b3c3`. RED was observed as
`no non-test Go files in .../internal/inbox/gmail [build failed]`, exit 1. A setup failure followed:
after `go get google.golang.org/api@latest golang.org/x/oauth2@latest` the build reported 16
`missing go.sum entry` errors, because `go get` records requirements without the transitive sums;
`go mod tidy` resolved it. GREEN: 11 test functions in `internal/inbox/gmail`, 51 passing across the
module. `go build ./...` exit 0, `go vet ./...` exit 0 (5m09s cold, moved to the background after a
600s synchronous timeout), `gofmt -l .` clean. Dead code in the fake-server handler and a leftover
unused sentinel were removed from the test file before GREEN rather than committed.

L17 — environment finding (T5), important. `/home/ghandy` is itself a Git working tree:
`/home/ghandy/.git` exists, and `git rev-parse --show-toplevel` run from `/home/ghandy/.config`
returns `/home/ghandy`. The same finding is recorded in `04/odd/tasks/adk-meeting-summary-v1.md` L4.
Two consequences: every `$HOME`-based path fails `CheckTokenPath`, so the
`gmail.DefaultTokenPath()` default is unusable on this machine and `Config.TokenPath` has to be set
explicitly; and a `git add -A` run in `/home/ghandy` could sweep up anything beneath it. The guard
was kept strict rather than relaxed, because a false refusal costs a confusing error while a false
accept costs a published credential. `TestDefaultTokenPathIsAbsoluteAndNamed` therefore asserts the
shape of the default and logs the situation instead of asserting guard compliance, so the test does
not depend on the developer's home directory.

L18 — decision (T5). `authorizeWithBrowser`, the installed-app loopback flow, is the single path the
test suite cannot exercise, which is what S6 accepted. It is reported as unverified rather than
counted as covered. Every other path in T5 is exercised, including both scope guards and the token
path guard, which is where the risk actually sits.

L19 — user decisions (verbatim): "Acotar al repo del proyecto" and "Sí, pushear a origin/main".
The first narrows `CheckTokenPath` to the repository the working directory belongs to; the second
pushes six commits. The user first declined to read the questionnaire, so it was re-asked with shorter
labels.

L20 — environment finding (correction of L17). `/home/ghandy/.git` is **not** a user repository. It
holds `engram-project-identity.json`, has zero commits, no remotes and three index entries, and was
created on 2026-09-24: it is a project-identity marker left by the memory provider. It must not be
deleted without checking first, because Engram's project detection depends on it. The guard was
narrowed in response, not the marker removed.

L21 — evidence (T5 revision and first push). Commit `9e54ae9` on `main`. The guard walk now starts at
the working directory instead of at the token path, so an unrelated enclosing working tree no longer
rejects every path under it; a new test asserts that acceptance, so narrowing further would need a
test change too. 12 test functions in `internal/inbox/gmail`, 52 across the module, all green under
`-count=1`; `go build ./...` exit 0 and `gofmt -l .` clean. `git push origin main` advanced
`adda593..9e54ae9`; `git ls-remote origin` and the local `HEAD` both report
`9e54ae91ecf38cf4d5c890c010603ff7b04d5106`, and `gh repo view` reports `PUBLIC` with
`pushedAt 2026-10-13T17:58:01Z`. This is the first push; every earlier commit was local only.

L22 — the memory provider was unavailable from T5 onward: every memory call timed out against
`http://127.0.0.1:7437`, including after the server accepted the connection. This document is
therefore the only durable record of T1-T5, and the standard memory mirror was not written. The
failure is reported, not retried into extra ceremony.

L23 — evidence (T6). Commit `7859490`. The ADK Go v2 API was mapped first by a read-only exploration
over the module cache at `~/go/pkg/mod/google.golang.org/adk/v2@v2.6.0`. That is what found
`llmagent.Config.OutputSchema` and confirmed that a nil `Tools` registers no tool at all, while
`ModeTask` would add a `finish_task` tool. RED was observed twice: first as
`missing go.sum entry for ... google.golang.org/genai` (a setup failure, fixed by `go mod tidy`), then
as the expected build failure. GREEN: 16 test functions in `internal/classifier`, 68 across the module
(27 priority + 13 inbox + 12 gmail + 16 classifier). `go build ./...` exit 0, `go vet ./...` exit 0
(2m34s, backgrounded), `gofmt -l .` clean. `google.golang.org/adk/v2` is pinned at v2.6.0 and is now a
direct requirement; it is the version whose `go >= 1.26.6` floor L10 pinned at T1. Three assertions
were corrected before GREEN for a real type error: `priority.Rule` is not `string`, so
`strings.Contains` needed an explicit conversion.

L24 — decision (T6). `Responder` is exported even though `ADKResponder` is the only real
implementation, because it is the seam that keeps the suite offline. Keeping it unexported and testing
through an internal file would have hidden the contract from callers and made the agent untestable
from outside the package.
