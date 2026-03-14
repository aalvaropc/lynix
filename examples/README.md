# Lynix Examples

Runnable examples demonstrating common Lynix workflows. All examples use [JSONPlaceholder](https://jsonplaceholder.typicode.com) — no API keys or accounts required.

## Examples

| Example | Description |
|---------|-------------|
| [health-check](health-check/) | Single GET with status, latency, and JSONPath assertions |
| [rest-crud](rest-crud/) | Full Create-Read-Update-Delete lifecycle with JSON Schema validation |
| [auth-chain](auth-chain/) | Login, extract token, chain into authenticated requests |

## Quick start

```bash
# Run any example
lynix run -c examples/health-check/collection.yaml -e examples/health-check/env.yaml --no-save

# Validate without making requests
lynix validate -c examples/rest-crud/collection.yaml -e examples/rest-crud/env.yaml
```

## Structure

Each example contains:

```
examples/<name>/
├── collection.yaml   # Requests, assertions, and extraction rules
├── env.yaml          # Environment variables
└── README.md         # What it demonstrates and how to run
```
