# CLI Reference

All commands auto-detect the workspace root by walking up from the current directory until `lynix.yaml` is found. Override with `--workspace`.

All commands accept `--debug` to enable structured JSON logging to `.lynix/logs/lynix.log`.

---

## `lynix` (TUI)

```bash
lynix               # Launch interactive TUI
lynix --debug       # TUI with debug logging
```

### Navigation

| Key | Action |
|-----|--------|
| `Up` / `Down` | Move selection up/down |
| `Left` / `Right` | Switch tabs in results view |
| `Enter` | Confirm selection / advance step |
| `Esc` | Go back to previous screen |
| `c` | Cancel in-flight execution |
| `s` | Toggle artifact save (on/off) |
| `q` | Quit |
| `?` | Show help |

### Run Wizard

The TUI guides you through three steps:

**Step 1 -- Select collection**
Lists all `.yaml` files discovered in `collections/`. Navigate with `Up`/`Down` and press `Enter`.

**Step 2 -- Select environment**
Lists all environment files in `env/` (excluding `secrets.local.yaml`). The workspace default is pre-selected.

**Step 3 -- Confirm & run**
Shows a summary of what will run. Press `s` to toggle whether the run artifact is saved, then `Enter` to execute.

During execution a spinner is shown. Press `c` to cancel.

### Results View

After a run completes:

- **Per-request status** -- pass / fail with HTTP status code and latency
- **Assertion breakdown** -- each assertion result with descriptive message
- **Extracted variables** -- key=value pairs available in subsequent requests
- **Tabs** -- switch between request details and the raw response body

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

### Collection Resolution Order

1. If the value contains `/` or `\` -- treated as a file path
2. Tries `collections/{name}.yaml`, then `collections/{name}.yml`
3. Falls back to matching by collection `name` field (case-insensitive)

### Exit Codes

- `0` -- all requests completed and all assertions passed
- `1` -- any request failed or any assertion was violated

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

Outputs `OK` on success, or a descriptive error message on failure.

---

## `lynix collections list`

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

## `lynix envs list`

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
