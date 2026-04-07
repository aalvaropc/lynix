# Lynix

**Lynix** is a TUI-first CLI tool for API testing and functional validation. Define HTTP requests, assert responses, extract variables, and chain them — all from the terminal using plain YAML that lives in your Git repository.

No accounts. No dashboards. No proprietary formats. Just a single binary and a folder of YAML files.


---

## Why Lynix?

| | Lynix | Postman / Insomnia | curl scripts |
|---|---|---|---|
| TUI interface | ✓ | GUI only | ✗ |
| Git-friendly config | ✓ | JSON exports | Manual |
| CI/CD headless mode | ✓ | Paid plans / Newman | ✓ |
| Variable chaining | ✓ | ✓ | Manual |
| JSONPath assertions | ✓ | ✓ | Manual |
| JSON Schema validation | ✓ | ✗ | Manual |
| Single binary | ✓ | ✗ | ✓ |
| Sensitive data masking | ✓ | Partial | ✗ |
| Import from curl/Postman | ✓ | N/A | N/A |

---

## Features

- **TUI-first** — interactive interface with arrow-key navigation; no commands to memorize for daily use
- **Headless CLI** — `lynix run` works in CI pipelines with JSON or JUnit XML output and proper exit codes
- **Git-friendly** — collections and environments are plain YAML you can diff, review, and version
- **Assertions** — validate status codes, latency thresholds, JSONPath expressions (6 operators), and JSON Schema
- **Variable extraction** — pull values from responses via JSONPath and inject them into subsequent requests
- **Environment layering** — base environment files + a gitignored `secrets.local.yaml` for local overrides
- **Run artifacts** — timestamped JSON files saved under `runs/` for traceability and audit
- **Sensitive data masking** — redacts `Authorization`, `Cookie`, token, secret, and password fields before saving
- **Built-in variables** — `{{$uuid}}` and `{{$timestamp}}` available out of the box
- **Import from curl & Postman** — `lynix import curl` and `lynix import postman` convert existing collections to Lynix YAML
- **Single binary** — one Go executable, no runtime dependencies

---

## Installation

### Quick install (Linux / macOS)

```bash
curl -sSfL https://raw.githubusercontent.com/aalvaropc/lynix/main/install.sh | sh
```

This detects your OS and architecture, downloads the latest release, verifies the SHA-256 checksum, and installs to `/usr/local/bin`.

Pin a version or change the install directory:

```bash
LYNIX_VERSION=0.3.0 LYNIX_INSTALL_DIR=~/.local/bin \
  curl -sSfL https://raw.githubusercontent.com/aalvaropc/lynix/main/install.sh | sh
```

### Homebrew (macOS / Linux)

```bash
brew install aalvaropc/tap/lynix
```

### Go install

```bash
go install github.com/aalvaropc/lynix/cmd/lynix@latest
```

Requires Go 1.22+.

### Manual download

Download the binary for your platform from the [Releases](https://github.com/aalvaropc/lynix/releases) page, verify the checksum against `checksums.txt`, and place it in your `$PATH`.

### Build from source

```bash
git clone https://github.com/aalvaropc/lynix
cd lynix
make build
# binary is at ./bin/lynix
```

Requires Go 1.22+.

---

## Quick Start

### Initialize a workspace

```bash
cd your-project/
lynix init --path .
```

This scaffolds:

```
your-project/
├── lynix.yaml                  # Workspace config (anchors the workspace root)
├── collections/
│   └── demo.yaml               # Example collection to get started
├── env/
│   ├── dev.yaml                # Dev environment variables
│   ├── stg.yaml                # Staging environment variables
│   └── secrets.local.yaml      # Local secrets override (gitignored)
├── runs/                       # Saved run artifacts (gitignored)
└── .lynix/logs/                # Debug logs (gitignored)
```

`.gitignore` is automatically patched to exclude `runs/`, `.lynix/`, and `secrets.local.yaml`.

### Run your first collection

**Interactively (TUI):**
```bash
lynix
```

**Headlessly (CLI):**
```bash
lynix run -c demo -e dev
```

---

## Examples

The [`examples/`](examples/) directory contains runnable examples you can try immediately — no API keys required:

| Example | What it shows |
|---------|---------------|
| [health-check](examples/health-check/) | Single GET with status, latency, and JSONPath assertions |
| [rest-crud](examples/rest-crud/) | Full CRUD lifecycle with JSON Schema validation |
| [auth-chain](examples/auth-chain/) | Login, extract token, chain into authenticated requests |

```bash
# Try one now
lynix run -c examples/health-check/collection.yaml -e examples/health-check/env.yaml --no-save
```

---

## TUI Guide

Launch the TUI by running `lynix` with no subcommand.

### Navigation

| Key | Action |
|-----|--------|
| `↑` / `↓` | Move selection up/down |
| `←` / `→` | Switch tabs in results view |
| `Enter` | Confirm selection / advance step |
| `Esc` | Go back to previous screen |
| `c` | Cancel in-flight execution |
| `s` | Toggle artifact save (on/off) |
| `q` | Quit |
| `?` | Show help |

### Run Wizard

The TUI guides you through three steps:

**Step 1 — Select collection**
Lists all `.yaml` files discovered in `collections/`. Navigate with `↑`/`↓` and press `Enter`.

**Step 2 — Select environment**
Lists all environment files in `env/` (excluding `secrets.local.yaml`). The workspace default is pre-selected.

**Step 3 — Confirm & run**
Shows a summary of what will run. Press `s` to toggle whether the run artifact is saved, then `Enter` to execute.

During execution a spinner is shown. Press `c` to cancel.

### Results View

After a run completes:

- **Per-request status** — `✓` pass / `✗` fail with HTTP status code and latency
- **Assertion breakdown** — each assertion result with descriptive message
- **Extracted variables** — key=value pairs available in subsequent requests
- **Tabs** — switch between request details and the raw response body

---

## CLI Reference

All commands auto-detect the workspace root by walking up from the current directory until `lynix.yaml` is found. Override with `--workspace`.

All commands accept `--debug` to enable structured JSON logging to `.lynix/logs/lynix.log`.

### `lynix` (TUI)

```bash
lynix               # Launch interactive TUI
lynix --debug       # TUI with debug logging
```

---

### `lynix version`

```bash
lynix version
# lynix v1.2.0 (commit=abc1234, date=2024-06-01T12:00:00Z)
```

---

### `lynix init`

Initialize a new workspace.

```bash
lynix init --path .                  # Initialize in current directory
lynix init --path /path/to/project   # Initialize at specific path
lynix init --path . --force          # Overwrite existing files
```

| Flag | Short | Description |
|------|-------|-------------|
| `--path` | `-p` | Target directory (default: `.`) |
| `--force` | | Overwrite existing files |

---

### `lynix run`

Execute a collection and assert all responses.

```bash
lynix run -c demo -e dev                     # Run with explicit environment
lynix run -c demo                            # Use default env from lynix.yaml
lynix run -c demo -e dev --no-save           # Skip saving the run artifact
lynix run -c demo -e dev --format json       # Machine-readable JSON output
lynix run -c demo -e dev --format pretty     # Human-readable output (default)
lynix run -c demo -e dev --report junit --report-path results.xml  # JUnit XML report
lynix run -c demo -e dev --fail-fast         # Stop on first failure
lynix run -c demo -e dev --only health,login # Run only named requests
lynix run -c demo -e dev --tags smoke,auth   # Run only requests with matching tags
lynix run -c demo -e dev --retries 3 --retry-delay 500  # Retry transient errors
lynix run -c demo -e dev --retries 2 --retry-5xx        # Also retry 5xx responses
lynix run -w /custom/root -c demo -e dev     # Override workspace root
```

| Flag | Short | Description |
|------|-------|-------------|
| `--collection` | `-c` | Collection name or path **(required)** |
| `--env` | `-e` | Environment name or path (optional) |
| `--workspace` | `-w` | Workspace root (optional, auto-detected) |
| `--no-save` | | Skip saving the run artifact |
| `--format` | | Output format: `pretty` or `json` (default: `pretty`) |
| `--report` | | Report type to generate (currently only `junit`) |
| `--report-path` | | File path to write the report to |
| `--fail-fast` | | Stop execution on the first failed request |
| `--only` | | Run only the named requests (comma-separated) |
| `--tags` | | Run only requests matching any of these tags (comma-separated) |
| `--retries` | | Number of retries for transient errors (default: 0) |
| `--retry-delay` | | Delay between retries in milliseconds (default: 0) |
| `--retry-5xx` | | Also retry on HTTP 5xx responses |

**Collection resolution order:**
1. If the value contains `/` or `\` → treated as a file path
2. Tries `collections/{name}.yaml`, then `collections/{name}.yml`
3. Falls back to matching by collection `name` field (case-insensitive)

**Exit codes:**
- `0` — all requests completed and all assertions passed
- `1` — any request failed or any assertion was violated

---

### `lynix validate`

Parse and validate a collection without making any HTTP requests. Useful in pre-commit hooks or PR checks.

```bash
lynix validate -c demo -e dev
lynix validate -c demo
```

| Flag | Short | Description |
|------|-------|-------------|
| `--collection` | `-c` | Collection name or path **(required)** |
| `--env` | `-e` | Environment name or path (optional) |
| `--workspace` | `-w` | Workspace root (optional) |

Outputs `OK` on success, or a descriptive error message on failure.

---

### `lynix collections list`

List all collections discovered in the workspace.

```bash
lynix collections list
lynix collections list -w /path/to/workspace

# Output:
# Workspace: /home/user/project
#
# - Auth Flow  (collections/auth.yaml)
# - Demo API   (collections/demo.yaml)
```

---

### `lynix envs list`

List all environments discovered in the workspace.

```bash
lynix envs list

# Output:
# Workspace: /home/user/project
# Default:   dev
#
# - dev  (env/dev.yaml)
# - stg  (env/stg.yaml)
```

---

### `lynix import curl`

Import a curl command into a Lynix YAML collection.

```bash
lynix import curl 'curl -X POST -H "Content-Type: application/json" -d '\''{"name":"test"}'\'' https://api.example.com/users'
lynix import curl 'curl https://api.example.com/health' -o collections/health.yaml
lynix import curl --from-file saved-curl.txt --name "My API"
```

| Flag | Short | Description |
|------|-------|-------------|
| `--output` | `-o` | Write YAML to file instead of stdout |
| `--from-file` | | Read curl command from a file |
| `--name` | | Override collection name |

**Supported curl flags:** `-X`, `-H`, `-d`/`--data`/`--data-raw`, `--json`, `-u` (basic auth).

**Unsupported (warned):** `--compressed`, `-k`, `-L`, `--cert`, `--key`, `-o`, `-v`, `-s`, `-F` (multipart), `-d @file`.

The importer extracts `base_url` as a variable and rewrites the URL to use `{{base_url}}`.

---

### `lynix import postman`

Import a Postman v2.1 collection JSON file into a Lynix YAML collection.

```bash
lynix import postman collection.json
lynix import postman collection.json -o collections/imported.yaml
lynix import postman collection.json --name "Renamed API"
```

| Flag | Short | Description |
|------|-------|-------------|
| `--output` | `-o` | Write YAML to file instead of stdout |
| `--name` | | Override collection name |

**Supported:** Requests with headers, JSON bodies (`raw` + `language: json`), URL-encoded bodies, collection variables, nested folders (flattened with dot-prefix names).

**Unsupported (warned):** Pre-request/test scripts, auth blocks, multipart form-data, Postman dynamic variables (`{{$randomInt}}`).

Postman `{{variable}}` syntax passes through directly as it matches Lynix templating.

### Migrate from existing tools

Already have curl commands or Postman collections? Import them in seconds:

```bash
# From a curl command (e.g., copied from browser DevTools)
lynix import curl 'curl -H "Authorization: Bearer tok" https://api.example.com/users' -o collections/users.yaml

# From a Postman export
lynix import postman my-collection.json -o collections/imported.yaml

# Then run immediately
lynix run -c imported -e dev
```

---

## Collection Format

Collections are YAML files stored in `collections/`. They describe a sequence of HTTP requests with optional assertions and variable extraction.

```yaml
schema_version: 1
name: Auth Flow

# Default variables (lowest priority — overridden by env vars)
vars:
  base_url: "https://api.example.com"
  timeout_ms: 2000

requests:
  - name: health-check
    method: GET
    url: "{{base_url}}/health"
    assert:
      status: 200
      max_ms: 500

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
      max_ms: "{{timeout_ms}}"
      jsonpath:
        "$.token":
          exists: true
          matches: "^[A-Za-z0-9._-]+$"
    extract:
      auth_token: "$.token"          # available in all subsequent requests

  - name: list-users
    method: GET
    url: "{{base_url}}/v1/users"
    headers:
      Authorization: "Bearer {{auth_token}}"
    assert:
      status: 200
      jsonpath:
        "$.data":
          exists: true
        "$.data[0].active":
          eq: "true"
        "$.total":
          gt: 0
```

### Request fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | ✓ | Unique identifier for the request |
| `method` | ✓ | HTTP method: `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD`, `OPTIONS` |
| `url` | ✓ | URL — supports `{{variable}}` templating |
| `headers` | | Key-value map of HTTP headers (templating supported) |
| `json` | | JSON request body (object or array) |
| `form` | | Form URL-encoded body (string key-value map) |
| `raw` | | Raw text body |
| `tags` | | List of tags for selective execution with `--tags` |
| `delay_ms` | | Delay in milliseconds before executing this request |
| `timeout_ms` | | Per-request timeout in ms — aborts the request if exceeded (distinct from `max_ms` which is an assertion) |
| `assert` | | Assertions on the response |
| `extract` | | Variables to extract from the response body |

> Only one of `json`, `form`, or `raw` may be specified per request.

### Templating

Variables are injected using `{{variable_name}}` syntax. Works in URLs, headers, body values, and assertion values.

**Built-in variables** (generated fresh per request):

| Variable | Value |
|----------|-------|
| `{{$uuid}}` | Random UUID v4 |
| `{{$timestamp}}` | Current Unix timestamp (seconds) |
| `{{$isoTimestamp}}` | ISO 8601 UTC timestamp (`2024-06-01T12:00:00Z`) |
| `{{$randomInt}}` | Random integer 0–9999 |
| `{{$randomString}}` | Random 8-character alphanumeric string |
| `{{$randomEmail}}` | Random email (`user_abc123@test.lynix`) |
| `{{$randomBool}}` | Random `true` or `false` |

---

## Assertions

Assertions are evaluated on every response regardless of previous failures. Each produces a named result with a pass/fail status.

### Status code

```yaml
assert:
  status: 200
```

Checks the HTTP status code exactly.

### Latency

```yaml
assert:
  max_ms: 500          # fixed value
  max_ms: "{{timeout_ms}}"  # or from a variable
```

Checks that the response latency is at or below the threshold.

### JSONPath assertions

Keyed by a JSONPath expression. Each key can combine multiple operators against the same path.

```yaml
assert:
  jsonpath:
    "$.data.field":
      exists: true          # path exists and value is non-empty
      eq: "expected"        # string equality
      contains: "partial"   # substring match
      matches: "^regex$"    # Go stdlib regexp
      gt: 10                # numeric greater-than
      lt: 1000              # numeric less-than
      not_eq: "forbidden"   # string inequality
      not_contains: "error" # substring absence
```

| Operator | Type | Description |
|----------|------|-------------|
| `exists` | bool | Path resolves to a non-null, non-empty value |
| `eq` | string | Value equals the given string (after type coercion) |
| `contains` | string | Value contains the given substring |
| `matches` | string | Value matches the given regular expression |
| `gt` | number | Numeric value is greater than threshold |
| `lt` | number | Numeric value is less than threshold |
| `not_eq` | string | Value does NOT equal the given string |
| `not_contains` | string | Value does NOT contain the given substring |

**JSONPath syntax** follows [PaesslerAG/jsonpath](https://github.com/PaesslerAG/jsonpath):
- `$.field` — top-level field
- `$.nested.field` — nested field
- `$.array[0].id` — array element
- `$.array[*].status` — all elements (returns array)

**Example:**
```yaml
assert:
  jsonpath:
    "$.token":
      exists: true
    "$.user_id":
      matches: "^user-[0-9]+$"
    "$.count":
      gt: 0
      lt: 100
```

### JSON Schema validation

Validate the entire response body against a JSON Schema document. Supports Draft 7 and 2020-12.

**File reference** (path relative to the collection file):

```yaml
assert:
  schema: "schemas/user.json"
```

**Inline schema** (embedded in the collection YAML):

```yaml
assert:
  schema_inline:
    type: object
    required: ["id", "name"]
    properties:
      id:
        type: integer
      name:
        type: string
      email:
        type: string
        format: email
```

| Field | Type | Description |
|-------|------|-------------|
| `schema` | string | Path to a `.json` schema file, relative to the collection directory |
| `schema_inline` | object | Inline JSON Schema definition |

> `schema` and `schema_inline` are mutually exclusive — using both on the same request is a validation error.

On failure, the assertion message includes the instance location path and a description of the violation (e.g., `/user: missing property 'id'`).

**Combining with other assertions:**

```yaml
assert:
  status: 200
  max_ms: 1000
  schema: "schemas/user.json"
  jsonpath:
    "$.email":
      exists: true
```

Schema validation runs alongside status, latency, and JSONPath assertions — all results are reported independently.

---

## Variable Extraction

Extract values from a JSON response body and inject them into all subsequent requests in the collection.

```yaml
extract:
  auth_token: "$.token"
  user_id: "$.data.users[0].id"
  roles: "$.data.users[0].roles"   # array → stored as JSON string
```

Extraction rules are applied in sorted order. If a rule fails (path not found, empty value), the error is reported but remaining extractions continue.

**Value conversion rules:**

| Response value | Stored as |
|---------------|-----------|
| String | String |
| Number / bool | String representation |
| Single-element array | The element's value |
| Multi-element array | JSON string |
| Object/map | JSON string |
| Null or empty | Extraction fails |

Extracted variables are available in all subsequent requests within the same run.

---

## Variable Resolution Order

When the same variable name appears in multiple places, the highest priority wins:

```
secrets.local.yaml  >  environment YAML  >  collection vars  >  built-ins
```

| Source | Priority | Description |
|--------|----------|-------------|
| `secrets.local.yaml` | Highest | Local overrides — gitignored |
| Environment YAML | High | Selected environment file (`-e dev`) |
| Collection `vars` | Medium | Defined in the collection itself |
| Built-ins (`$uuid`, `$timestamp`) | Lowest | Generated at request time |

Variables extracted from responses are merged into the running set and available to the next request.

---

## Environment Format

```yaml
# env/dev.yaml
vars:
  base_url: "http://localhost:8080"
  username: "dev-user"
  timeout_ms: 2000
```

```yaml
# env/stg.yaml
vars:
  base_url: "https://staging-api.example.com"
  username: "stg-user"
```

```yaml
# env/secrets.local.yaml  — gitignored, local overrides only
vars:
  password: "s3cr3t"
  api_key: "sk-1234567890abcdef"
```

`secrets.local.yaml` is automatically merged at runtime if it exists. Never commit it — the `.gitignore` added by `lynix init` excludes it.

---

## Workspace Configuration

`lynix.yaml` anchors the workspace root and controls global behaviour. All fields are optional — the defaults are shown below.

```yaml
lynix:
  schema_version: 1                   # Schema version (required >= 1)

  # Redact sensitive headers and variables before saving run artifacts
  masking:
    enabled: true
    # mask_request_headers: true
    # mask_request_body: true
    # mask_response_headers: true     # Toggle response header masking
    # mask_response_body: true
    # mask_query_params: true
    # mask_cli_output: false
    # fail_on_detected_secret: false  # Fail if unmasked secrets detected in artifacts

  # Default environment when -e / --env is not specified
  defaults:
    env: dev

  # Directory paths relative to workspace root
  paths:
    collections_dir: collections
    environments_dir: env
    runs_dir: runs

  # What to include in saved run artifacts
  artifacts:
    save_response_headers: true    # Include response headers
    save_response_body: false      # Include response body (opt-in, capped at 256 KB)

  # Global timeout for an entire collection run
  run:
    timeout_seconds: 300           # Default: 300 (5 minutes)
    retries: 0                     # Number of retries for transient errors
    retry_delay_ms: 0              # Delay between retries in milliseconds
    retry_5xx: false               # Also retry on HTTP 5xx responses
```

---

## Run Artifacts

Each successful run (unless `--no-save` is used) is saved as a timestamped JSON file:

```
runs/
├── 20240601T120000Z_auth-flow.json
├── 20240601T130500Z_demo.json
└── index.jsonl                    # Append-only index of all runs
```

**Artifact structure:**
```json
{
  "collection_name": "Auth Flow",
  "environment_name": "dev",
  "started_at": "2024-06-01T12:00:00Z",
  "ended_at": "2024-06-01T12:00:05Z",
  "results": [
    {
      "name": "login",
      "method": "POST",
      "url": "https://api.example.com/auth/login",
      "status_code": 200,
      "latency_ms": 123,
      "assertions": [
        { "name": "status", "passed": true, "message": "status 200" },
        { "name": "jsonpath.exists", "passed": true, "message": "jsonpath \"$.token\" exists" }
      ],
      "extracts": [
        { "name": "auth_token", "success": true, "message": "extracted auth_token" }
      ],
      "extracted": {
        "auth_token": "********"
      }
    }
  ]
}
```

**`index.jsonl`** — one JSON object per line, append-only:
```json
{"id":"20240601T120000Z_auth-flow","file":"20240601T120000Z_auth-flow.json","collection":"Auth Flow","env":"dev","started_at":"2024-06-01T12:00:00Z"}
```

### Sensitive data masking

When `masking.enabled: true` (default), the following are replaced with `"********"` before saving:

**Headers:** `Authorization`, `Proxy-Authorization`, `Cookie`, `Set-Cookie`, `X-API-Key`, `X-Auth-Token`, and any header whose name contains `token`, `secret`, `password`, `api-key`, or `apikey`.

**Extracted variables:** Any variable whose name contains `token`, `secret`, or `password`.

### Response body

Response bodies are **not saved by default**. Enable with:
```yaml
artifacts:
  save_response_body: true
```

Bodies are capped at **256 KB** per response. If truncated, `"truncated": true` is set in the artifact.

---

## CI Integration

Lynix is designed to work in CI pipelines without any configuration changes.

```bash
# Run smoke tests and exit with non-zero code on failure
lynix run -c smoke-tests -e stg

# JSON output for parsing or downstream steps
lynix run -c integration-tests -e prod --format json | jq '.results[].assertions'

# JUnit XML report alongside pretty output
lynix run -c smoke-tests -e stg --report junit --report-path results.xml

# Stop on first failure (fail-fast)
lynix run -c smoke-tests -e stg --fail-fast --no-save

# Run only tagged requests
lynix run -c integration-tests -e stg --tags smoke

# Run specific requests by name
lynix run -c integration-tests -e stg --only health,login

# Skip saving artifacts in ephemeral environments
lynix run -c smoke-tests -e stg --no-save

# Validate config before running (e.g., in a pre-commit hook)
lynix validate -c smoke-tests -e stg
```

**GitHub Actions example with JUnit report:**
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

**Simple example (exit code only):**
```yaml
- name: Run API tests
  run: lynix run -c smoke-tests -e prod --no-save
```

**Exit codes:**
- `0` — all assertions passed
- `1` — any request failed or any assertion was violated

---

## Error Handling

### Request errors

If a request cannot be completed (network failure, timeout, DNS error), the result includes a structured error:

| Kind | Description |
|------|-------------|
| `dns` | DNS resolution failed |
| `connection` | Could not connect to host |
| `timeout` | Request exceeded HTTP client timeout |
| `canceled` | Run was canceled by the user |
| `http` | HTTP protocol error |
| `unknown` | Unexpected error |

### Missing variables

If a template placeholder like `{{my_var}}` cannot be resolved, the request fails with a `missing_variable` error pointing to the exact variable name. The TUI shows a human-readable message.

---

## Development

```bash
make dev            # Run TUI in dev mode (go run with ldflags)
make build          # Build binary → bin/lynix
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

### Build metadata

Three values are injected at build time via ldflags:

| Variable | Source |
|----------|--------|
| `Version` | `git describe --tags --dirty --always` |
| `Commit` | `git rev-parse --short HEAD` |
| `Date` | UTC build timestamp |

---

## Editor Integration

### VS Code / YAML Schema Validation

Lynix ships JSON Schema files under `schemas/`. With the [YAML extension](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml), add to `.vscode/settings.json`:

```json
{
  "yaml.schemas": {
    "./schemas/collection.schema.json": "collections/*.yaml",
    "./schemas/environment.schema.json": "env/*.yaml",
    "./schemas/workspace.schema.json": "lynix.yaml"
  }
}
```

This gives auto-complete and inline validation for collection, environment, and workspace YAML files.

---

## Architecture

Lynix follows **Hexagonal Architecture (Ports & Adapters)**. The core rule: domain never imports infra; use cases depend only on ports.

```
┌─────────────────────────────────────────┐
│  UI Layer                               │
│  ├── TUI (Bubble Tea)  internal/ui/tui/ │
│  └── CLI (Cobra)       internal/cli/    │
└────────────────┬────────────────────────┘
                 │
      ┌──────────▼──────────┐
      │  Use Cases          │   internal/usecase/
      │  RunCollection      │
      │  ValidateCollection │
      │  InitWorkspace      │
      └──────────┬──────────┘
                 │
    ┌────────────▼────────────┐
    │  Ports (Interfaces)     │   internal/ports/
    │  CollectionLoader       │
    │  EnvironmentLoader      │
    │  RequestRunner          │
    │  ArtifactStore          │
    │  WorkspaceLocator       │
    └────────────┬────────────┘
                 │
      ┌──────────▼──────────┐
      │  Infra (Adapters)   │   internal/infra/
      │  yamlcollection/    │
      │  yamlenv/           │
      │  httpclient/        │
      │  httprunner/        │
      │  runstore/          │
      │  workspacefinder/   │
      │  fsworkspace/       │
      └──────────┬──────────┘
                 │
      ┌──────────▼──────────┐
      │  Domain (Pure Go)   │   internal/domain/
      │  Collection         │
      │  Environment        │
      │  RunResult          │
      │  VarResolver        │
      │  Config             │
      └─────────────────────┘
```

Both the TUI and CLI wire the same use cases with the same adapters via `internal/infra/wiring`.

### Project layout

```
cmd/lynix/              # Binary entrypoint → cli.Execute()
internal/
├── domain/             # Pure domain model (zero external deps)
│   ├── collection.go   # Collection, RequestSpec, AssertionsSpec, BodySpec
│   ├── environment.go  # Environment, Vars, merge helpers
│   ├── run.go          # RunResult, RequestResult, ResponseSnapshot
│   ├── config.go       # WorkspaceConfig, defaults
│   ├── vars_resolver.go# Template engine: resolves {{var}}, {{$uuid}}, etc.
│   └── errors.go       # Sentinel errors, OpError, IsKind()
├── ports/              # Interface definitions
├── infra/              # Adapter implementations
│   ├── httpclient/     # net/http client with timeouts + HTTP/2
│   ├── httprunner/     # Resolves vars → executes → captures response
│   ├── yamlcollection/ # YAML ↔ domain.Collection (loader + writer)
│   ├── yamlenv/        # YAML → domain.Environment
│   ├── curlparse/      # curl command → domain.Collection
│   ├── postmanparse/   # Postman v2.1 JSON → domain.Collection
│   ├── redaction/      # Sensitive data masking engine
│   ├── runstore/       # JSON run artifacts + JSONL index
│   ├── fsworkspace/    # Workspace initializer (embed.FS templates)
│   ├── workspacefinder/# Walks up dir tree to find lynix.yaml
│   ├── logger/         # slog-based structured logger
│   └── wiring/         # Shared adapter factory
├── usecase/            # Application orchestration
│   ├── assert/         # Evaluates assertions
│   └── extract/        # JSONPath extraction
├── ui/tui/             # Bubble Tea TUI
└── cli/                # Cobra commands
```

---

## License

MIT
