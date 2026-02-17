# Lynix

**Lynix** is a TUI-first CLI for APIs: build and run **requests**, **functional checks**, and (soon) **performance benchmarks** — all from the terminal, in a Git-friendly way.

## Goals

- **TUI-first** — navigate with arrow keys, no need to memorize commands
- **Git-friendly** — YAML collections & environments, easy to version control
- **CI-friendly** — thresholds, JUnit output, artifacts (planned)
- **Single-binary** — one Go executable, no runtime dependencies

## Requirements

- Go 1.22+

## Quick Start

```bash
# Install dependencies
make tidy

# Run the TUI
make dev

# Or build the binary
make build
./bin/lynix
```

## Initialize a Workspace

```bash
lynix init --path .
```

This creates:
```
.
├── lynix.yaml           # Workspace configuration
├── collections/         # API request collections
│   └── demo.yaml
└── env/                 # Environment variables
    ├── dev.yaml
    ├── stg.yaml
    └── secrets.local.yaml  # (gitignored)
```

It also ensures your `.gitignore` includes common Lynix artifacts like `runs/`, `.lynix/`, and `env/secrets.local.yaml`.

## Project Structure

```
├── cmd/lynix/           # CLI entrypoint
├── internal/
│   ├── domain/          # Core domain types (Collection, Environment, Run, etc.)
│   ├── ports/           # Interfaces (CollectionLoader, EnvironmentLoader, etc.)
│   ├── infra/           # Implementations
│   │   ├── fsworkspace/       # Workspace initialization
│   │   ├── workspacefinder/   # Workspace detection & config loading
│   │   ├── yamlcollection/    # YAML collection parser
│   │   └── yamlenv/           # YAML environment parser
│   ├── ui/tui/          # Bubble Tea TUI
│   └── usecase/         # Application use cases
├── collections/         # Example collections
└── env/                 # Example environments
```

## Collection Format

Collections define API requests in YAML:

```yaml
name: Demo API
vars:
  base_url: "https://api.example.com"

requests:
  - name: health
    method: GET
    url: "{{base_url}}/health"
    assert:
      status: 200
      max_ms: 1500

  - name: users.list
    method: GET
    url: "{{base_url}}/v1/users"
    headers:
      Accept: "application/json"
    assert:
      status: 200
```

Optional: you can set `content_type` for requests with a body (useful for `raw` payloads):

```yaml
  - name: upload.text
    method: POST
    url: "{{base_url}}/upload"
    raw: "hello"
    content_type: "text/plain"
```

## Environment Format

Environments define variables per context (dev, stg, prod):

```yaml
# env/dev.yaml
vars:
  base_url: "http://localhost:8080"
```

Secrets are loaded from `secrets.local.yaml` (gitignored) and override base vars.

## Development

```bash
make tidy      # Tidy go.mod
make dev       # Run TUI in dev mode
make test      # Run tests
make lint      # Run golangci-lint
make build     # Build binary to bin/
```

## License

MIT
