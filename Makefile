SHELL := /bin/sh
GO ?= go
BUN ?= bun
.PHONY: setup dev build generate test test-integration test-e2e lint fmt clean
setup:
	$(GO) mod download
	cd web && $(BUN) install --frozen-lockfile
	@echo 'For isolated execution: docker pull python:3.12.15-alpine3.23'
dev:
	@./scripts/dev.sh
build: generate
	cd web && $(BUN) run build
	mkdir -p bin
	CGO_ENABLED=0 $(GO) build -trimpath -o bin/lexor ./cmd/lexor
generate:
	$(GO) run ./cmd/lexor openapi > docs/openapi.json
	cd web && $(BUN) run generate
test:
	$(GO) test ./...
	cd web && $(BUN) run test
test-integration:
	LEXOR_DOCKER_TESTS=1 LEXOR_PYRIGHT_TESTS=1 $(GO) test -tags=integration -count=1 -v ./internal/execution/... ./internal/intelligence/...
test-e2e: build
	cd web && $(BUN) run test:e2e
lint:
	@test -z "$$(gofmt -l cmd internal pkg schemas problems examples web/embed.go)" || (echo 'Run make fmt'; exit 1)
	$(GO) vet ./...
	cd web && $(BUN) run typecheck && $(BUN) run lint
fmt:
	gofmt -w cmd internal pkg schemas/*.go problems/*.go web/*.go examples
	cd web && $(BUN) run fmt
clean:
	rm -rf bin web/dist/assets web/dist/index.html web/tsconfig.tsbuildinfo
