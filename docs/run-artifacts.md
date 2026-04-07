# Run Artifacts

Each successful run (unless `--no-save` is used) is saved as a timestamped JSON file.

## Directory Structure

```
runs/
├── 20240601T120000Z_auth-flow.json
├── 20240601T130500Z_demo.json
└── index.jsonl                    # Append-only index of all runs
```

---

## Artifact Structure

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

---

## Index File

`index.jsonl` -- one JSON object per line, append-only:

```json
{"id":"20240601T120000Z_auth-flow","file":"20240601T120000Z_auth-flow.json","collection":"Auth Flow","env":"dev","started_at":"2024-06-01T12:00:00Z"}
```

---

## Sensitive Data Masking

When `masking.enabled: true` (default), the following are replaced with `"********"` before saving:

**Headers:** `Authorization`, `Proxy-Authorization`, `Cookie`, `Set-Cookie`, `X-API-Key`, `X-Auth-Token`, and any header whose name contains `token`, `secret`, `password`, `api-key`, or `apikey`.

**Extracted variables:** Any variable whose name contains `token`, `secret`, or `password`.

---

## Response Body

Response bodies are **not saved by default**. Enable with:

```yaml
artifacts:
  save_response_body: true
```

Bodies are capped at **256 KB** per response. If truncated, `"truncated": true` is set in the artifact.

---

## Artifact Rotation

Control the maximum number of saved runs with `max_runs` in `lynix.yaml`. When exceeded, the oldest run artifacts are deleted automatically.
