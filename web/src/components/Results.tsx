import { Check, ChevronRight, Clock3, Terminal } from "lucide-react";
import { useState } from "react";
import type { Execution } from "../services/api";
import { Badge } from "./ui";
export default function Results({ result }: { result: Execution }) {
  const [tab, setTab] = useState("results");
  const [selected, setSelected] = useState(0);
  const test = result.results[selected];
  return (
    <div className="results-content">
      <div className="result-tabs">
        <button
          className={tab === "results" ? "selected" : ""}
          onClick={() => setTab("results")}
        >
          Test results{" "}
          <Badge kind={result.status === "Accepted" ? "easy" : "warning"}>
            {result.passed}/{result.total}
          </Badge>
        </button>
        <button
          className={tab === "console" ? "selected" : ""}
          onClick={() => setTab("console")}
        >
          <Terminal size={13} /> Console
        </button>
        <div className="toolbar-spacer" />
        <span>
          <Clock3 size={12} /> {Math.round(result.durationMs)} ms
        </span>
      </div>
      {tab === "console" ? (
        <pre className="console-output">
          {result.stdout || result.stderr
            ? `${result.stdout}${result.stderr}`
            : "No console output."}
        </pre>
      ) : (
        <>
          <div
            className={`result-status ${result.status === "Accepted" ? "green-text" : "warning-text"}`}
          >
            {result.status === "Accepted" ? (
              <Check size={19} />
            ) : (
              <ChevronRight size={19} />
            )}
            <strong data-testid="execution-status">{result.status}</strong>
            <span>
              {result.passed} of {result.total} tests passed
            </span>
          </div>
          {result.stderr ? (
            <div className="runtime-diagnostic">{result.stderr}</div>
          ) : null}
          <div className="test-case-tabs">
            {result.results.map((t, i) => (
              <button
                key={i}
                className={selected === i ? "selected" : ""}
                onClick={() => setSelected(i)}
              >
                <span
                  className={`test-dot ${t.status === "Accepted" ? "passed" : "failed"}`}
                />
                {t.hidden ? `Hidden ${i + 1}` : `Case ${i + 1}`}
              </button>
            ))}
          </div>
          {test ? (
            <div className="test-detail">
              <span className="small muted">
                {test.name} · {test.status} · {Math.round(test.durationMs)} ms
              </span>
              {test.hidden ? (
                <p className="muted">
                  Hidden-test values and console output are redacted.
                </p>
              ) : (
                <>
                  <div className="result-values">
                    <div>
                      <label>YOUR OUTPUT</label>
                      <pre>{JSON.stringify(test.actual, null, 2) ?? "—"}</pre>
                    </div>
                    <div>
                      <label>EXPECTED</label>
                      <pre>{JSON.stringify(test.expected, null, 2) ?? "—"}</pre>
                    </div>
                  </div>
                  {test.error ? (
                    <pre className="runtime-diagnostic">{test.error}</pre>
                  ) : null}
                </>
              )}
            </div>
          ) : null}
        </>
      )}
    </div>
  );
}
