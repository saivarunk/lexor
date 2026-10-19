# Lexor implementation plan and evidence

The supplied product specification is the scope. This repository began empty with no commits or user changes. Work proceeds through real user flows, with no cloud dependencies.

## Sequence

1. Foundation and vertical slice: Cobra CLI/config → filesystem/schema library → React list → Monaco practice → container runner/evaluator → SQLite history → dashboard.
2. Library management: structured/raw JSON editor, optimistic concurrent writes, atomic file operations, filters/rescan, six curated original problems.
3. Language intelligence: optional local Pyright over a restricted LSP bridge, offline typeshed completions, diagnostics/hover/signatures. Editor works without it.
4. Portability: validated local/HTTPS/Git imports, allowlisted Git progress export, compile-time extension SDK and example.
5. Hardening: security middleware, bounded sandbox, tests and actual browser workflow, reproducible build, CI and architecture/security/schema/ADR documentation.

## Decisions

- Go modular monolith; standard HTTP mux. Shared application services power CLI and HTTP.
- JSON files authoritative; SQLite holds user state with embedded SQL migrations.
- Docker-compatible runtime with fail-closed execution, no host mounts, network disabled and bounded resources.
- Local Pyright stdio bridge (optional Node dependency), workers bundled with Monaco. No CDN assets.
- Go domain types generate OpenAPI schemas, which generate TypeScript API models.
- Compile-time language/source/sandbox interfaces; no dynamic code loading.

## Verification checklist

- [ ] Go formatting, unit/race and API/persistence/security integration tests
- [ ] Bun typecheck, UI tests and frontend production build
- [ ] Embedded binary CLI init/doctor/library/export/import smoke tests
- [ ] Actual browser CRUD, practice draft and reload/restart persistence
- [ ] Actual sandbox correct/incorrect/error/timeout/output/network/isolation tests
- [ ] Pyright actual semantic completion/diagnostic check
- [ ] CI, documentation, ADRs and accurate limitations

Detailed results are recorded in docs/verification.md. A missing Docker daemon is an environment blocker for execution checks, never permission to execute submissions on the host.
