# Health Check Example

Simplest possible Lynix collection: one GET request with status, latency, and JSONPath assertions.

## Run

```bash
lynix run -c examples/health-check/collection.yaml -e examples/health-check/env.yaml --no-save
```

## What it demonstrates

- Basic GET request
- Status code assertion (`status: 200`)
- Latency threshold (`max_ms: 3000`)
- JSONPath existence check (`$.id` exists)
- Environment variable override (`base_url`)
