# Compile-time extension development

The public contract lives in `pkg/pluginapi` and is versioned independently (`Version = "1.0.0"`). Lexor registers trusted Go implementations when composing the application; it does not load shared libraries, arbitrary subprocess plugins, downloaded validators or marketplace code.

## Add a language

Implement `LanguagePlugin`:

- `ID`, `DisplayName`, `Extensions` describe the language.
- `Prepare(context, PrepareRequest)` returns an `Artifact` containing named file bytes and an argv command. Names must be simple basenames. Source, the language-specific signature, and a typed test case are separate inputs; serialize test data, never interpolate it into executable code.
- `Decode(SandboxResult)` parses bounded machine results and returns the actual JSON value plus console output/errors. A `RunnerError` supplies the language-neutral execution status. An optional `MeasuredRunner` reports process peak memory.

Then register the instance in `internal/app.Open` and add a compatible runtime image/provider configuration. Add a problem with a matching language ID. The engine resolves a problem's declared language and registry entry, prepares each case, calls the sandbox, and invokes the decoder/evaluator; no Python branches are needed in the engine.

`examples/plugins/main.go` wraps the existing Python plugin with preparation tracing while retaining its sandbox requirements:

```sh
go run ./examples/plugins
```

This demonstrates an extension registration, not an installed dynamic plugin. Python is the only built-in practice language. The example imports internal implementation packages because it is part of the Lexor module; a separate module can implement the public `pluginapi` interfaces but application composition remains an explicit source change.

## Other boundaries

- `SandboxProvider`: `ID`, prerequisite `Available`, and bounded `Execute(context, Artifact, Limits)`. It must enforce network, memory, CPU, process, filesystem, output and wall-time constraints or fail closed. Cancellation must remove the execution environment. Container is the implemented provider; gVisor/VM implementations remain future work.
- `Evaluator`: compares JSON values with `Comparison` options under a context. Exact/float/unordered are implemented by `execution.JSONEvaluator`; register/select a future custom evaluator through trusted application code, not problem metadata.
- `registry.ProblemSource`: resolves a source reference into a validated data package. `Resolver` handles local/HTTPS JSON and guarded Git imports; installation is separate and collision-safe.
- `syncgit.Exporter`: generates an explicit file allowlist from the state store, previews it, writes it, and optionally commits/pushes those paths. A future sync adapter should preserve this boundary and avoid database/config exports.
- Frontend `LanguageIntelligenceProvider`: `languageId`, `initialize(monaco, model)`, `dispose()`. Pyright implements it independently of the editor component; Monaco keeps working when initialization fails.

## Lifecycle and failures

Registration happens once before requests; duplicate or empty IDs fail startup. One execution runs at a time; a busy engine returns 409. Contexts carry cancellation through prepare/run/evaluate/persist. Each test gets an independent sandbox; artifacts are not reused across tests. Runner errors become execution statuses, transport/provider errors become diagnostics, and execution results are persisted before returning success. Do not log submitted source or credentials in an extension.

Exercise a new plugin with domain/evaluator tests, fake-sandbox orchestration tests, and a tagged real-backend integration test. Include malicious code, invalid output, resource exhaustion and cancellation. Never implement a test-only host-execution mode.
