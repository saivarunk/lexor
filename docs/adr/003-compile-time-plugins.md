# ADR 003 — Compile-time extension registration

Status: accepted.

Version the Go extension contract in pkg/pluginapi. Register trusted language implementations during app composition. Keep sandbox, evaluation and source resolution as separate boundaries. Python is implemented using the same language interface that another language would use.

This preserves portable single-binary deployment without Go native plugin ABI/platform problems. Adding a language currently requires rebuilding Lexor. External RPC/Wasm, signatures and dynamic marketplace loading are future work, not partially active runtime paths.
