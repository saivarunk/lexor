# ADR 006 — Optional local Pyright through a narrow LSP bridge

Status: accepted.

Use installed Pyright's stdio language server and bundled typeshed through a same-origin WebSocket bridge. The browser implements Monaco completion, hover, signature and diagnostic adapters behind LanguageIntelligenceProvider. The server fixes the document URI/root to an isolated temporary directory and limits allowed methods.

Browser-hosted Pyright integrations can avoid Node at runtime, but typically add large worker/typeshed payloads, virtual filesystem packaging and version-coupling beyond Monaco. A local server reuses upstream Pyright's published language-server entrypoint and typeshed, with fast optional startup and reliable offline standard-library analysis. Its cost is a trusted Node/Pyright installation for semantic features and one local child process while editing.

Pyright is optional. Without it, syntax highlighting, indentation, bracket matching, drafts, running and submissions still work. Monaco word suggestions are disabled so the UI cannot misrepresent word completion as semantic intelligence. Only explicitly imported Monaco editing contributions are bundled; the browser test proves real `math.sqrt` completion, and a real-LSP test verifies hover, signature help and type diagnostics.

Upstream interfaces consulted: [Pyright installation](https://github.com/microsoft/pyright/blob/main/docs/installation.md), [Monaco completion providers](https://microsoft.github.io/monaco-editor/typedoc/interfaces/editor_editor_api.languages.CompletionItemProvider.html).
