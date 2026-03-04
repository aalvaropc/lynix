#!/bin/sh
# Lynix installer — downloads the latest (or pinned) release binary.
#
# Usage:
#   curl -sSfL https://raw.githubusercontent.com/aalvaropc/lynix/main/install.sh | sh
#
# Environment variables:
#   LYNIX_VERSION       Pin a specific version (e.g. "0.3.0"). Default: latest.
#   LYNIX_INSTALL_DIR   Installation directory. Default: /usr/local/bin.

set -e

REPO="aalvaropc/lynix"
INSTALL_DIR="${LYNIX_INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="lynix"

# --- helpers ---------------------------------------------------------------

log()  { printf '[lynix] %s\n' "$*"; }
fail() { log "ERROR: $*" >&2; exit 1; }

need_cmd() {
    if ! command -v "$1" > /dev/null 2>&1; then
        fail "required command not found: $1"
    fi
}

# --- detect OS/arch -------------------------------------------------------

detect_os() {
    os="$(uname -s)"
    case "$os" in
        Linux*)  echo "linux"  ;;
        Darwin*) echo "darwin" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *)       fail "unsupported OS: $os" ;;
    esac
}

detect_arch() {
    arch="$(uname -m)"
    case "$arch" in
        x86_64|amd64)  echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *)             fail "unsupported architecture: $arch" ;;
    esac
}

# --- resolve version -------------------------------------------------------

resolve_version() {
    if [ -n "${LYNIX_VERSION:-}" ]; then
        echo "$LYNIX_VERSION"
        return
    fi

    need_cmd curl

    tag="$(curl -sSfL \
        -H "Accept: application/vnd.github+json" \
        "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' \
        | head -1 \
        | sed 's/.*"tag_name"[[:space:]]*:[[:space:]]*"//;s/".*//')"

    if [ -z "$tag" ]; then
        fail "could not determine latest release"
    fi

    # Strip leading "v" if present.
    echo "$tag" | sed 's/^v//'
}

# --- download & verify ----------------------------------------------------

download_and_install() {
    version="$1"
    os="$2"
    arch="$3"

    ext="tar.gz"
    if [ "$os" = "windows" ]; then
        ext="zip"
    fi

    archive="lynix_${version}_${os}_${arch}.${ext}"
    checksums="checksums.txt"
    base_url="https://github.com/${REPO}/releases/download/v${version}"

    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' EXIT

    log "downloading ${archive}..."
    curl -sSfL -o "${tmpdir}/${archive}" "${base_url}/${archive}" \
        || fail "download failed — does release v${version} exist for ${os}/${arch}?"

    log "downloading checksums..."
    curl -sSfL -o "${tmpdir}/${checksums}" "${base_url}/${checksums}" \
        || fail "checksum download failed"

    log "verifying checksum..."
    expected="$(grep "${archive}" "${tmpdir}/${checksums}" | awk '{print $1}')"
    if [ -z "$expected" ]; then
        fail "archive ${archive} not found in checksums file"
    fi

    if command -v sha256sum > /dev/null 2>&1; then
        actual="$(sha256sum "${tmpdir}/${archive}" | awk '{print $1}')"
    elif command -v shasum > /dev/null 2>&1; then
        actual="$(shasum -a 256 "${tmpdir}/${archive}" | awk '{print $1}')"
    else
        fail "no sha256sum or shasum command found"
    fi

    if [ "$expected" != "$actual" ]; then
        fail "checksum mismatch: expected ${expected}, got ${actual}"
    fi
    log "checksum OK"

    log "extracting..."
    if [ "$ext" = "zip" ]; then
        need_cmd unzip
        unzip -q -o "${tmpdir}/${archive}" -d "${tmpdir}/out"
    else
        mkdir -p "${tmpdir}/out"
        tar -xzf "${tmpdir}/${archive}" -C "${tmpdir}/out"
    fi

    bin_src="${tmpdir}/out/${BINARY_NAME}"
    if [ "$os" = "windows" ]; then
        bin_src="${bin_src}.exe"
    fi

    if [ ! -f "$bin_src" ]; then
        fail "binary not found in archive"
    fi

    log "installing to ${INSTALL_DIR}/${BINARY_NAME}..."
    mkdir -p "$INSTALL_DIR"
    if [ -w "$INSTALL_DIR" ]; then
        mv "$bin_src" "${INSTALL_DIR}/${BINARY_NAME}"
        chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    else
        sudo mv "$bin_src" "${INSTALL_DIR}/${BINARY_NAME}"
        sudo chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    fi

    log "installed lynix v${version} to ${INSTALL_DIR}/${BINARY_NAME}"
}

# --- main ------------------------------------------------------------------

main() {
    need_cmd curl
    need_cmd uname

    os="$(detect_os)"
    arch="$(detect_arch)"
    version="$(resolve_version)"

    log "resolved: lynix v${version} for ${os}/${arch}"

    download_and_install "$version" "$os" "$arch"

    log "done! Run 'lynix version' to verify."
}

main
