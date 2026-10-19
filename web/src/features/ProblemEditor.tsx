import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, Braces, Eye, FileText, Save } from "lucide-react";
import {
  api,
  stripProblem,
  type Problem,
  type ProblemView,
} from "../services/api";
import { ErrorBox, Loading, PageTitle } from "../components/ui";
import ProblemFields from "../components/ProblemFields";
const blank: Problem = {
  schemaVersion: "1.0.0",
  id: "custom.my-problem",
  slug: "my-problem",
  title: "",
  description: "",
  difficulty: "easy",
  organization: {
    topic: "Arrays",
    path: ["Algorithms", "Arrays"],
    tags: [],
    companies: [],
  },
  constraints: [],
  examples: [{ input: { nums: [1, 2] }, output: 3, explanation: "" }],
  languages: {
    python: {
      entrypoint: {
        type: "function",
        name: "solve",
        parameters: [{ name: "nums", type: "list[int]" }],
        returns: "int",
      },
      starterCode: "def solve(nums: list[int]) -> int:\n    pass\n",
    },
  },
  tests: {
    public: [{ name: "Example 1", input: { nums: [1, 2] }, expected: 3 }],
    hidden: [],
  },
  metadata: { source: "local", author: "You", version: "1.0.0" },
};
export default function ProblemEditor() {
  const { id } = useParams();
  const q = useQuery({
    queryKey: ["problem", id],
    queryFn: () => api<ProblemView>(`/problems/${id}`),
    enabled: !!id,
  });
  if (id && q.isPending) return <Loading />;
  if (q.error) return <ErrorBox error={q.error} />;
  return <EditorForm key={id ?? "new"} existing={q.data} />;
}
function EditorForm({ existing }: { existing?: ProblemView }) {
  const [problem, setProblem] = useState<Problem>(
    existing ? stripProblem(existing) : structuredClone(blank),
  );
  const [mode, setMode] = useState("form");
  const [raw, setRaw] = useState(JSON.stringify(problem, null, 2));
  const [error, setError] = useState<unknown>();
  const [busy, setBusy] = useState(false);
  const [dirty, setDirty] = useState(false);
  const navigate = useNavigate();
  const client = useQueryClient();
  const [parts, setParts] = useState(() => ({
    examples: JSON.stringify(problem.examples, null, 2),
    public: JSON.stringify(problem.tests.public, null, 2),
    hidden: JSON.stringify(problem.tests.hidden, null, 2),
    entrypoint: JSON.stringify(
      problem.languages.python?.entrypoint ?? {},
      null,
      2,
    ),
    comparison: JSON.stringify(
      problem.comparison ?? { mode: "exact" },
      null,
      2,
    ),
  }));
  const change = (patch: Partial<Problem>) => {
    setProblem((p) => ({ ...p, ...patch }));
    setDirty(true);
  };
  function assemble(): Problem {
    return {
      ...problem,
      examples: JSON.parse(parts.examples),
      tests: {
        public: JSON.parse(parts.public),
        hidden: JSON.parse(parts.hidden),
      },
      languages: {
        ...problem.languages,
        python: {
          ...problem.languages.python,
          entrypoint: JSON.parse(parts.entrypoint),
        },
      },
      comparison: JSON.parse(parts.comparison),
    };
  }
  async function switchMode(next: string) {
    try {
      if (mode === "raw") {
        const parsed = await api<Problem>("/problems/validate", {
          method: "POST",
          body: raw,
        });
        setProblem(parsed);
        setParts({
          examples: JSON.stringify(parsed.examples, null, 2),
          public: JSON.stringify(parsed.tests.public, null, 2),
          hidden: JSON.stringify(parsed.tests.hidden, null, 2),
          entrypoint: JSON.stringify(
            parsed.languages.python?.entrypoint ?? {},
            null,
            2,
          ),
          comparison: JSON.stringify(
            parsed.comparison ?? { mode: "exact" },
            null,
            2,
          ),
        });
      } else {
        setRaw(JSON.stringify(assemble(), null, 2));
      }
      setError(undefined);
      setMode(next);
    } catch (e) {
      setError(e);
    }
  }
  useEffect(() => {
    const fn = (e: BeforeUnloadEvent) => {
      if (dirty) {
        e.preventDefault();
        e.returnValue = "";
      }
    };
    window.addEventListener("beforeunload", fn);
    return () => window.removeEventListener("beforeunload", fn);
  }, [dirty]);
  async function save() {
    setBusy(true);
    setError(undefined);
    try {
      if (mode !== "form") JSON.parse(raw);
      const body = mode === "form" ? JSON.stringify(assemble()) : raw;
      const data = await api<ProblemView>(
        `/problems${existing ? `/${existing.id}` : ""}`,
        {
          method: existing ? "PUT" : "POST",
          headers: existing ? { "If-Match": existing.revision } : {},
          body,
        },
      );
      setDirty(false);
      client.setQueryData(["problem", data.id], data);
      await client.invalidateQueries({ queryKey: ["problems"] });
      await client.invalidateQueries({ queryKey: ["stats"] });
      navigate("/problems");
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      <Link className="back-link" to="/problems">
        <ArrowLeft size={14} /> Back to library
      </Link>
      <PageTitle
        eyebrow="YOUR LIBRARY, YOUR RULES"
        title={existing ? "Edit problem" : "Create a new problem"}
        description="Portable JSON on disk. A great challenge in your library."
        action={
          <button className="button primary" disabled={busy} onClick={save}>
            <Save size={15} />
            {busy ? "Validating & saving…" : "Save problem"}
          </button>
        }
      />
      <div className="editor-mode-tabs">
        <button
          className={mode === "form" ? "selected" : ""}
          onClick={() => switchMode("form")}
        >
          <FileText size={15} /> Structured editor
        </button>
        <button
          className={mode === "raw" ? "selected" : ""}
          onClick={() => switchMode("raw")}
        >
          <Braces size={15} /> Raw JSON
        </button>
        <button
          className={mode === "preview" ? "selected" : ""}
          onClick={() => switchMode("preview")}
        >
          <Eye size={15} /> JSON preview
        </button>
        <span className="toolbar-spacer" />
        <span className="muted small">Schema v1.0.0 · Atomic file writes</span>
      </div>
      <ErrorBox error={error} />
      {mode === "raw" ? (
        <textarea
          className="raw-editor"
          spellCheck={false}
          aria-label="Raw problem JSON"
          value={raw}
          onChange={(e) => {
            setRaw(e.target.value);
            setDirty(true);
          }}
        />
      ) : mode === "preview" ? (
        <pre className="json-preview">{raw}</pre>
      ) : (
        <ProblemFields
          problem={problem}
          change={change}
          parts={parts}
          setParts={setParts}
          setDirty={setDirty}
          existing={!!existing}
        />
      )}
      <div className="form-bottom">
        <span className="muted">
          {dirty ? "Unsaved changes" : "Your problem is up to date"}
        </span>
        <button className="button primary" disabled={busy} onClick={save}>
          <Save size={15} /> Save problem
        </button>
      </div>
    </>
  );
}
