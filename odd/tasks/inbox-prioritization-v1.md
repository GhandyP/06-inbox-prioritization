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
| T0 | ODD tracking: this document | S2, S5 | inline | recorded below |
| T1 | Reset: drop the inherited prototype, MIT LICENSE, `.gitignore`, own `go.mod`, `main.go` that compiles without `internal/adkhelper` | S1, S2, S5 | inline | recorded below |
| T2 | Contract + validation in `internal/priority` (closed label set, ID closure, literal-quote check, typed errors) with RED tests | S7, S9, S10 | inline | pending |
| T3 | Deterministic ordering in `internal/priority` (`labelRank` -> `receivedAt` -> `messageID`, reproducible ties) with RED tests | S8 | inline | pending |
| T4 | `internal/inbox`: `Message` + `Reader` interface + JSON fixture adapter + edge-case fixtures in `testdata/` with RED tests | S9, S10 | inline | pending |
| T5 | `internal/inbox/gmail`: OAuth installed-app adapter, single `gmail.readonly` scope, token cached outside the repo, scope test | S4, S6, S11 | inline + verifier | pending |
| T6 | `internal/classifier`: schema-constrained ADK Go v2 agent, one session per batch, no mutating tools, with RED tests | S7, S9 | inline | pending |
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
