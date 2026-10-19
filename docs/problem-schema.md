# Portable problem schema v1

The authoritative contract is `schemas/problem/v1.schema.json` (JSON Schema draft 2020-12). Every discovered and imported problem is validated with the embedded copy and the maintained Go jsonschema validator. The supported `schemaVersion` is exactly `1.0.0`; unsupported versions are rejected with an instance path in the validation error.

Required fields are `schemaVersion`, stable `id`, `slug`, `title`, `description`, `difficulty`, `organization`, `constraints`, `examples`, `languages`, `tests`, and `metadata`. Unknown properties are rejected. IDs use lowercase alphanumerics separated by dots/hyphens, at most 128 characters; slugs use lowercase alphanumerics/hyphens. Difficulty is `easy`, `medium` or `hard`.

```json
{
  "schemaVersion": "1.0.0",
  "id": "custom.sum",
  "slug": "sum",
  "title": "Add a sequence",
  "description": "Return the sum of the supplied numbers.",
  "difficulty": "easy",
  "organization": {"topic":"Arrays","path":["Algorithms","Arrays"],"tags":["array"],"companies":[]},
  "constraints": ["The sequence may be empty."],
  "examples": [{"input":{"nums":[1,2]},"output":3,"explanation":"1 + 2 = 3."}],
  "languages": {
    "python": {
      "entrypoint": {"type":"function","name":"solve","parameters":[{"name":"nums","type":"list[int]"}],"returns":"int"},
      "starterCode": "def solve(nums: list[int]) -> int:\n    pass\n"
    }
  },
  "tests": {
    "public":[{"name":"A pair","input":{"nums":[1,2]},"expected":3}],
    "hidden":[{"name":"Empty","input":{"nums":[]},"expected":0}]
  },
  "comparison": {"mode":"exact"},
  "metadata": {"source":"local","author":"You","version":"1.0.0"}
}
```

Each language has its own entrypoint and starter code. Parameter names must be unique valid identifiers and every test's named input keys must match the declared parameters. Parameters are passed positionally in declaration order. Type strings are human-readable documentation, not executable expressions or runtime casts. Entrypoint names are validated, resolved from the submitted module namespace, and never interpolated into generated source. Only function entrypoints are supported now; a future version can introduce stdin/stdout contracts without reinterpreting v1 functions.

## Values and comparison

JSON values are supported as input members or expected output: null, bool, number, string, nested arrays and objects. Null is distinct from a missing field. Python inputs decode with `json.load`; results must serialize as finite JSON, so NaN, infinity, generators and arbitrary Python objects fail with a runtime diagnostic.

Comparison can be specified for the whole problem or overridden per test:

| Mode | Meaning |
| --- | --- |
| `exact` | Recursive equality; object key order is irrelevant, array order matters. JSON numbers compare by numeric value. |
| `float` | Recursive numeric comparison: `abs(a-b) <= max(absTolerance, relTolerance * max(abs(a),abs(b)))`. Defaults to 1e-9 absolute/relative when both are zero. Non-numeric leaves still compare exactly. |
| `unordered` | The first array at each object branch is a multiset; duplicate counts matter. Each element's nested arrays preserve their order. Objects use canonical JSON key ordering. |

Field names are `absoluteTolerance` and `relativeTolerance`; both must be nonnegative. Evaluation rejects nonfinite/oversized numeric literals (4096 characters or exponent magnitude over 4096); problem validation also requires numbers accepted by the JSON Schema implementation. Exact grading preserves integer precision beyond JavaScript’s safe-integer range, but the structured browser form uses JavaScript numbers: author such values in JSON files/CLI and avoid round-tripping them through that form.

Custom validation code is not accepted inside metadata or test specifications. Future custom evaluators must be explicitly registered trusted extensions.

Matrices are ordinary nested arrays. For future marketplace interchange, reserve these data-only tagged representations (v1 passes them unchanged as dictionaries; automatic materialization is deferred):

```json
{"$type":"linked-list","values":[1,2,3],"cycleAt":null}
{"$type":"binary-tree","levelOrder":[1,null,2,3]}
{"$type":"graph","directed":false,"nodes":[{"id":"a","value":1}],"edges":[]}
{"$type":"matrix","rows":2,"cols":2,"values":[[1,2],[3,4]]}
```

Tree nulls denote absent children in breadth-first order. Graph IDs are stable strings; edges will be `{ "from":"a", "to":"b" }` entries. Custom objects need an explicit problem contract; Lexor does not assume all structures are primitive arrays or instantiate arbitrary classes.

## Files and revisions

Filesystem folders and logical topic hierarchy are independent. A new `custom.sum` problem creates `custom/sum.json`; a discovered file can live elsewhere and retains that path on update. CRUD saves only the portable Problem object. Progress, revision, file path and solved status are view metadata, not fields saved into the JSON specification.

GET returns a revision/ETag; PUT and DELETE require `If-Match` with that revision. A stale/missing revision returns 409. Atomic rename prevents partial files for application-controlled writes. An external editor should also write atomically; rescan after editing to refresh the index. Multiple simultaneously running Lexor processes against one library are not supported; external writes are detected at revision comparison but no cross-process filesystem transaction is claimed.

## Evolution and collection provenance

Schema version and problem content version are independent. V1 readers reject future schema versions rather than dropping fields. A future v2 migration must be explicit, preserve IDs and maintain source files/backups. Additive optional semantics require a documented reader version before import support is widened.

Collections may contain a JSON problem, an array of problems, recursively discovered JSON files, or a `lexor-pack.json` manifest:

```json
{"schemaVersion":"1.0.0","name":"Essentials","description":"Original exercises","version":"0.1.0","problems":["arrays/two-sum.json"]}
```

A manifest limits installation to its listed relative JSON paths. Unsupported manifests, duplicate IDs, collisions and unsafe paths are rejected. The preview SHA-256 hashes canonical problem content; UI confirmation re-resolves and compares that digest before installation. Source URL/path, digest, import time and IDs are recorded in SQLite. Self-declared author/source metadata is attribution, not an authenticity guarantee. Imports never execute scripts or automatically overwrite local content. Updating installed packs is a deliberate future feature; edit or remove local problems before a replacement import.
