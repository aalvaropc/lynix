# CLI Reference

All commands auto-detect the workspace root by walking up from the current directory until `lynix.yaml` is found. Override with `--workspace`.

Running `lynix` with no subcommand prints the help.

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Everything passed |
| `1` | Assertion failures (the API misbehaved) |
| `2` | Usage or configuration error (bad flags, invalid YAML, missing files/vars) |
| `3` | Execution error (network failure, timeout, cancellation) |

If a run contains both execution errors and assertion failures, the exit code is `3`.

---

## `lynix version`

```bash
lynix version
# lynix v1.2.0 (commit=abc1234, date=2024-06-01T12:00:00Z)
```

---

## `lynix init`

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

## `lynix run`

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
| `--var` | | Override a variable (`key=value`, repeatable; highest precedence) |
| `--no-save` | | Skip saving the run artifact |
| `--format` | | Output format: `pretty` or `json` (default: `pretty`) |
| `--quiet` | `-q` | Show only failed requests in pretty output |
| `--no-color` | | Disable colored output (`NO_COLOR` is also honored) |
| `--report` | | Report type to generate (currently only `junit`) |
| `--report-path` | | File path to write the report to |
| `--fail-fast` | | Stop execution on the first failed request |
| `--only` | | Run only the named requests (comma-separated) |
| `--tags` | | Run only requests matching any of these tags (comma-separated) |
| `--parallel` | | Execute independent requests in parallel (dependency-graph scheduling) |
| `--dry-run` | | Resolve variables and show requests without executing |
| `--retries` | | Retries for transient errors (default: `run.retries` from `lynix.yaml`) |
| `--retry-delay` | | Delay between retries in ms (default: `run.retry_delay_ms`) |
| `--retry-5xx` | | Also retry on HTTP 5xx responses |
| `--insecure` | | Skip TLS certificate verification (prints a warning) |
| `--no-redirects` | | Do not follow HTTP redirects |

### Collection Resolution Order

1. If the value contains `/` or `\` -- treated as a file path
2. Tries `collections/{name}.yaml`, then `collections/{name}.yml`
3. Falls back to matching by collection `name` field (case-insensitive)

See [Exit Codes](#exit-codes) for the run exit code convention.

---

## `lynix validate`

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
| `--var` | | Override a variable (`key=value`, repeatable) |

Outputs `OK` on success, or a descriptive error message on failure.

---

## `lynix collections list`

List all collections discovered in the workspace.

```bash
lynix collections list
lynix collections list --format json
lynix collections list -w /path/to/workspace

# Output:
# Workspace: /home/user/project
#
# - Auth Flow  (collections/auth.yaml)
# - Demo API   (collections/demo.yaml)
```

---

## `lynix envs list`

List all environments discovered in the workspace.

```bash
lynix envs list
lynix envs list --format json

# Output:
# Workspace: /home/user/project
# Default:   dev
#
# - dev  (env/dev.yaml)
# - stg  (env/stg.yaml)
```

---

## `lynix import curl`

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

See [Importing](importing.md) for details on supported curl flags.

---

## `lynix import postman`

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

See [Importing](importing.md) for details on supported Postman features.
