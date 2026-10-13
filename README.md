# 06 — Prioritize the inbox

## Goal

Classify supplied messages by urgency and explain the evidence for each priority without archiving, replying to, or deleting messages.

## Run the current prototype

From this case directory:

```bash
export GOOGLE_API_KEY="your-key"
go run .
```

`internal/adkhelper.Run` uses ADK Go v2.2.0, Gemini, and `gemini-flash-latest`; set `ADK_MODEL` to override the model.

Sample prompt:

```text
Prioritize these five messages and explain each label.
```

## What exists now

- `main.go (exists)` invokes one focused Gemini ADK agent with user-supplied message text.
- The response has no stable message IDs, closed label set, score, evidence span, or deterministic ordering.
- This is intentionally text-only: it does not read an inbox or perform any mailbox action.

## Missing elements

- **Contract:** Define `MessageBatch{messages[]}` -> `PriorityResult{messageID, label, score, reasons[], sortKey}[]`, with labels such as `urgent`, `normal`, and `low`.
- **ADK layer:** Add a schema-constrained classifier and a session per batch; use a future `list_inbox` function tool only for read access.
- **Domain boundary:** Add inbox-reader and timezone/business-calendar adapters, plus a deterministic sorter that handles ties.
- **Fixtures and validation:** Include deadline language, VIP context, newsletters, duplicates, and empty batches; test label closure and stable ordering.
- **Safety:** Keep archive, delete, and reply tools absent; require human approval before any future mailbox mutation.

## Target structure

```text
06-inbox-prioritization/
├── main.go (exists)
├── README.md (exists)
├── schema.go (next)
├── classifier.go (next)
├── service.go (next)
├── testdata/messages.json (next)
└── main_test.go (next)
```

## Implementation order

1. Define message IDs, urgency labels, scores, reasons, and tie rules.
2. Add schema validation and a batch session around the classifier.
3. Implement deterministic sorting and optional inbox-reader boundaries.
4. Test deadlines, duplicates, newsletters, and missing metadata.
5. Add only a dry-run mailbox action after a human reviews priorities.

## Done when

- [ ] Every input message appears exactly once with a valid label and reason.
- [ ] Equal scores sort deterministically and malformed output is rejected.
- [ ] Fixtures cover urgent, routine, noisy, duplicate, and empty inputs.
- [ ] No archive, delete, or reply occurs without explicit human approval.

## Next improvement

Replace free-form labels with a closed `PriorityResult` schema and deterministic tie-breaking.
