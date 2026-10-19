import { useState } from "react";
import { Link } from "react-router-dom";
import { Braces, FileText } from "lucide-react";
import { Badge } from "./ui";
import type { ProblemView } from "../services/api";
export default function PracticeDescription({
  problem,
}: {
  problem: ProblemView;
}) {
  const [tab, setTab] = useState("description");
  return (
    <section className="description-pane">
      <div className="pane-tabs">
        <button
          className={tab === "description" ? "selected" : ""}
          onClick={() => setTab("description")}
        >
          <FileText size={14} /> Description
        </button>
        <button
          className={tab === "tests" ? "selected" : ""}
          onClick={() => setTab("tests")}
        >
          <Braces size={14} /> Sample tests
        </button>
        <Link
          to={`/submissions?problemId=${problem.id}`}
          className="submissions-link"
        >
          Submissions
        </Link>
      </div>
      <div className="description-scroll">
        <div className="eyebrow">{problem.organization.topic}</div>
        <h1>{problem.title}</h1>
        <div className="problem-meta">
          <Badge kind={problem.difficulty}>{problem.difficulty}</Badge>
          <span>Python 3.12</span>
          <span>Function</span>
        </div>
        {tab === "description" ? (
          <>
            <p className="problem-description">{problem.description}</p>
            <div className="description-tags">
              {problem.organization.tags.map((t) => (
                <span className="tag" key={t}>
                  {t}
                </span>
              ))}
            </div>
            <h3>Examples</h3>
            {problem.examples.map((example, i) => (
              <div className="example" key={i}>
                <div className="example-label">EXAMPLE {i + 1}</div>
                <pre>
                  <span>Input: </span>
                  {JSON.stringify(example.input, null, 2)}
                  {"\n"}
                  <span>Output: </span>
                  {JSON.stringify(example.output)}
                </pre>
                <p>{example.explanation}</p>
              </div>
            ))}
            <h3>Constraints</h3>
            <ul className="constraints">
              {problem.constraints.map((c) => (
                <li key={c}>
                  <code>{c}</code>
                </li>
              ))}
            </ul>
            <div className="problem-source">
              {problem.metadata.author} · {problem.metadata.source} · v
              {problem.metadata.version}
            </div>
          </>
        ) : (
          problem.tests.public.map((test, i) => (
            <div className="example" key={i}>
              <h4>{test.name}</h4>
              <pre>{JSON.stringify(test.input, null, 2)}</pre>
              <div className="small muted">EXPECTED</div>
              <pre>{JSON.stringify(test.expected)}</pre>
            </div>
          ))
        )}
      </div>
    </section>
  );
}
