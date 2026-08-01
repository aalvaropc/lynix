# Changelog

All notable changes to Lynix are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased] — v0.4.0

### Added
- `{{$env.NAME}}` built-in reads process environment variables (hard error when unset) — inject CI secrets without materializing files.
- Repeatable `--var key=value` on `run` and `validate`, highest precedence; also available as the `vars` input of the GitHub Action.
- `assert.body` block (`eq`, `contains`, `not_contains`, `matches`, `not_matches`) for raw-body assertions on any content type.
- New value operators: `not_matches`, `gte`, `lte`, `len`; `status` accepts a list of accepted codes.
- Assertion expected values resolve `{{var}}` references (compare against previously extracted values).
- Differentiated exit codes: `0` passed · `1` assertion failures · `2` usage/config error · `3` execution error.
- Colored output (TTY-aware, `NO_COLOR`, `--no-color`), `--quiet` mode, response excerpts on failures.
- `lynix runs list|show|diff` — inspect saved artifacts and compare runs (status changes, latency deltas, assertion regressions).
- `run.cookies` (in-memory cookie jar), `run.tls.ca_file` (custom CA trust), `run.max_body_kb` (response cap).
- Artifacts carry `schema_version` and store text bodies as plain text (greppable/diffable).

### Changed
- Artifact JSON is fully snake_case (previously a PascalCase hybrid that matched neither docs nor index).
- Form bodies in artifacts/dry-run are URL-encoded exactly as sent on the wire.
- JUnit reports never double-count a request as both failure and error; failed cases include `<system-out>` with a response excerpt.
- `validate` works standalone (without a workspace) and compiles JSONPath/regex up front.
- Per-request `follow_redirects` overrides `--no-redirects` in both directions; requests send `User-Agent: lynix/<version>`; TLS 1.2 minimum.
- `run.insecure` prints a loud warning on every run.

## [0.3.0] — 2026-08-01

Integrity release: every known way to get a false green or leak a secret, fixed.

### Fixed
- `exists: false` was silently skipped (always passed); it is now a real absence check.
- Unknown YAML keys (e.g. a typo like `assrt:`) were silently ignored, dropping assertions; all loaders are strict now.
- With `--only`/`--tags`, JSON Schemas could validate the wrong request or not run at all (cache indexed pre-filter).
- Filters matching zero requests reported a green run that executed nothing; now an error.
- Requests interrupted in parallel mode vanished from reports; they are recorded with their error.
- Large integer IDs lost precision (float64 decoding) in `eq` comparisons and `extract`.
- Redaction leaks closed: unmasked `url` field, network-error messages embedding the full URL, non-JSON bodies (form logins stored `password=...` verbatim), and assertion messages quoting response values.
- `fail_on_detected_secret` was circular (reused the redactor's own predicates); it now scans serialized artifacts for known secret values and credential formats.
- `timeout_ms` above 30s was silently capped by the HTTP client; `run.timeout_seconds` now bounds the whole run in the CLI.
- The `auth-chain` example chained a non-existent user id and failed for every new user; examples now run in CI.
- Command injection in the GitHub Action (`eval` over interpolated inputs) removed; the action runs its own bundled installer.

### Changed
- The unreachable TUI (~2,300 LOC and the whole Charm dependency tree), dead logger, and orphaned code removed; Lynix is CLI-first for CI/CD.
- Errors print a friendly headline; duplicate request names are rejected.
- Go 1.25; dependencies updated (clears two known CVEs); CI runs an OS/Go matrix, govulncheck, and the examples end-to-end.

## [0.2.0] — 2026-04-07

- Parallel execution (`--parallel`) with dependency-graph scheduling.
- Reusable GitHub Action; artifact rotation (`max_runs`); run options in the TUI wizard.
- README slimmed; reference docs moved to `docs/`.

## [0.1.0] — 2026-03-06

- Initial release: YAML collections, environments and secrets, JSONPath/JSON Schema assertions, variable chaining, artifact storage with masking, TUI wizard, curl/Postman import, JUnit reports, Homebrew tap and install script.
