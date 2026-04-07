# Lynix

Declarative API testing for CI/CD pipelines. Define requests in YAML, assert responses, run anywhere.

No accounts. No dashboards. No proprietary formats. Single binary, plain YAML, Git-native.

---

## Why Lynix?

| | Lynix | Bruno | Hurl | Postman | curl scripts |
|---|---|---|---|---|---|
| Single binary (no runtime) | Yes | No (Electron) | Yes | No | Yes |
| Git-friendly YAML | Yes | Yes (Bru format) | Yes (.hurl) | JSON exports | Manual |
| CI/CD native | Yes | CLI mode | Yes | Newman (Node) | Yes |
| JSONPath assertions | Yes | Yes | Limited | Yes | Manual |
| JSON Schema validation | Yes | No | No | No | Manual |
| Variable chaining | Yes | Yes | Captures | Yes | Manual |
| Sensitive data masking | Yes | No | No | Partial | No |
| Import from curl/Postman | Yes | Postman only | No | N/A | N/A |
| GitHub Action | Yes | No | No | No | Manual |

---

## Quick Start

```bash
# Install
curl -sSfL https://raw.githubusercontent.com/aalvaropc/lynix/main/install.sh | sh

# Initialize workspace
cd your-project && lynix init --path .

# Run your first collection
lynix run -c demo -e dev
```

---

## What a Collection Looks Like

```yaml
schema_version: 1
name: Auth Flow

vars:
  base_url: "https://api.example.com"

requests:
  - name: login
    method: POST
    url: "{{base_url}}/auth/login"
    headers:
      Content-Type: "application/json"
    json:
      username: "{{username}}"
      password: "{{password}}"
    assert:
      status: 200
      max_ms: 500
      jsonpath:
        "$.token":
          exists: true
    extract:
      auth_token: "$.token"

  - name: list-users
    method: GET
    url: "{{base_url}}/v1/users"
    headers:
      Authorization: "Bearer {{auth_token}}"
    assert:
      status: 200
      schema: "schemas/user-list.json"
      jsonpath:
        "$.total":
          gt: 0
```

---

## CI/CD Integration

### Official GitHub Action

```yaml
- uses: aalvaropc/lynix@v1
  with:
    collection: smoke-tests
    environment: prod
```

### With JUnit Report

```yaml
- name: Run API tests
  run: lynix run -c smoke-tests -e prod --no-save --report junit --report-path results.xml

- name: Publish test report
  uses: dorny/test-reporter@v1
  if: always()
  with:
    name: API Tests
    path: results.xml
    reporter: java-junit
```

Exit codes: `0` all passed, `1` any failure.

---

## Installation

| Method | Command |
|--------|---------|
| Quick install | `curl -sSfL https://raw.githubusercontent.com/aalvaropc/lynix/main/install.sh \| sh` |
| Homebrew | `brew install aalvaropc/tap/lynix` |
| Go install | `go install github.com/aalvaropc/lynix/cmd/lynix@latest` |
| Manual | [Download from Releases](https://github.com/aalvaropc/lynix/releases) |
| Source | `git clone && make build` |

See [Getting Started](docs/getting-started.md) for detailed installation instructions and workspace setup.

---

## Features

- **Headless CI mode** -- `lynix run` with JSON or JUnit XML output and proper exit codes
- **Assertions** -- status codes, latency thresholds, JSONPath (8 operators), JSON Schema (Draft 7 & 2020-12)
- **Variable chaining** -- extract values from responses and inject into subsequent requests
- **Git-friendly** -- collections and environments are plain YAML you can diff, review, and version
- **Environment layering** -- base environment files + gitignored `secrets.local.yaml` for local overrides
- **Sensitive data masking** -- redacts auth headers, tokens, secrets, and passwords before saving artifacts
- **Import from curl & Postman** -- convert existing API definitions to Lynix YAML
- **Single binary** -- one Go executable, no runtime dependencies

---

## Examples

The [`examples/`](examples/) directory contains runnable examples -- no API keys required:

| Example | What it shows |
|---------|---------------|
| [health-check](examples/health-check/) | Single GET with status, latency, and JSONPath assertions |
| [rest-crud](examples/rest-crud/) | Full CRUD lifecycle with JSON Schema validation |
| [auth-chain](examples/auth-chain/) | Login, extract token, chain into authenticated requests |

```bash
lynix run -c examples/health-check/collection.yaml -e examples/health-check/env.yaml --no-save
```

---

## Documentation

| Topic | Link |
|-------|------|
| Getting Started | [docs/getting-started.md](docs/getting-started.md) |
| Collection Format | [docs/collections.md](docs/collections.md) |
| Environments & Config | [docs/environments.md](docs/environments.md) |
| CLI Reference | [docs/cli-reference.md](docs/cli-reference.md) |
| CI/CD Integration | [docs/ci-cd.md](docs/ci-cd.md) |
| Run Artifacts | [docs/run-artifacts.md](docs/run-artifacts.md) |
| Importing (curl/Postman) | [docs/importing.md](docs/importing.md) |
| Architecture & Development | [docs/architecture.md](docs/architecture.md) |
| Editor Integration | [docs/editor-integration.md](docs/editor-integration.md) |
| Error Handling | [docs/error-handling.md](docs/error-handling.md) |

---

## License

MIT
