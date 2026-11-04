#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${VERSION:-0.5.5}"
APP="dist/NavMini.app"
GO_IMAGE="bin/navmini"

mkdir -p bin .build/swift-modules
export GOFLAGS="-mod=vendor"
export GOCACHE="${GOCACHE:-$PWD/.build/go-cache}"
go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o "$GO_IMAGE" ./cmd/navmini

rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp "$GO_IMAGE" "$APP/Contents/Resources/navmini"
chmod +x "$APP/Contents/Resources/navmini"

swiftc -O -swift-version 5 -module-cache-path "$PWD/.build/swift-modules" -framework Cocoa -framework WebKit shell/main.swift -o "$APP/Contents/MacOS/NavMini"
cp shell/Info.plist "$APP/Contents/Info.plist"
cp shell/AppIcon.icns "$APP/Contents/Resources/AppIcon.icns"
chmod +x "$APP/Contents/MacOS/NavMini"

codesign --force --deep --sign - --identifier com.navmini.app "$APP" >/dev/null 2>&1 || true
echo "built: $APP"
echo "run:   open \"$APP\""
