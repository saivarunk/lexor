import { useEffect } from "react";
import { lazy, Suspense, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowLeft,
  Check,
  ChevronDown,
  Code2,
  GripVertical,
  LoaderCircle,
  Play,
  RotateCcw,
  Save,
  Send,
  Settings2,
  Terminal,
} from "lucide-react";
import {
  api,
  post,
  type Draft,
  type Execution,
  type ProblemView,
  type Preferences,
} from "../services/api";
import { ErrorBox, Loading } from "../components/ui";
import Results from "../components/Results";
const CodeEditor = lazy(() => import("../components/CodeEditor"));
import { useDraft } from "../hooks/useDraft";
import PracticeDescription from "../components/PracticeDescription";
export default function Practice() {
  const { id } = useParams();
  const problem = useQuery({
    queryKey: ["problem", id],
    queryFn: () => api<ProblemView>(`/problems/${id}`),
  });
  const draft = useQuery({
    queryKey: ["draft", id],
    queryFn: () => api<Draft>(`/drafts/${id}?language=python`),
  });
  if (problem.isPending || draft.isPending) return <Loading />;
  if (problem.error || draft.error)
    return <ErrorBox error={problem.error ?? draft.error} />;
  if (!problem.data!.languages.python) {
    return (
      <div className="page">
        <h1>Python is not configured for this problem.</h1>
        <p className="muted">
          Add a Python signature and starter code in the problem editor, or
          register the problem’s language plugin.
        </p>
        <Link className="button secondary" to={`/problems/${id}/edit`}>
          Edit problem
        </Link>
      </div>
    );
  }
  return <Workspace problem={problem.data!} draft={draft.data!} />;
}
function Workspace({ problem, draft }: { problem: ProblemView; draft: Draft }) {
  const { source, setSource, save, saved, saveError, latest, starter } =
    useDraft(problem, draft);
  const [result, setResult] = useState<Execution>();
  const [intelligence, setIntelligence] = useState("Connecting Pyright…");
  const [width, setWidth] = useState(45);
  const [preferenceError, setPreferenceError] = useState<unknown>();
  const [settings, setSettings] = useState(false);
  const [theme, setTheme] = useState(
    localStorage.getItem("lexor:editor-theme") ?? "dark",
  );
  const [fontSize, setFontSize] = useState(
    Number(localStorage.getItem("lexor:font-size") ?? 14),
  );
  const [resultsHeight, setResultsHeight] = useState(280);
  const client = useQueryClient();
  const preferences = useQuery({
    queryKey: ["preferences"],
    queryFn: () => api<Preferences>("/settings"),
  });
  useEffect(() => {
    if (preferences.data) {
      setTheme(preferences.data.editorTheme);
      setFontSize(preferences.data.editorFontSize);
    }
  }, [preferences.data]);
  const savePreferences = (patch: Partial<Preferences>) => {
    void api<Preferences>("/settings", {
      method: "PUT",
      body: JSON.stringify({
        theme: localStorage.getItem("lexor:theme") ?? "dark",
        editorTheme: theme,
        editorFontSize: fontSize,
        ...preferences.data,
        ...patch,
      }),
    })
      .then((value) => client.setQueryData(["preferences"], value))
      .catch(setPreferenceError);
  };
  const execute = useMutation({
    mutationFn: async (mode: "run" | "submit") => {
      const submittedSource = latest.current;
      await save(submittedSource);
      return post<Execution>(`/executions/${mode}`, {
        problemId: problem.id,
        language: "python",
        source: submittedSource,
      });
    },
    onSuccess: (r) => {
      setResult(r);
      void client.invalidateQueries({ queryKey: ["stats"] });
      void client.invalidateQueries({ queryKey: ["problems"] });
      void client.invalidateQueries({ queryKey: ["submissions"] });
    },
  });
  const run = () => {
    if (!execute.isPending) execute.mutate("run");
  };
  const submit = () => {
    if (!execute.isPending) execute.mutate("submit");
  };
  const drag = (e: React.PointerEvent) => {
    e.currentTarget.setPointerCapture(e.pointerId);
  };
  return (
    <div className="practice-workspace">
      <div className="practice-heading">
        <Link
          to="/problems"
          className="icon-button"
          aria-label="Back to problems"
        >
          <ArrowLeft size={18} />
        </Link>
        <span className="muted">Problem library</span>
        <span className="muted">/</span>
        <strong>{problem.title}</strong>
        <div className="toolbar-spacer" />
        <span className="draft-state">
          <Check size={13} />
          {saved}
        </span>
        <button
          className="icon-button"
          title="Save draft"
          onClick={() => void save(source)}
          aria-label="Save draft"
        >
          <Save size={16} />
        </button>
      </div>
      <div
        className="practice-columns"
        style={{
          gridTemplateColumns: `minmax(260px, ${width}fr) 8px minmax(350px, ${100 - width}fr)`,
        }}
      >
        <PracticeDescription problem={problem} />
        <div
          role="separator"
          aria-label="Resize description and editor"
          aria-orientation="vertical"
          tabIndex={0}
          className="split-handle"
          onPointerDown={drag}
          onPointerMove={(e) => {
            if (e.currentTarget.hasPointerCapture(e.pointerId)) {
              const rect =
                e.currentTarget.parentElement!.getBoundingClientRect();
              setWidth(
                Math.min(
                  65,
                  Math.max(25, ((e.clientX - rect.left) / rect.width) * 100),
                ),
              );
            }
          }}
          onKeyDown={(e) => {
            if (e.key === "ArrowLeft") setWidth((v) => Math.max(25, v - 5));
            if (e.key === "ArrowRight") setWidth((v) => Math.min(65, v + 5));
          }}
        >
          <GripVertical size={12} />
        </div>
        <section className="coding-pane">
          <div className="editor-toolbar">
            <Code2 size={15} />
            <strong>solution.py</strong>
            <span className="editor-language">
              Python <ChevronDown size={12} />
            </span>
            <div className="toolbar-spacer" />
            <button
              className="icon-button"
              aria-label="Reset to starter code"
              title="Reset to starter code"
              onClick={() => {
                if (
                  confirm("Replace your current draft with the starter code?")
                )
                  setSource(starter);
              }}
            >
              <RotateCcw size={15} />
            </button>
            <button
              className="icon-button"
              aria-label="Editor preferences"
              onClick={() => setSettings((v) => !v)}
            >
              <Settings2 size={15} />
            </button>
          </div>
          {settings ? (
            <div className="editor-preferences">
              <label>
                Theme{" "}
                <select
                  value={theme}
                  onChange={(e) => {
                    setTheme(e.target.value);
                    savePreferences({ editorTheme: e.target.value });
                    localStorage.setItem("lexor:editor-theme", e.target.value);
                  }}
                >
                  <option value="dark">Dark</option>
                  <option value="light">Light</option>
                </select>
              </label>
              <label>
                Font size{" "}
                <input
                  aria-label="Editor font size"
                  type="number"
                  min="10"
                  max="24"
                  value={fontSize}
                  onChange={(e) => {
                    const n = Math.max(
                      10,
                      Math.min(24, Number(e.target.value)),
                    );
                    setFontSize(n);
                    savePreferences({ editorFontSize: n });
                    localStorage.setItem("lexor:font-size", String(n));
                  }}
                />
              </label>
            </div>
          ) : null}
          <div className="monaco-pane">
            <Suspense fallback={<Loading />}>
              <CodeEditor
                value={source}
                onChange={setSource}
                onRun={run}
                onSubmit={submit}
                onIntelligence={setIntelligence}
                theme={theme}
                fontSize={fontSize}
              />
            </Suspense>
          </div>
          <div className="editor-status">
            <span>
              <span
                className={`online-dot ${intelligence.includes("unavailable") ? "dim" : ""}`}
              />
              {intelligence}
            </span>
            <span>
              UTF-8 <span>4 spaces</span>
            </span>
          </div>
          <div
            role="separator"
            aria-label="Resize results"
            tabIndex={0}
            aria-orientation="horizontal"
            className="results-handle"
            onPointerDown={drag}
            onPointerMove={(e) => {
              if (e.currentTarget.hasPointerCapture(e.pointerId)) {
                const rect =
                  e.currentTarget.parentElement!.getBoundingClientRect();
                setResultsHeight(
                  Math.max(
                    120,
                    Math.min(rect.height - 180, rect.bottom - e.clientY),
                  ),
                );
              }
            }}
          />
          <div className="results-pane" style={{ height: resultsHeight }}>
            <ErrorBox error={execute.error ?? saveError ?? preferenceError} />
            {execute.isPending ? (
              <div className="execution-loading">
                <LoaderCircle size={20} className="spin" />
                <strong>Running in an isolated container…</strong>
                <span>Fresh environment. Network disabled.</span>
              </div>
            ) : result ? (
              <Results key={result.id} result={result} />
            ) : (
              <>
                <div className="result-tabs">
                  <button className="selected">
                    <Terminal size={14} /> Test results
                  </button>
                </div>
                <div className="ready-results">
                  <div className="ready-icon">
                    <Play size={21} />
                  </div>
                  <strong>Ready when you are.</strong>
                  <span>
                    Run the sample tests to see your solution in action.
                  </span>
                  <kbd>⌘ / Ctrl + Enter</kbd>
                </div>
              </>
            )}
          </div>
        </section>
      </div>
      <div className="execution-bar">
        <span>
          <span className="online-dot" /> Isolated execution{" "}
          <span className="muted">· No network access</span>
        </span>
        <div>
          <span className="shortcut-hint">⌘ / Ctrl ↵ to run</span>
          <button
            className="button secondary"
            onClick={run}
            disabled={execute.isPending}
          >
            <Play size={15} /> Run tests
          </button>
          <button
            className="button primary"
            onClick={submit}
            disabled={execute.isPending}
          >
            {execute.isPending ? (
              <LoaderCircle size={15} className="spin" />
            ) : (
              <Send size={15} />
            )}{" "}
            Submit solution
          </button>
        </div>
      </div>
    </div>
  );
}
