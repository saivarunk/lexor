# ADR 004 — Disposable containers for Python execution

Status: accepted.

Use a Docker-compatible CLI behind SandboxProvider. Supply artifacts through stdin, with no host bind mounts. Each case gets a fresh restricted container; time and output budgets cover the whole job. A child harness writes a result file and a supervisor captures OS stdout independently.

This is practical on macOS and Linux and provides a real OS boundary. Runtime installation/image pull are explicit prerequisites; startup and per-test container overhead are accepted. Fail closed on missing runtime. Containers are not a sufficient boundary for a future hostile multi-tenant hosted service; gVisor/VM providers remain possible behind the same interface.
