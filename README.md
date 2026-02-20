# Lynix

**Lynix** is a TUI-first CLI for API testing and functional validation — run requests, assert responses, and chain variables, all from the terminal in a Git-friendly way.

## Features

- **TUI-first** — navigate collections and environments with arrow keys; no commands to memorize
- **Git-friendly** — all config is plain YAML: version control, diff, and review in PRs
- **Assertions** — validate HTTP status codes, latency thresholds, and JSONPath expressions
- **Variable extraction** — extract values from responses via JSONPath and chain them into subsequent requests
- **Environment layering** — base environment + gitignored local secrets override
- **Run artifacts** — timestamped JSON files saved under `runs/` for traceability and CI diffing
- **Sensitive data masking** — redacts `Authorization`, `Cookie`, token, secret, and password fields before saving artifacts
- **CI-friendly** — headless `lynix run` with JSON output; cancellable with context timeout
- **Single binary** — one Go executable, no runtime dependencies; version/commit/date injected at build time
- **Built-in template variables** — `{{$uuid}}`, `{{$timestamp}}` out of the box

---

## Requirements

- Go 1.22+

---

## Quick Start

```bash
# Clone and build
git clone https://github.com/aalvaropc/lynix
cd lynix
make build
./bin/lynix

# Or run in dev mode directly
make dev
```

### Initialize a workspace

```bash
lynix init --path .
```

This scaffolds the following structure and patches your `.gitignore`:

```
.
├── lynix.yaml                  # Workspace config (anchors the workspace root)
├── collections/
│   └── demo.yaml               # Example collection
└── env/
    ├── dev.yaml                # Dev environment variables
    ├── stg.yaml                # Staging environment variables
    └── secrets.local.yaml      # Local secrets override (gitignored)
```

---

## TUI Navigation

Launch the TUI by running `lynix` (no subcommand):

| Key | Action |
|-----|--------|
| `↑` / `↓` | Move selection |
| `Enter` | Confirm selection / advance step |
| `Esc` | Go back |
| `c` | Cancel running execution |
| `s` | Toggle artifact save on/off |
| `q` | Quit |
| `?` | Show help |

The run wizard walks through three steps:
1. **Select collection** — pick from discovered `.yaml` files in `collections/`
2. **Select environment** — pick from discovered `.yaml` files in `env/`
3. **Confirm** — toggle whether to save the run artifact, then execute

---

## CLI Usage

All commands support the `-debug` flag to enable structured logging to `.lynix/logs/lynix.log`.

```bash
# Launch TUI (default)
lynix

# Print version info
lynix version

# Initialize a workspace
lynix init --path .

# Run a collection headlessly
lynix run -c demo -e dev
lynix run -c demo -e dev --no-save          # Skip saving the run artifact
lynix run -c demo -e dev --format json      # JSON output (for CI pipelines)

# Static validation (no HTTP requests)
lynix validate -c demo -e dev

# List collections in the workspace
lynix collections list

# List environments in the workspace
lynix envs list
```

The workspace root is auto-detected by walking up from the current directory until `lynix.yaml` is found.

---

## Collection Format

Collections are YAML files that describe a sequence of API requests:

```yaml
name: Auth Flow

vars:
  base_url: "https://api.example.com"
  timeout_ms: 2000

requests:
  # Simple GET with status assertion
  - name: health
    method: GET
    url: "{{base_url}}/health"
    assert:
      status: 200
      max_ms: 500

  # POST with JSON body, assertion, and variable extraction
  - name: login
    method: POST
    url: "{{base_url}}/auth/login"
    headers:
      Content-Type: "application/json"
    body:
      json:
        username: "{{username}}"
        password: "{{password}}"
    assert:
      status: 200
      max_ms: "{{timeout_ms}}"
      jsonpath:
        - expr: "$.token"
          exists: true
    extract:
      auth_token: "$.token"          # extracted into runtime vars

  # Subsequent request uses the extracted variable
  - name: users.list
    method: GET
    url: "{{base_url}}/v1/users"
    headers:
      Authorization: "Bearer {{auth_token}}"
    assert:
      status: 200
      jsonpath:
        - expr: "$.users[0].id"
          exists: true

  # Raw body with custom content type
  - name: upload.csv
    method: POST
    url: "{{base_url}}/upload"
    body:
      raw: "id,name\n1,alice"
    content_type: "text/csv"
    assert:
      status: 201
```

### Supported methods
`GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD`, `OPTIONS`

### Body types
| Field | Description |
|-------|-------------|
| `body.json` | Object — serialized as `application/json` |
| `body.form` | Key-value map — serialized as `application/x-www-form-urlencoded` |
| `body.raw` | String — content type set via `content_type` |

### Assertions
| Field | Description |
|-------|-------------|
| `assert.status` | Expected HTTP status code |
| `assert.max_ms` | Maximum acceptable latency in milliseconds |
| `assert.jsonpath[].expr` | JSONPath expression to evaluate |
| `assert.jsonpath[].exists` | Assert the path exists (and is non-null) |

### Variable extraction
```yaml
extract:
  my_var: "$.path.to.value"   # extracted from response body JSON
```
Extracted variables are available in all subsequent requests within the same run.

### Built-in template variables
| Variable | Value |
|----------|-------|
| `{{$uuid}}` | Random UUID v4 |
| `{{$timestamp}}` | Current Unix timestamp (seconds) |

---

## Environment Format

```yaml
# env/dev.yaml
vars:
  base_url: "http://localhost:8080"
  username: "dev-user"
```

```yaml
# env/secrets.local.yaml  (gitignored — overrides base env vars)
vars:
  password: "s3cr3t"
  api_key: "sk-..."
```

### Variable resolution order (highest priority wins)

```
secrets.local.yaml  >  environment YAML  >  collection vars  >  built-ins
```

---

## Workspace Configuration

`lynix.yaml` anchors the workspace root and controls global behavior:

```yaml
lynix:
  masking:
    enabled: true               # Redact sensitive headers/vars in saved artifacts

  defaults:
    env: dev                    # Default environment when none is specified

  paths:
    collections_dir: collections
    environments_dir: env
    runs_dir: runs

  artifacts:
    save_response_headers: true
    save_response_body: false   # Set to true to persist response bodies
```

---

## Run Artifacts

Each run is saved as a timestamped JSON file under `runs/`:

```
runs/
├── 20240601T120000Z_demo.json
└── index.jsonl                  # Append-only index of all runs
```

Sensitive fields (e.g., `Authorization`, `Cookie`, variables named `*token*`, `*secret*`, `*password*`) are masked as `********` before saving when `masking.enabled: true`.

Response body saving is opt-in (`artifacts.save_response_body: true`) and is capped at **256KB** per response.

---

## CI Integration

```bash
# Run and exit with code 1 on any assertion failure
lynix run -c smoke-tests -e stg --format json | jq .

# Skip artifact saving in ephemeral CI environments
lynix run -c smoke-tests -e stg --no-save
```

The process exits with a non-zero code if any request fails or any assertion is violated, making it drop-in compatible with GitHub Actions, GitLab CI, and similar systems.

---

## Project Structure

```
cmd/lynix/            # Binary entrypoint → delegates to cli.Execute()
internal/
├── domain/           # Pure domain model — zero external deps
│   ├── collection.go       # Collection, RequestSpec, AssertionsSpec, BodySpec
│   ├── environment.go      # Environment, Vars, merge/get/set helpers
│   ├── run.go              # RunResult, RequestResult, ResponseSnapshot
│   ├── config.go           # WorkspaceConfig, defaults, masking settings
│   ├── vars_resolver.go    # Template engine: resolves {{var}}, {{$uuid}}, etc.
│   └── errors.go           # Sentinel errors, OpError, IsKind()
│
├── ports/            # Interface definitions (ports/adapters pattern)
│   ├── collection_loader.go
│   ├── environment_loader.go
│   ├── environment_catalog.go
│   ├── request_runner.go
│   ├── artifact_store.go
│   ├── workspace.go
│   └── workspace_locator.go
│
├── infra/            # Adapter implementations
│   ├── httpclient/         # net/http client with tunable timeouts + HTTP/2
│   ├── httprunner/         # RequestRunner: resolves vars → executes → captures
│   ├── yamlcollection/     # YAML → domain.Collection
│   ├── yamlenv/            # YAML → domain.Environment (merges base + secrets)
│   ├── runstore/           # Saves JSON run artifacts + JSONL index
│   ├── fsworkspace/        # Workspace initializer (embed.FS templates)
│   ├── workspacefinder/    # Walks up dir tree to find lynix.yaml
│   └── logger/             # slog-based structured logger
│
├── usecase/          # Application use cases (orchestration layer)
│   ├── run_collection.go
│   ├── validate_collection.go
│   ├── init_workspace.go
│   ├── assert/             # Evaluates status / latency / JSONPath assertions
│   └── extract/            # Applies JSONPath extraction to response bodies
│
├── ui/tui/           # Bubble Tea TUI (Elm architecture)
│   ├── app.go              # Model, Init, Update, View state machine
│   ├── commands.go         # Async tea.Cmd wrappers
│   ├── messages.go         # Message types for the update loop
│   ├── theme.go            # Lipgloss styles
│   └── safe_model.go       # Panic-recovery wrapper
│
└── cli/              # Cobra CLI commands
    ├── run.go
    ├── validate.go
    ├── collections.go
    ├── envs.go
    └── workspace.go
```

### Architecture overview

Lynix follows **Hexagonal Architecture (Ports & Adapters)**:

- `domain/` is pure Go — no YAML, no `net/http`, no filesystem I/O
- `ports/` defines interfaces at the boundary of the application core
- `infra/` provides concrete implementations injected at startup
- `usecase/` orchestrates domain logic through port interfaces
- Both the TUI and CLI wire up the same use cases with the same adapters

---

## Development

```bash
make tidy      # go mod tidy
make dev       # Run TUI in dev mode (go run with ldflags)
make test      # go test ./...
make lint      # golangci-lint
make fmt       # gofmt -w .
make build     # Build binary → bin/lynix
```

Build flags inject version metadata:
- **Version** — `git describe --tags --dirty --always`
- **Commit** — `git rev-parse --short HEAD`
- **Date** — UTC build timestamp

---

## License

MIT
