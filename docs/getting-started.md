# Getting Started

## Installation

### Quick install (Linux / macOS)

```bash
curl -sSfL https://raw.githubusercontent.com/aalvaropc/lynix/main/install.sh | sh
```

This detects your OS and architecture, downloads the latest release, verifies the SHA-256 checksum, and installs to `/usr/local/bin`.

Pin a version or change the install directory:

```bash
LYNIX_VERSION=0.3.0 LYNIX_INSTALL_DIR=~/.local/bin \
  curl -sSfL https://raw.githubusercontent.com/aalvaropc/lynix/main/install.sh | sh
```

### Homebrew (macOS / Linux)

```bash
brew install aalvaropc/tap/lynix
```

### Go install

```bash
go install github.com/aalvaropc/lynix/cmd/lynix@latest
```

Requires Go 1.22+.

### Manual download

Download the binary for your platform from the [Releases](https://github.com/aalvaropc/lynix/releases) page, verify the checksum against `checksums.txt`, and place it in your `$PATH`.

### Build from source

```bash
git clone https://github.com/aalvaropc/lynix
cd lynix
make build
# binary is at ./bin/lynix
```

Requires Go 1.22+.

---

## Initialize a Workspace

```bash
cd your-project/
lynix init --path .
```

This scaffolds:

```
your-project/
├── lynix.yaml                  # Workspace config (anchors the workspace root)
├── collections/
│   └── demo.yaml               # Example collection to get started
├── env/
│   ├── dev.yaml                # Dev environment variables
│   ├── stg.yaml                # Staging environment variables
│   └── secrets.local.yaml      # Local secrets override (gitignored)
├── runs/                       # Saved run artifacts (gitignored)
└── .lynix/logs/                # Debug logs (gitignored)
```

`.gitignore` is automatically patched to exclude `runs/`, `.lynix/`, and `secrets.local.yaml`.

---

## Run Your First Collection

**Headlessly (CLI):**
```bash
lynix run -c demo -e dev
```

**Interactively (TUI):**
```bash
lynix
```

---

## Examples

The [`examples/`](../examples/) directory contains runnable examples you can try immediately — no API keys required:

| Example | What it shows |
|---------|---------------|
| [health-check](../examples/health-check/) | Single GET with status, latency, and JSONPath assertions |
| [rest-crud](../examples/rest-crud/) | Full CRUD lifecycle with JSON Schema validation |
| [auth-chain](../examples/auth-chain/) | Login, extract token, chain into authenticated requests |

```bash
# Try one now
lynix run -c examples/health-check/collection.yaml -e examples/health-check/env.yaml --no-save
```
