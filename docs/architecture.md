# Architecture

Lynix follows **Hexagonal Architecture (Ports & Adapters)**. The core rule: domain never imports infra; use cases depend only on ports.

```
+------------------------------------------+
|  UI Layer                                |
|  +-- TUI (Bubble Tea)  internal/ui/tui/  |
|  +-- CLI (Cobra)       internal/cli/     |
+-------------------+----------------------+
                    |
         +----------v----------+
         |  Use Cases          |   internal/usecase/
         |  RunCollection      |
         |  ValidateCollection |
         |  InitWorkspace      |
         +----------+----------+
                    |
       +------------v------------+
       |  Ports (Interfaces)     |   internal/ports/
       |  CollectionLoader       |
       |  EnvironmentLoader      |
       |  RequestRunner          |
       |  ArtifactStore          |
       |  WorkspaceLocator       |
       +------------+------------+
                    |
         +----------v----------+
         |  Infra (Adapters)   |   internal/infra/
         |  yamlcollection/    |
         |  yamlenv/           |
         |  httpclient/        |
         |  httprunner/        |
         |  runstore/          |
         |  workspacefinder/   |
         |  fsworkspace/       |
         +----------+----------+
                    |
         +----------v----------+
         |  Domain (Pure Go)   |   internal/domain/
         |  Collection         |
         |  Environment        |
         |  RunResult          |
         |  VarResolver        |
         |  Config             |
         +---------------------+
```

Both the TUI and CLI wire the same use cases with the same adapters via `internal/infra/wiring`.

---

## Project Layout

```
cmd/lynix/              # Binary entrypoint -> cli.Execute()
internal/
+-- domain/             # Pure domain model (zero external deps)
|   +-- collection.go   # Collection, RequestSpec, AssertionsSpec, BodySpec
|   +-- environment.go  # Environment, Vars, merge helpers
|   +-- run.go          # RunResult, RequestResult, ResponseSnapshot
|   +-- config.go       # WorkspaceConfig, defaults
|   +-- vars_resolver.go# Template engine: resolves {{var}}, {{$uuid}}, etc.
|   +-- errors.go       # Sentinel errors, OpError, IsKind()
+-- ports/              # Interface definitions
+-- infra/              # Adapter implementations
|   +-- httpclient/     # net/http client with timeouts + HTTP/2
|   +-- httprunner/     # Resolves vars -> executes -> captures response
|   +-- yamlcollection/ # YAML <-> domain.Collection (loader + writer)
|   +-- yamlenv/        # YAML -> domain.Environment
|   +-- curlparse/      # curl command -> domain.Collection
|   +-- postmanparse/   # Postman v2.1 JSON -> domain.Collection
|   +-- redaction/      # Sensitive data masking engine
|   +-- runstore/       # JSON run artifacts + JSONL index
|   +-- fsworkspace/    # Workspace initializer (embed.FS templates)
|   +-- workspacefinder/# Walks up dir tree to find lynix.yaml
|   +-- logger/         # slog-based structured logger
|   +-- wiring/         # Shared adapter factory
+-- usecase/            # Application orchestration
|   +-- assert/         # Evaluates assertions
|   +-- extract/        # JSONPath extraction
+-- ui/tui/             # Bubble Tea TUI
+-- cli/                # Cobra commands
```

---

## Development

```bash
make dev            # Run TUI in dev mode (go run with ldflags)
make build          # Build binary -> bin/lynix
make test           # go test -race ./...
make test-coverage  # Tests + HTML coverage report
make lint           # golangci-lint run (v1.64.2)
make fmt            # gofmt -w .
make tidy           # go mod tidy
make check          # lint + test (run before PRs)
make clean          # Remove build artifacts
make vulncheck      # Check for known vulnerabilities
```

**Run a single test:**
```bash
go test ./internal/usecase/assert/... -run TestEvaluate_JSONPathEq_Pass
```

### Build Metadata

Three values are injected at build time via ldflags:

| Variable | Source |
|----------|--------|
| `Version` | `git describe --tags --dirty --always` |
| `Commit` | `git rev-parse --short HEAD` |
| `Date` | UTC build timestamp |
