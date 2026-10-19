#!/bin/sh
# Verify source-only Go tests with no generated frontend or installed JS deps.
set -eu
root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
task_go="$(go env GOROOT)/bin/go"
task_dir=$(mktemp -d)
trap 'rm -rf "$task_dir"' EXIT INT TERM
cd "$root"
tar --exclude=.git --exclude=node_modules --exclude=dist --exclude=.lexor --exclude=lexor.yaml --exclude=bin --exclude=test-results --exclude=graphify-out --exclude=.claude --exclude=.codex -cf - . | (cd "$task_dir" && tar -xf -)
mkdir -p "$task_dir/web/dist"
touch "$task_dir/web/dist/.gitkeep"
cd "$task_dir"
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off "$task_go" test -count=1 ./...
