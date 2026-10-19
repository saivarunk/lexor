# Verification evidence

Verified locally on 19 October 2026 using macOS ARM64, Go 1.26.9, Bun 1.4.2, Node 22.15, Docker Desktop Engine 29.7.2, Python `3.12.15-alpine3.23`, and Playwright Chromium. Docker was initially unavailable; it was started and every container check below subsequently ran against the real daemon. No submission was executed on the host.

## Executed checks

| Command | Observed result |
| --- | --- |
| `make setup` / `cd web && bun install --frozen-lockfile` | Go modules and locked frontend dependencies install successfully. |
| `make build` | OpenAPI/types generated, TypeScript compiled, Vite assets built and embedded in a CGO-free `bin/lexor` (~21 MB). |
| `make lint` | Go formatting, `go vet`, TypeScript and ESLint pass. |
| `make test` | All Go tests pass; 7 Vitest/Testing Library tests pass. |
| `go test -race ./internal/...` | All tested packages pass with the race detector. |
| `make test-integration` | Real Docker execution scenarios and real Pyright language-service checks pass. |
| `cd web && bun run test:e2e` | 3 browser journeys pass, most recent full run about 25 seconds. |
| `scripts/check-clean-source.sh` | Go tests pass from a fresh temporary source copy without node_modules or built SPA assets, with module networking disabled and the cached pinned Go toolchain. |
| `./bin/lexor problem validate` | All 6 original problems pass schema and semantic validation. |
| `./bin/lexor doctor` | Workspace, SQLite, schemas, container/image, Pyright and Git report Ready locally. |
| `go run ./examples/plugins` | Registers and lists the tracing Python extension. |
| `cd web && bun audit` | No vulnerabilities found. |
| `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` | No vulnerabilities found, including after updating transitive text dependencies. |
| `CGO_ENABLED=0 GOOS=<darwin/linux> GOARCH=<arm64/amd64> go build ...` | All four binary targets compile. Linux outputs are statically linked ELF binaries; macOS outputs are Mach-O. |

Vite reports one expected large lazy Monaco chunk (~705 KB gzip); the initial application chunk is about 85 KB gzip. All worker/fonts/editor assets are local. This warning is a documented size tradeoff, not a claim that Monaco is tiny.

## Browser proof

`web/e2e/workbench.spec.ts` starts the real embedded binary in a temporary workspace, uses the actual Monaco editor and Pyright, and exercises:

1. Dashboard → library → Two Sum → verify starter code → actual `math.sqrt` standard-library suggestion → paste a correct solution → run 2 public tests → submit all 4 tests → Accepted → solved dashboard metric → submission history.
2. Stop the server process, start a new process on the same SQLite workspace and verify the saved code. Submit an incorrect solution and see Wrong Answer. Run an infinite loop and see Time Limit Exceeded while the application stays responsive.
3. Create a problem using the structured form, search/filter it, edit through raw JSON, save and delete it with confirmation.
4. Preview and confirm a local problem import, export allowlisted progress and inspect the written manifest, change theme and reload to prove settings persistence. All browser asset/API requests in that workflow stay on the local server.

The first two items are one test; CRUD and settings/import/export are the other two. Screenshots in `docs/screenshots` are captured from these actual flows. The initial API smoke submissions were removed from the default workspace; test progress stays in disposable workspaces.

## Real sandbox evidence

`internal/execution/sandbox_integration_test.go` checks 11 scenarios:

| Scenario | Observed result |
| --- | --- |
| Correct Two Sum | Accepted, all public/hidden tests; nonzero Python peak RSS reported. |
| Incorrect return | Wrong Answer. |
| Raised exception | Runtime Error. |
| Invalid Python syntax | Compilation Error. |
| Direct OS-level stdout flood | Output Limit Exceeded; does not corrupt the result protocol or hang the server. |
| Allocate 1 GB under the configured 256 MB cap | Memory Limit Exceeded, detected from container OOM state. |
| Return NaN | Runtime Error rather than invalid JSON/false acceptance. |
| Open an outbound socket to a public IP | The socket fails; the solution asserts this restriction and then passes the problem tests. |
| Spawn 64 processes | Process creation fails with EAGAIN under the 32-PID cap; the test cleans up children and then passes. |
| UID, root write, Docker socket checks | Non-root; writing `/host-write-test` fails; Docker socket is absent. No host directories are mounted by the provider. |
| Infinite loop, two-second job budget | Time Limit Exceeded in roughly 2.2 seconds including cleanup. |

The network, PID, write and timeout tests actively try the prohibited action; this evidence is stronger than checking argv flags alone. No active Lexor execution containers remained after the suite. CPU/file/tmpfs restrictions are configured and documented; no kernel-escape resistance or independent CPU usage benchmarking is claimed.

`internal/intelligence/bridge_integration_test.go` connects to the real Pyright process through the bridge and verifies standard-library completion, hover text, signatures and a type-assignment diagnostic. This is separate from the browser completion check, not a fake completion provider.

## State, library and source evidence

Go tests cover schema version rejection, null values, parameter validation, recursive discovery, duplicates, malformed files, symlink escape rejection, CRUD revision conflicts, externally changed files, exact/float/multiset evaluation (including precision and resource-exhaustion edge cases), orchestration and hidden-output redaction. Persistence tests exercise migrations, transaction rollback, restart drafts/history, repeat accepted counts, settings and timezone persistence, distinct-day streaks, DST and gaps.

CLI tests run initialization twice without overwriting a modified problem, preview/import/export a portable problem, reject an ID collision, validate the installed collection, and inspect stats/plugin output. API tests exercise CRUD/filtering, CSRF, Host/Origin/cross-site rejection, body limits, drafts, unsupported languages and JSON-only API fallback.

Source tests cover local JSON/directory imports, a local Git manifest, invalid versions/manifest traversal, collisions and public/private IP policy. An actual guarded HTTPS Git clone of `https://github.com/octocat/Hello-World.git` completed through archive extraction and was correctly rejected because it contains no problem JSON. This smoke check caught and fixed handling of Git's non-executable PAX metadata headers. A hosted valid Lexor pack was not published merely to test remote transport; validation/install code is shared with the tested local packages.

Git tests verify allowlisted files, no draft/database/settings export, symlink rejection, no unrelated staged files in commits, and optional push into a disposable local bare Git repository. No user's hosted repository or credentials were used for a push.

## MVP acceptance map

| Requirement | Evidence |
| --- | --- |
| 1–3. Initialize, serve, embed SPA | CLI tests, production build, browser startup. |
| 4–5. Recursive discovery / create missing directories | Library/config tests and fresh workspaces in CLI/browser suites. |
| 6–7. UI CRUD and filters | Browser CRUD journey, UI search/difficulty test, API company/topic/tag/status filter implementation and lifecycle test. |
| 8–9. Monaco and real Python intelligence | Browser editor/stdlib completion plus real-LSP completion/hover/signature/diagnostic test. |
| 10–14. Isolated execution, no network, runs/submits/evaluation | Actual Docker scenarios; browser observes 2 public / 4 submitted cases; evaluator and redaction tests. |
| 15–16. History and drafts persist | SQLite close/reopen tests and real process restart in Playwright. |
| 17–18. Metrics and streaks | Browser solved metric plus repeat-solve/DST/calendar unit tests. |
| 19. Git export and optional sync | Allowlist, commit isolation and local bare push tests; browser export. |
| 20. Import valid collections | Local JSON/directory/Git tests and actual browser preview/confirm; guarded remote transport smoke. |
| 21. Extensible execution interfaces | Language registry tests, actual Python plugin calls, fake-provider orchestration, compiled extension example. |
| 22. Offline | Embedded assets, cached image with pull=never, local SQLite/library/Pyright; browser local-only requests. |
| 23–24. Automated checks and accurate docs | Commands above; architecture/schema/plugin/security guides, six ADRs and explicit roadmap. |

## Boundaries still requiring external evidence

- GitHub Actions is configured but has not been run or reported green by this local session.
- Cross-compilation passed; full host-runtime/browser verification was performed on macOS ARM64. Linux CI is the next host-runtime gate. Docker Desktop's Linux container restrictions were exercised locally.
- Docker-compatible alternatives, hosted authenticated pushes, a public marketplace, non-Python runners and VM sandboxes are not claimed as verified implementations. See `roadmap.md`.
- Browser structured editing uses JavaScript numbers; author unusually large exact integer fixtures directly as JSON files/CLI rather than round-tripping them through that form. Backend grading preserves exact numeric equality within documented bounds.

App changes are kept in local commits. No push, remote publication or external account action was performed. Repository instruction/hook/graph files that appeared during the run were preserved separately; `graphify update .` refreshes their requested local graph.
