# Roadmap

Lynix is a declarative API test runner for CI/CD: plain YAML, a single binary,
JSON Schema assertions, and secret redaction by default. This roadmap lists
direction, not promises — issues and PRs are the best way to influence it.

## Next (v0.5.x)

- **Polling / retry-until**: re-run a request until its assertions pass
  (async APIs: submit job → poll status), with per-request backoff.
- **Data-driven requests**: iterate a request over a dataset
  (inline list or CSV/JSON file) — one logical test, N cases.
- **Per-request retry overrides**: `retries`/`retry_5xx` at request level,
  restricted to idempotent methods by default.
- **`--only`/`--tags` aware dep graph**: pull in producers of consumed
  variables automatically instead of failing.

## Later

- **OpenAPI integration**: validate collections against a spec (drift
  detection), and scaffold collections from a spec.
- **Multipart/file bodies**: `body_file:` and multipart form-data uploads.
- **GraphQL first-class**: `graphql:` block with query/variables and
  error-aware assertions (a 200 with `errors[]` is a failure).
- **HTML run report**: single-file report for CI artifacts.
- **Depends-on**: explicit request ordering for side-effect dependencies
  the variable graph cannot see.

## Non-goals

- **Embedded scripting** (JS/Lua). Lynix is declarative on purpose; when you
  need scripting, generators/wrappers or another tool are the better fit.
- **GUI / desktop app**. Bruno, Yaak, and Posting do this well already.
- **Load testing**. Use k6 or friends; Lynix asserts correctness, not throughput.
