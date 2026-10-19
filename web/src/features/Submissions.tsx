import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link, useSearchParams } from "react-router-dom";
import { ArrowLeft, ArrowUpRight, Code2 } from "lucide-react";
import { api, formatDate, type Execution } from "../services/api";
import { Empty, ErrorBox, Loading, PageTitle, Status } from "../components/ui";
import Results from "../components/Results";
export default function Submissions() {
  const [params] = useSearchParams();
  const [id, setId] = useState<string>();
  const [history, setHistory] = useState(false);
  const q = useQuery({
    queryKey: ["submissions", params.get("problemId"), history],
    queryFn: () =>
      api<Execution[]>(
        `/${history ? "executions" : "submissions"}?${params.toString()}`,
      ),
  });
  const detail = useQuery({
    queryKey: ["submission", id],
    queryFn: () => api<Execution>(`/submissions/${id}`),
    enabled: !!id,
  });
  return (
    <>
      <PageTitle
        eyebrow="A RECORD OF YOUR EFFORT"
        title="Every attempt teaches you."
        description="Revisit your solutions, inspect test results, and see how far you’ve come."
        action={
          <button
            className="button secondary"
            onClick={() => setHistory((v) => !v)}
          >
            {history ? "Show submissions" : "Include test runs"}
          </button>
        }
      />
      <ErrorBox error={q.error ?? detail.error} />
      {id ? (
        <section className="card submission-detail">
          <button className="text-link" onClick={() => setId(undefined)}>
            <ArrowLeft size={14} /> All submissions
          </button>
          {detail.isPending ? (
            <Loading />
          ) : detail.data ? (
            <>
              <div className="submission-detail-heading">
                <h2>{detail.data.problemId}</h2>
                <Status value={detail.data.status} />
                <Link
                  className="button secondary"
                  to={`/practice/${detail.data.problemId}`}
                >
                  Open in editor <ArrowUpRight size={14} />
                </Link>
              </div>
              <span className="small muted">
                {formatDate(detail.data.createdAt)} · {detail.data.language} ·{" "}
                {detail.data.mode} ·{" "}
                {detail.data.memoryBytes
                  ? `${Math.round(detail.data.memoryBytes / 1024)} KB memory`
                  : "Memory not measured"}
              </span>
              <pre className="source-preview">{detail.data.source}</pre>
              <button
                className="button secondary"
                onClick={() => {
                  if (confirm("Restore this source as your practice draft?")) {
                    const key = `lexor:draft:v1:${detail.data!.problemId}:${detail.data!.language}`;
                    localStorage.setItem(
                      key,
                      JSON.stringify({
                        source: detail.data!.source,
                        updatedAt: new Date().toISOString(),
                      }),
                    );
                    window.location.assign(
                      `/practice/${detail.data!.problemId}`,
                    );
                  }
                }}
              >
                Restore as draft
              </button>
              <Results key={detail.data.id} result={detail.data} />
            </>
          ) : null}
        </section>
      ) : q.isPending ? (
        <Loading />
      ) : q.data?.length ? (
        <section className="card">
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>Problem</th>
                  <th>Result</th>
                  <th>Tests</th>
                  <th>Runtime</th>
                  <th>Language</th>
                  <th>Submitted</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {q.data.map((r) => (
                  <tr key={r.id}>
                    <td>
                      <button
                        className="submission-title"
                        onClick={() => setId(r.id)}
                      >
                        <Code2 size={15} />
                        {r.problemId}
                      </button>
                      <span className="file-path">
                        {r.mode === "run" ? "Sample test run" : "All tests"}
                      </span>
                    </td>
                    <td>
                      <Status value={r.status} />
                    </td>
                    <td>
                      {r.passed} / {r.total}
                    </td>
                    <td>{Math.round(r.durationMs)} ms</td>
                    <td>Python</td>
                    <td className="date-cell">{formatDate(r.createdAt)}</td>
                    <td>
                      <button
                        className="icon-button"
                        aria-label={`Inspect submission ${r.id}`}
                        onClick={() => setId(r.id)}
                      >
                        <ArrowUpRight size={15} />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      ) : (
        <section className="card">
          <Empty title="Your story starts with a solution.">
            Run a few tests, then submit your solution. Your history will appear
            here.
            <br />
            <Link className="button primary" to="/problems">
              Find a challenge <ArrowUpRight size={14} />
            </Link>
          </Empty>
        </section>
      )}
    </>
  );
}
