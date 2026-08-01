# Run Artifacts

Each run (unless `--no-save` is used) is saved as a timestamped JSON file.

## Directory Structure

```
runs/
├── 20240601T120000Z_auth-flow.json
├── 20240601T130500Z_demo.json
└── index.jsonl                    # Index of saved runs (JSONL)
```

---

## Artifact Structure

All keys are snake_case. `schema_version` identifies the artifact format so it
can evolve without breaking consumers.

```json
{
  "schema_version": 1,
  "collection_name": "Auth Flow",
  "collection_path": "collections/auth-flow.yaml",
  "environment_name": "dev",
  "started_at": "2024-06-01T12:00:00Z",
  "ended_at": "2024-06-01T12:00:05Z",
  "results": [
    {
      "name": "login",
      "method": "POST",
      "url": "https://api.example.com/auth/login",
      "resolved_url": "https://api.example.com/auth/login",
      "request_headers": { "Content-Type": "application/json" },
      "request_body": "{\"username\":\"alice\",\"password\":\"********\"}",
      "status_code": 200,
      "latency_ms": 123,
      "assertions": [
        { "name": "status", "passed": true, "message": "status 200" },
        { "name": "jsonpath.exists", "passed": true, "message": "jsonpath \"$.token\" exists" }
      ],
      "extracts": [
        { "name": "auth_token", "success": true, "message": "extracted \"auth_token\"" }
      ],
      "extracted": { "auth_token": "********" },
      "response": {
        "headers": { "Content-Type": ["application/json"] },
        "body": "{\"token\":\"********\"}"
      },
      "attempts": 1
    }
  ]
}
```

### Bodies

Request and response bodies are stored as **plain text** when they are valid
UTF-8 — artifacts stay greppable and diffable. Binary bodies are stored as a
`"base64:"`-prefixed string.

---

## Index File

`index.jsonl` — one JSON object per line:

```json
{"id":"20240601T120000Z_auth-flow","file":"20240601T120000Z_auth-flow.json","collection":"Auth Flow","env":"dev","started_at":"2024-06-01T12:00:00Z"}
```

---

## Sensitive Data Masking

When `masking.enabled: true` (default), sensitive data is masked with
`"********"` before saving, across every surface: request/response headers,
JSON and form bodies, query strings (in both `url` and `resolved_url`),
extracted variables, assertion messages, and error messages.

Masking combines key-based rules (`token`, `secret`, `password`, `api-key`,
...) with literal scrubbing of every value from `secrets.local.yaml` and
detection of well-known credential formats (JWT, GitHub/Stripe/AWS/Slack
tokens). See `masking` in `lynix.yaml` for per-surface toggles and custom
rules; `fail_on_detected_secret: true` aborts the save if a known secret
value or credential-shaped string survives redaction.

---

## Response Body

Response bodies and headers are saved by default. Disable with:

```yaml
artifacts:
  save_response_body: false
  save_response_headers: false
```

Bodies are capped at **256 KB** per response by default (configurable via `run.max_body_kb`). If truncated, `"truncated": true` is set in the artifact.

---

## Artifact Rotation

Control the maximum number of saved runs with `artifacts.max_runs` in
`lynix.yaml`. When exceeded, the oldest artifacts (by timestamp, then
collision suffix) are deleted and `index.jsonl` is rewritten atomically.
Rotation only ever touches Lynix's own timestamp-prefixed files.
