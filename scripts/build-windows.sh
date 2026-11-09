#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="${VERSION:-0.7.1}"
export GOCACHE="${GOCACHE:-$PWD/.build/go-cache}"
export GOFLAGS="${GOFLAGS:--mod=vendor}"
export CGO_ENABLED=0
mkdir -p dist/win
echo "building Windows server ${VERSION}…"
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o cmd/navmini-windows/server.exe ./cmd/navmini
echo "building Windows WebView2 shell ${VERSION}…"
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w -H=windowsgui" -o "dist/NavMini-$VERSION-windows-amd64.exe" ./cmd/navmini-windows
rm -f cmd/navmini-windows/server.exe
ls -lh "dist/NavMini-$VERSION-windows-amd64.exe"
