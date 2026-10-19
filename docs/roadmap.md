# Remaining scope and future work

The implemented release is the single-user local MVP: JSON library/CRUD, Python practice, optional Pyright, container execution, SQLite progress, imports and one-way Git export. Verification is recorded separately; an interface alone is not a claimed implementation.

Deliberately future features:

- A hosted community marketplace, signed packs, registry updates and authenticated private remote Git imports. Public HTTPS Git repositories and local private clones are supported now.
- Non-Python language implementations, stdin/stdout entrypoints, custom evaluators, automatic tagged linked-list/tree/graph materialization.
- Cross-process library transactions and filesystem watching. External edits require rescan; run one Lexor process per workspace.
- Alternative sandbox providers (gVisor/Firecracker/platform VMs), external plugins/RPC/Wasm and multi-tenant remote grading.
- Bidirectional Git merges, automatic scheduling/synchronization, force pushes. Current Git flow is explicit one-way export, optional allowlisted commit and optional ordinary push.
- Rich Markdown rendering, richer testcase form widgets, mobile-first editing and editor preference synchronization across independent workspaces.

Platform verification boundaries: macOS ARM64 runtime and browser behavior are locally exercised. Linux execution and the four macOS/Linux architecture builds have CI jobs; until CI runs, they are configuration plus cross-build evidence, not a claim that every target's full runtime suite was executed. Existing-user credentialed pushes and public publishing are not performed by development tests.
