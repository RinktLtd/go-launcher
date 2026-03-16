#!/usr/bin/env bash
set -euo pipefail

# Update demo: builds v1 and v2, serves v2 over HTTP, runs the launcher.
# You'll see: launcher starts v1 → v1 requests update → launcher downloads v2 → launcher starts v2.

DEMO_DIR="/tmp/go-launcher-update-demo"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
SERVE_DIR="/tmp/go-launcher-update-serve"
PORT=9384

CHILD_NAME="demo-child"
if [[ "$(uname)" == MINGW* || "$(uname)" == MSYS* ]]; then
    CHILD_NAME="demo-child.exe"
fi

echo "=== go-launcher update demo ==="
echo

# Clean previous run
rm -rf "$DEMO_DIR" "$SERVE_DIR"
mkdir -p "$DEMO_DIR/versions/current" "$SERVE_DIR"

# Build v1 → install as current version
echo "Building v1..."
go build -o "$DEMO_DIR/versions/current/$CHILD_NAME" "$SCRIPT_DIR/v1/"

# Build v2 → place in serve directory for HTTP download
echo "Building v2..."
go build -o "$SERVE_DIR/$CHILD_NAME" "$SCRIPT_DIR/v2/"

# Compute SHA-256 checksum of v2
if command -v sha256sum &>/dev/null; then
    CHECKSUM=$(sha256sum "$SERVE_DIR/$CHILD_NAME" | awk '{print $1}')
elif command -v shasum &>/dev/null; then
    CHECKSUM=$(shasum -a 256 "$SERVE_DIR/$CHILD_NAME" | awk '{print $1}')
else
    echo "Error: no sha256sum or shasum found"
    exit 1
fi

echo "v2 checksum: sha256:$CHECKSUM"
echo

# Start a simple HTTP file server for v2
echo "Starting file server on :$PORT..."
cd "$SERVE_DIR"
python3 -m http.server "$PORT" --bind 127.0.0.1 &>/dev/null &
HTTP_PID=$!
trap 'kill $HTTP_PID 2>/dev/null; rm -rf "$SERVE_DIR"' EXIT

# Wait for server to be ready
for i in $(seq 1 20); do
    curl -s -o /dev/null "http://127.0.0.1:$PORT/" && break
    sleep 0.1
done

# Set env vars so v1 knows where to find the update
export DEMO_UPDATE_URL="http://127.0.0.1:$PORT/$CHILD_NAME"
export DEMO_UPDATE_CHECKSUM="sha256:$CHECKSUM"

echo "v1 installed in: $DEMO_DIR/versions/current/"
echo "v2 served at:    $DEMO_UPDATE_URL"
echo
echo "--- launcher output ---"
echo

# Run the launcher
cd "$REPO_DIR"
go run "$SCRIPT_DIR/launcher/"
