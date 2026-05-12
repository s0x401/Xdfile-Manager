#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

go test ./...

build_target() {
  local goos="$1"
  local goarch="$2"
  local output="$3"

  mkdir -p "$(dirname "$output")"
  echo "Building ${goos}/${goarch} -> ${output}"
  GOOS="$goos" GOARCH="$goarch" go build -o "$output" .
}

build_target windows amd64 dist/windows-amd64/xdfile.exe
build_target darwin amd64 dist/darwin-amd64/xdfile
build_target darwin arm64 dist/darwin-arm64/xdfile
