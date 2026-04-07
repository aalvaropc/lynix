# CI/CD Integration

Lynix is designed to work in CI pipelines without any configuration changes. A single binary, proper exit codes, and machine-readable output formats make it a natural fit for automated testing.

---

## Headless Run Examples

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

---

## GitHub Actions

### Using the Official GitHub Action

For the simplest setup, use the official GitHub Action — see [action.yml](../action.yml).

```yaml
- uses: aalvaropc/lynix@v1
  with:
    collection: smoke-tests
    environment: prod
```

### Simple Example (Exit Code Only)

```yaml
- name: Run API tests
  run: lynix run -c smoke-tests -e prod --no-save
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

---

## Exit Codes

- `0` -- all assertions passed
- `1` -- any request failed or any assertion was violated

Use exit codes directly in CI scripts to gate deployments or mark builds as failed.
