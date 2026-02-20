# Contributing to Lynix

Thank you for your interest in contributing! This document explains how to set up your environment, run the project, and submit changes.

---

## Table of contents

- [Development environment](#development-environment)
- [Project structure](#project-structure)
- [Running the project](#running-the-project)
- [Running tests](#running-tests)
- [Linting](#linting)
- [Commit conventions](#commit-conventions)
- [Pull request guidelines](#pull-request-guidelines)

---

## Development environment

**Requirements:**

| Tool | Version |
|------|---------|
| Go   | 1.22+   |
| Git  | any     |

Clone the repository and install dependencies:

```bash
git clone https://github.com/aalvaropc/lynix.git
cd lynix
go mod tidy
```

Verify everything builds:

```bash
make build
```

---

## Project structure

Lynix follows **hexagonal architecture** (ports & adapters):

```
internal/
  domain/          # Pure domain types — no external dependencies
  ports/           # Interface definitions (abstraction boundaries)
  usecase/         # Application orchestration (depends on ports, not infra)
  infra/           # Adapter implementations (YAML, HTTP, filesystem, store)
  cli/             # Cobra CLI commands
  ui/tui/          # Bubble Tea TUI
cmd/lynix/         # Entry point
```

**Rule:** `domain` must never import `infra`. Use cases depend only on `ports`.

---

## Running the project

```bash
# Run with TUI (default)
make dev

# Run a specific CLI subcommand
go run ./cmd/lynix run --collection demo --workspace /path/to/ws

# Build binary
make build           # output: bin/lynix
```

---

## Running tests

```bash
# Run all tests with race detector
make test

# Run tests and display coverage summary
make test-coverage

# Run a single package
go test ./internal/usecase/assert/...

# Run a specific test
go test ./internal/usecase/assert/... -run TestEvaluate_JSONPathEq_Pass
```

All tests must pass with `-race` before submitting a PR.

---

## Linting

Lynix uses [golangci-lint](https://golangci-lint.run/). Run all lint + test checks:

```bash
make check
```

Or lint only:

```bash
make lint
```

The lint configuration is in `.golangci.yml`. Fix all lint errors before submitting.

---

## Commit conventions

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <short description>

[optional body]
```

**Types:**

| Type       | When to use                                    |
|------------|------------------------------------------------|
| `feat`     | New feature                                    |
| `fix`      | Bug fix                                        |
| `test`     | Adding or updating tests                       |
| `refactor` | Code change that is not a feature or bug fix   |
| `docs`     | Documentation only                             |
| `chore`    | Build, deps, CI, tooling                       |

**Scopes** (optional but helpful): `assert`, `cli`, `tui`, `domain`, `infra`, `ci`, `config`.

**Examples:**

```
feat(assert): add jsonpath eq/contains/matches/gt/lt checks
fix(tui): recover from panic in run goroutine
test(cli): add coverage for printRun helpers
chore(ci): add race detector and multi-platform build
```

---

## Pull request guidelines

1. **Branch off `main`** using `<type>/<short-description>`, e.g. `feat/jsonpath-value-checks`.
2. **Keep PRs focused** — one logical change per PR.
3. **Tests are required** for new behaviour. All tests must pass with `-race`.
4. **No lint errors** — run `make check` locally before pushing.
5. **Update documentation** (README, comments) if public behaviour changes.
6. **Reference issues** in the PR description when applicable (`Closes #123`).

The CI pipeline runs on every push and PR:
- `test` job: `go test -race` + coverage summary
- `lint` job: golangci-lint
- `build` job: cross-compile for Linux, macOS, and Windows (amd64 + arm64)

All three jobs must pass for a PR to be merged.
