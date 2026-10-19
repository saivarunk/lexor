import { useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import {
  ArrowDownUp,
  ArrowRight,
  ArrowUpRight,
  Check,
  Code2,
  FilePlus2,
  Folder,
  LayoutList,
  Pencil,
  Plus,
  RefreshCw,
  Search,
  SlidersHorizontal,
  Trash2,
  X,
} from "lucide-react";
import { api, formatDate, post, type ProblemView } from "../services/api";
import {
  Badge,
  Empty,
  ErrorBox,
  Loading,
  PageTitle,
  Status,
} from "../components/ui";
export const filterProblems = (
  items: ProblemView[],
  f: Record<string, string>,
) =>
  items.filter(
    (p) =>
      (!f.search || p.title.toLowerCase().includes(f.search.toLowerCase())) &&
      (!f.difficulty || p.difficulty === f.difficulty) &&
      (!f.topic || p.organization.topic === f.topic) &&
      (!f.tag || p.organization.tags.includes(f.tag)) &&
      (!f.company || p.organization.companies.includes(f.company)) &&
      (!f.status || p.status === f.status),
  );
export default function Problems() {
  const [filters, setFilters] = useState<Record<string, string>>({});
  const [sort, setSort] = useState("title");
  const [error, setError] = useState<unknown>();
  const [advanced, setAdvanced] = useState(false);
  const client = useQueryClient();
  const query = useQuery({
    queryKey: ["problems", {}],
    queryFn: () => api<ProblemView[]>("/problems"),
  });
  const searchRef = useRef<HTMLInputElement>(null);
  useEffect(() => {
    const fn = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        searchRef.current?.focus();
      }
    };
    window.addEventListener("keydown", fn);
    return () => window.removeEventListener("keydown", fn);
  }, []);
  const all = query.data ?? [];
  const items = filterProblems(all, filters).toSorted((a, b) =>
    sort === "difficulty"
      ? ({ easy: 0, medium: 1, hard: 2 }[a.difficulty as "easy"] ?? 0) -
        ({ easy: 0, medium: 1, hard: 2 }[b.difficulty as "easy"] ?? 0)
      : sort === "recent"
        ? (b.lastAttemptedAt ?? "").localeCompare(a.lastAttemptedAt ?? "")
        : a.title.localeCompare(b.title),
  );
  const set = (key: string, value: string) =>
    setFilters((f) => ({ ...f, [key]: value }));
  const topics = [...new Set(all.map((p) => p.organization.topic))];
  async function remove(p: ProblemView) {
    if (
      !confirm(
        `Delete “${p.title}”? Its JSON file will be removed. Submission history will be kept.`,
      )
    )
      return;
    try {
      await api(`/problems/${p.id}`, {
        method: "DELETE",
        headers: { "If-Match": p.revision },
        body: "{}",
      });
      await client.invalidateQueries({ queryKey: ["problems"] });
      await client.invalidateQueries({ queryKey: ["stats"] });
    } catch (e) {
      setError(e);
    }
  }
  async function rescan() {
    try {
      const r = await post<{ issues: string[] }>("/problems/rescan");
      setError(r.issues.length ? new Error(r.issues.join("\n")) : undefined);
      await client.invalidateQueries({ queryKey: ["problems"] });
    } catch (e) {
      setError(e);
    }
  }
  return (
    <>
      <PageTitle
        eyebrow="THE PROBLEM LIBRARY"
        title="Find your next challenge."
        description="Explore the fundamentals, stretch your thinking, and make it click."
        action={
          <Link className="button primary" to="/problems/new">
            <Plus size={16} /> New problem
          </Link>
        }
      />
      <div className="library-summary">
        <div>
          <span className="summary-icon">
            <Code2 size={20} />
          </span>
          <strong>{all.length}</strong>
          <span>curated problems</span>
        </div>
        <div>
          <Check size={17} className="green-text" />
          <strong>{all.filter((p) => p.status === "solved").length}</strong>
          <span>solved by you</span>
        </div>
        <div>
          <Folder size={17} />
          <strong>{topics.length}</strong>
          <span>topics to explore</span>
        </div>
        <span className="library-summary-end">
          <span className="online-dot" /> Your library. Your pace.
        </span>
      </div>
      <div className="topic-tabs">
        <button
          className={!filters.topic ? "selected" : ""}
          onClick={() => set("topic", "")}
        >
          <LayoutList size={15} /> All problems <span>{all.length}</span>
        </button>
        {topics.map((t) => (
          <button
            key={t}
            className={filters.topic === t ? "selected" : ""}
            onClick={() => set("topic", t)}
          >
            {t}
            <span>{all.filter((p) => p.organization.topic === t).length}</span>
          </button>
        ))}
      </div>
      <section className="card library-card">
        <div className="filter-toolbar">
          <div className="search-input">
            <Search size={16} />
            <input
              ref={searchRef}
              aria-label="Search problems"
              placeholder="Search problems…"
              value={filters.search ?? ""}
              onChange={(e) => set("search", e.target.value)}
            />
            <kbd>⌘ K</kbd>
          </div>
          <select
            aria-label="Filter difficulty"
            value={filters.difficulty ?? ""}
            onChange={(e) => set("difficulty", e.target.value)}
          >
            <option value="">Difficulty</option>
            {["easy", "medium", "hard"].map((v) => (
              <option key={v}>{v}</option>
            ))}
          </select>
          <select
            aria-label="Filter status"
            value={filters.status ?? ""}
            onChange={(e) => set("status", e.target.value)}
          >
            <option value="">Status</option>
            <option value="unsolved">Todo</option>
            <option value="attempted">Attempted</option>
            <option value="solved">Solved</option>
          </select>
          <button
            className={`button subtle ${advanced ? "selected" : ""}`}
            onClick={() => setAdvanced((v) => !v)}
          >
            <SlidersHorizontal size={15} /> Filters
          </button>
          <div className="toolbar-spacer" />
          <select
            aria-label="Sort problems"
            value={sort}
            onChange={(e) => setSort(e.target.value)}
          >
            <option value="title">Title A–Z</option>
            <option value="difficulty">Difficulty</option>
            <option value="recent">Last attempted</option>
          </select>
          <button
            className="icon-button"
            aria-label="Rescan library"
            title="Rescan JSON files"
            onClick={rescan}
          >
            <RefreshCw size={15} />
          </button>
        </div>
        {advanced ? (
          <div className="advanced-filters">
            <Select
              label="Tags"
              values={[...new Set(all.flatMap((p) => p.organization.tags))]}
              value={filters.tag ?? ""}
              onChange={(v) => set("tag", v)}
            />
            <Select
              label="Companies"
              values={[
                ...new Set(all.flatMap((p) => p.organization.companies)),
              ]}
              value={filters.company ?? ""}
              onChange={(v) => set("company", v)}
            />
            <button className="button subtle" onClick={() => setFilters({})}>
              <X size={13} /> Clear filters
            </button>
          </div>
        ) : null}
        <ErrorBox error={error ?? query.error} />
        {query.isPending ? (
          <Loading />
        ) : (
          <div className="table-scroll">
            <table className="problem-table">
              <thead>
                <tr>
                  <th className="number-cell">#</th>
                  <th>
                    Problem <ArrowDownUp size={12} />
                  </th>
                  <th>Difficulty</th>
                  <th>Topic & tags</th>
                  <th>Companies</th>
                  <th>Status</th>
                  <th>Last attempted</th>
                  <th aria-label="Actions" />
                </tr>
              </thead>
              <tbody>
                {items.map((p, i) => (
                  <tr key={p.id}>
                    <td className="number-cell">
                      {String(i + 1).padStart(2, "0")}
                    </td>
                    <td>
                      <Link className="problem-title" to={`/practice/${p.id}`}>
                        {p.title}
                        <ArrowUpRight size={13} />
                      </Link>
                      <span className="file-path">{p.file}</span>
                    </td>
                    <td>
                      <Badge kind={p.difficulty}>{p.difficulty}</Badge>
                    </td>
                    <td>
                      <span className="topic-name">{p.organization.topic}</span>
                      <div className="tags">
                        {p.organization.tags.slice(0, 2).map((t) => (
                          <button key={t} onClick={() => set("tag", t)}>
                            {t}
                          </button>
                        ))}
                      </div>
                    </td>
                    <td>
                      <span className="company-names">
                        {p.organization.companies.join(", ") || "—"}
                      </span>
                    </td>
                    <td>
                      <Status value={p.status} />
                    </td>
                    <td className="date-cell">
                      {formatDate(p.lastAttemptedAt)}
                    </td>
                    <td>
                      <div className="row-actions">
                        <Link
                          aria-label={`Edit ${p.title}`}
                          className="icon-button"
                          to={`/problems/${p.id}/edit`}
                        >
                          <Pencil size={14} />
                        </Link>
                        <button
                          aria-label={`Delete ${p.title}`}
                          className="icon-button danger-hover"
                          onClick={() => remove(p)}
                        >
                          <Trash2 size={14} />
                        </button>
                        <Link
                          aria-label={`Practice ${p.title}`}
                          className="icon-button"
                          to={`/practice/${p.id}`}
                        >
                          <ArrowRight size={15} />
                        </Link>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {items.length === 0 ? (
              <Empty title="No matching problems">
                Try different filters, or create a problem of your own.
              </Empty>
            ) : null}
          </div>
        )}
        <div className="table-footer">
          <span>
            Showing <strong>{items.length}</strong> of {all.length} problems
          </span>
          <span>
            <Folder size={13} /> Filesystem is the source of truth
          </span>
        </div>
      </section>
      <div className="library-tip">
        <FilePlus2 size={20} />
        <div>
          <strong>Bring your own challenges</strong>
          <p>
            Add a JSON file to your problems folder, or import a collection.
            Your library grows with you.
          </p>
        </div>
        <Link to="/settings" className="text-link">
          Import problems <ArrowRight size={14} />
        </Link>
      </div>
    </>
  );
}
function Select({
  label,
  values,
  value,
  onChange,
}: {
  label: string;
  values: string[];
  value: string;
  onChange: (v: string) => void;
}) {
  return (
    <label className="inline-field">
      {label}
      <select
        aria-label={`Filter ${label.toLowerCase()}`}
        value={value}
        onChange={(e) => onChange(e.target.value)}
      >
        <option value="">All {label.toLowerCase()}</option>
        {values.map((v) => (
          <option key={v}>{v}</option>
        ))}
      </select>
    </label>
  );
}
