# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest release | Yes |
| Older releases | Best effort |

## Reporting a Vulnerability

If you discover a security vulnerability in Lynix, please report it responsibly.

**Do not open a public GitHub issue for security vulnerabilities.**

Instead, email **security@aalvaropc.dev** with:

1. A description of the vulnerability
2. Steps to reproduce
3. Affected versions (if known)
4. Any suggested fix (optional)

You should receive an acknowledgment within 48 hours. We will work with you to understand the issue and coordinate a fix before any public disclosure.

## Scope

Lynix is a CLI tool that runs locally. Security concerns include:

- **Sensitive data leakage** — credentials or tokens written to artifacts, logs, or stdout
- **Command injection** — through variable resolution or importer parsing
- **Path traversal** — through collection paths, schema references, or workspace resolution
- **Dependency vulnerabilities** — in Go module dependencies

## Security Features

Lynix includes built-in protections:

- **Sensitive data masking** — Authorization headers, cookies, and fields matching token/secret/password/api-key patterns are redacted before artifact storage
- **secrets.local.yaml** — gitignored by default on `lynix init`
- **No network access beyond user-defined requests** — no telemetry, no phoning home
- **Dependency scanning** — `make vulncheck` runs `govulncheck` against known CVEs
