# Environments

## Environment Format

Environment files live in `env/` and define variables for a specific target (dev, staging, production, etc.).

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

---

## Secrets

```yaml
# env/secrets.local.yaml  — gitignored, local overrides only
vars:
  password: "s3cr3t"
  api_key: "sk-1234567890abcdef"
```

`secrets.local.yaml` is automatically merged at runtime if it exists. Never commit it — the `.gitignore` added by `lynix init` excludes it.

---

## Variable Layering

When the same variable name appears in multiple places, the highest priority wins:

```
--var flags  >  secrets.local.yaml  >  environment YAML  >  collection vars  >  built-ins
```

| Source | Priority | Description |
|--------|----------|-------------|
| `--var key=value` | Highest | CLI overrides (repeatable) |
| `secrets.local.yaml` | High | Local overrides — gitignored |
| Environment YAML | Medium-high | Selected environment file (`-e dev`) |
| Collection `vars` | Medium | Defined in the collection itself |
| Built-ins (`$uuid`, `$timestamp`) | Lowest | Generated at request time |

Variables extracted from responses are merged into the running set and available to the next request.

---

## Process Environment Variables

`{{$env.NAME}}` reads directly from the process environment — the natural carrier
for CI secrets, with no file materialization:

```yaml
headers:
  Authorization: "Bearer {{$env.API_TOKEN}}"
```

```yaml
# GitHub Actions
- run: lynix run -c smoke -e prod --no-save
  env:
    API_TOKEN: ${{ secrets.API_TOKEN }}
```

An unset environment variable is a hard error, never an empty string.

CLI overrides work the same way for ad-hoc values:

```bash
lynix run -c smoke -e dev --var base_url=http://localhost:8080 --var user=alice
```

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

## HTTP Client Options

```yaml
lynix:
  run:
    cookies: true            # in-memory cookie jar: Set-Cookie propagates to later requests
    max_body_kb: 512         # response body cap (default 256)
    tls:
      ca_file: certs/ca.pem  # extra trusted CAs (PEM bundle, relative to workspace root)
    insecure: false          # disables ALL certificate verification — prefer tls.ca_file
```

`tls.ca_file` extends the system trust store, so a self-signed or corporate CA
does not require `insecure`. When `insecure` is active, every run prints a
warning to stderr. TLS 1.2 is the minimum negotiated version.

Requests are sent with a `User-Agent: lynix/<version>` header unless the
collection sets its own. Proxies follow the standard `HTTP_PROXY`/`HTTPS_PROXY`/
`NO_PROXY` environment variables.
