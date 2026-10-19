#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
mkdir -p bin
go build -o bin/lexor ./cmd/lexor
./bin/lexor serve &
backend=$!
trap 'kill "$backend" 2>/dev/null || true' EXIT INT TERM
cd web
bun run dev
