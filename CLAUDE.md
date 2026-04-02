# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is Lynix

Lynix is a TUI-first CLI tool for API testing and functional validation. Users define HTTP requests in YAML collections, run them against environments, and assert on responses (status, latency, JSONPath, JSON Schema). Single Go binary, no accounts or external services.

## Common Commands

```bash
make build              # Build binary → bin/lynix
make dev                # Run TUI in dev mode (go run with ldflags)
make test               # go test -race ./...
make test-coverage      # Tests + coverage report
make lint               # golangci-lint
make check              # lint + test (pre-PR gate)
make fmt                # gofmt
make tidy               # go mod tidy
make vulncheck          # Check for known CVEs
make release-dry-run    # goreleaser snapshot build
```

Run a single package or test:
```bash
go test ./internal/usecase/assert/...
go test ./internal/usecase/assert/... -run TestEvaluate_JSONPathEq_Pass
```

## Architecture

Hexagonal architecture (ports & adapters). **Core rule: `domain` never imports `infra`; use cases depend only on `ports`.**

- **`internal/domain/`** — Pure domain types with zero external dependencies. Collection, Environment, RunResult, RequestResult, VarResolver, error classification.
- **`internal/ports/`** — Interface definitions (CollectionLoader, EnvironmentLoader, RequestRunner, ArtifactStore, EnvironmentCatalog, WorkspaceLocator).
- **`internal/usecase/`** — Application orchestration. RunCollection (load → resolve vars → execute → assert → extract → store), ValidateCollection, InitWorkspace. Sub-packages: `assert/` (status, latency, JSONPath, JSON Schema checks), `extract/` (JSONPath variable extraction).
- **`internal/infra/`** — Adapter implementations: `yamlcollection/`, `yamlenv/`, `httpclient/`, `httprunner/`, `redaction/`, `runstore/`, `fsworkspace/`, `workspacefinder/`, `curlparse/`, `postmanparse/`, `logger/`. Shared wiring in `wiring/wiring.go` (`NewAdapters()`).
- **`internal/cli/`** — Cobra commands (run, validate, init, collections list, envs list). JUnit XML report generation in `format_junit.go`.
- **`internal/ui/tui/`** — Bubble Tea TUI with wizard-style step orchestration.
- **`cmd/lynix/`** — Thin binary entrypoint.

## Build Requirements

- Go 1.22+
- CI runs `go mod tidy` + `git diff --exit-code` — always run `make tidy` before committing if you change dependencies.

## Key Conventions

- **Commit messages:** Conventional Commits — `<type>(<scope>): <description>`. Types: feat, fix, test, refactor, docs, chore. Scopes: assert, cli, tui, domain, infra, ci, config.
- **Branch names:** `<type>/<short-description>` off `main`.
- **Testing:** All tests run with `-race`. CI enforces 50% coverage threshold. Test names follow `TestFunctionName_Scenario_Expected`.
- **Linting:** golangci-lint with govet, errcheck, staticcheck, ineffassign, gosimple, revive, gofmt, goimports, errorlint, bodyclose. Config in `.golangci.yml`. `errcheck` is disabled for test files.
- **Error handling:** Explicit error returns, no panics in production. Sentinel errors defined in `domain/errors.go`. Runtime errors classified via `ClassifyRunError()` into UI-friendly kinds (dns, connection, timeout, http, unknown).
- **Redaction:** Sensitive data (Authorization, Cookie, token/secret/password/api-key fields) masked before artifact storage. Configured per-surface (header, body, query).
- **Variable resolution priority:** secrets.local.yaml > environment YAML > collection vars > built-ins ({{$uuid}}, {{$timestamp}}).
- **Build metadata:** Version, commit, date injected via ldflags in `internal/buildinfo/`.
