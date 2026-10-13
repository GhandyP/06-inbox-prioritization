# Changelog

All notable changes to this project are recorded here. This project follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- A closed priority contract in `internal/priority`: the labels `urgent`, `today`, `later` and
  `ignore`, the reason kinds, batch validation that reports every violation in one pass, and an
  ordering that depends only on the label rank, `received_at` and the message id.
- An offline JSON fixture reader in `internal/inbox`, which is what lets the whole suite run without
  a credential, a token, or a network connection.
- A read-only Gmail reader in `internal/inbox/gmail`, with a single `gmail.readonly` scope, a guard
  that refuses a token inside this project's repository, and a check of the scope the token was
  actually granted.
- A schema-constrained Google ADK agent in `internal/classifier`, bound with no tools and one session
  per batch, where every batch is validated before a caller sees it.
- A command in `internal/cli` with text and JSON reports, a priority-ordered grouping that quotes the
  evidence under each label, and separated exit codes.

### Changed

- The repository was reset from the inherited prototype, which could not build: it imported a package
  that lives in a private repository and did not resolve publicly, and the module had no `go.mod`.
