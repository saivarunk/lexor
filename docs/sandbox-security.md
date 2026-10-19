# Sandbox security

## Threat model and trust boundaries

Treat solution code and imported problem packages as untrusted. Lexor is a local, single-user application, not a public multi-tenant judge. Trusted components are the Go binary, its bundled runner/supervisor, installed Pyright, the selected container runtime and runtime image, and the host operating system. Keep those patched. A container is the host isolation boundary; removing Python builtins is not one.

On macOS Docker Desktop runs containers inside a Linux VM; on Linux containers share the host kernel. Neither Docker nor Lexor claims protection against a kernel/runtime escape. For stronger multi-tenant isolation, add a gVisor/VM sandbox provider and a separate authorization system before exposing this application to other users.

## Container restrictions

The implemented provider executes the pinned Python image using argv through `exec.CommandContext`, without shell interpolation:

- `--network=none`, no outbound network interface or host networking.
- `--read-only` root filesystem; writable `/work` and `/tmp` are small, noexec, nosuid, nodev tmpfs mounts (16 MB and 8 MB).
- UID/GID `65534:65534`; all capabilities dropped; no new privileges; default runtime seccomp remains enabled.
- One CPU, configured memory and equal memory+swap caps, 32 PIDs, 64 open file descriptors, 16 MB file-size limit and CPU ulimit.
- A wall-time context and a combined output budget span all tests in the job. A separate cleanup context forcibly removes the named container even after cancellation.
- No host bind mounts, no workspace/progress/config directory, no credentials, no Docker socket. The Docker CLI receives only a data-only JSON artifact on stdin.
- `--pull=never`: downloads are an explicit setup operation; no hidden network dependency during execution.

Each test gets an independent container/interpreter. The runner loads the submitted source and JSON inputs inside it. Normal Python stdout/stderr are bounded; a supervisor also caps OS-level output (for example `os.write`). The child writes machine results to a dedicated file, so ordinary user output cannot be mistaken for protocol messages. Nonfinite/non-JSON return values fail. JSON artifacts and entrypoint names are never evaluated as source expressions.

Container memory OOM state maps to Memory Limit Exceeded. Python peak RSS is an informational, runner-reported measurement, not authoritative container accounting; zero means unavailable. Container duration includes startup/cleanup and is not suitable for comparing microbenchmarks.

## Fail-closed behavior

The daemon and pinned image must be available. Missing runtime/daemon/image produces `Sandbox Unavailable` with installation/start/pull instructions. There is no host interpreter fallback in tests, dev mode, CLI, or HTTP. The library/editor and progress remain usable without Docker. A compatible alternative CLI is configurable, but only Docker Desktop was executed in the local verification; alternate runtime support must pass the real integration suite before being advertised as verified.

## Other attack surfaces

- HTTP binds to `127.0.0.1:8080` by default. Host is validated against the configured address/port, Origin must match the request origin, cross-site fetches are denied, and every mutation requires a process-random CSRF token obtained from a same-origin endpoint. Request headers/bodies and source size are capped. CSP denies external resources and framing. CLI clients must fetch the CSRF token too.
- The Vite development proxy rewrites Origin only when the original browser Origin matches its own Host; arbitrary cross-origin requests still fail at the backend.
- Root-confined filesystem operations, safe IDs, exclusive temporary files and revision checks protect CRUD. Discovery skips symlinks. External edits must be atomically written and explicitly rescanned. A hostile process already running as the same OS user can modify local files, the executable, the database or runtime; that is outside this app boundary. No cross-process library lock is claimed.
- Imports accept HTTPS without URL credentials on port 443. Proxy environment settings are ignored. DNS results and redirects are checked at connection time and connections use the validated IP; loopback/private/link-local/reserved ranges are rejected. Git uses a bounded CONNECT proxy under the same policy, with global/system config, hooks/templates, prompts, redirects and alternate protocols disabled. Bare clones are unpacked with explicit archive type/path/size checks. No downloaded code or Git hooks run.
- Git progress export writes only manifest, accepted Python solutions, progress and activity. It excludes drafts, SQLite, settings, tokens and temporary files. Commits use an explicit path list and `--only`, excluding unrelated staged files. Commits are disabled by default; push is explicit and never forced. Existing credentials remain with the user's Git installation.
- Problem descriptions, constraints and imported metadata render as React text. There is no raw HTML/Markdown execution surface.
- The optional LSP bridge allows document sync, completion, hover and signatures only. Browser-supplied filesystem roots/URIs are replaced with one temporary Python document. It rejects execute-command/configuration methods and never accepts executable args or a Python path from the browser. Pyright statically analyzes the document and uses its bundled typeshed; submitted Python is executed only in containers.

## Residual limitations

Local hidden tests are visible to the owner through their JSON files and the editing API. Execution responses hide their expected/actual/error/console data, but this is a presentation policy, not a secret grading service. A determined solution can tamper with runner state inside its own container; host isolation does not establish tamperproof grading for a user controlling the entire machine.

Only one execution and one language-server session run at once. Fixed PID limits, tmpfs size and job timeout can reject unusually process-heavy or large-return problems. The default five-second job budget includes container startup; increase it for a slow runtime, up to the configured maximum of 60 seconds.

No authentication is included. Binding to an external interface is an explicit advanced configuration, and does not turn Lexor into a safe shared service. Keep it on loopback. Platform assumptions and actual evidence are recorded in `verification.md`; a Docker flag being present is not proof of a restriction, so the integration suite actively attempts networking, writes, exhaustion and timeouts.
