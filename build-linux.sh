#!/usr/bin/env bash
# Build binary Linux (amd64) — buat upload ke server Debian.
# Hasil: snjsync_linux (static, tanpa runtime di server).
set -euo pipefail
cd "$(dirname "$0")"
GOOS=linux GOARCH=${GOARCH:-amd64} CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o snjsync_linux .
echo "OK -> $(pwd)/snjsync_linux"