import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import {
  ArrowDownRight,
  ArrowRight,
  ArrowUpRight,
  BookOpen,
  Check,
  ChevronRight,
  Clock3,
  Code2,
  Flame,
  Sparkles,
  Target,
  Trophy,
} from "lucide-react";
import { api, formatDate, type ProblemView, type Stats } from "../services/api";
import {
  Badge,
  Empty,
  ErrorBox,
  Loading,
  PageTitle,
  Status,
} from "../components/ui";
export default function Dashboard({
  progressOnly = false,
}: {
  progressOnly?: boolean;
}) {
  const stats = useQuery({
    queryKey: ["stats"],
    queryFn: () => api<Stats>("/stats"),
  });
  const problems = useQuery({
    queryKey: ["problems", {}],
    queryFn: () => api<ProblemView[]>("/problems"),
  });
  if (stats.isPending || problems.isPending) return <Loading />;
  if (stats.error || problems.error)
    return <ErrorBox error={stats.error ?? problems.error} />;
  const s = stats.data!;
  const all = problems.data!;
  const next =
    all.find((p) => p.status === "attempted") ??
    all.find((p) => p.slug === "two-sum") ??
    all[0];
  const solvedPercent = s.total ? Math.round((s.solved / s.total) * 100) : 0;
  return (
    <>
      <PageTitle
        eyebrow={
          progressOnly ? "YOUR JOURNEY" : "YOUR PERSONAL CODING WORKBENCH"
        }
        title={
          progressOnly
            ? "Every practice counts."
            : "A little better, every day."
        }
        description={
          progressOnly
            ? "A clear view of the progress you’re making."
            : "A quiet place to think, solve, and grow. Pick up where you left off."
        }
        action={
          <Link className="button secondary" to="/problems">
            <BookOpen size={15} /> Browse library <ArrowUpRight size={14} />
          </Link>
        }
      />
      {!progressOnly ? (
        <div className="dashboard-hero">
          <div className="hero-copy">
            <Badge kind="purple">
              <Sparkles size={12} /> YOUR NEXT CHALLENGE
            </Badge>
            <h2>
              Small steps.
              <br />
              <span>Stronger problem solving.</span>
            </h2>
            <p>
              Build intuition one problem at a time.
              <br />
              Your workspace is ready whenever you are.
            </p>
            {next ? (
              <Link className="button primary" to={`/practice/${next.id}`}>
                Let’s practice <ArrowRight size={16} />
              </Link>
            ) : (
              <Link className="button primary" to="/problems/new">
                Create your first problem
              </Link>
            )}
          </div>
          <div className="hero-art" aria-hidden="true">
            <div className="orbit orbit-one" />
            <div className="orbit orbit-two" />
            <div className="code-tile tile-back">
              <Code2 size={38} />
            </div>
            <div className="code-tile tile-front">
              <span className="tile-comment">
                # progress, one line at a time
              </span>
              <span>
                <b>def</b> improve(today):
              </span>
              <span>
                {" "}
                <b>return</b> today + <i>1</i>
              </span>
              <div className="tile-success">
                <Check size={13} /> Keep showing up
              </div>
            </div>
            <div className="floating-spark">✧</div>
          </div>
        </div>
      ) : null}
      <div className="metric-grid">
        <Metric
          icon={<Target size={18} />}
          label="Problems solved"
          value={s.solved}
          suffix={`/ ${s.total}`}
          detail={`${solvedPercent}% of your library`}
          tone="violet"
        />
        <Metric
          icon={<Flame size={18} />}
          label="Current streak"
          value={s.currentStreak}
          suffix="days"
          detail={`Personal best: ${s.longestStreak} days`}
          tone="orange"
        />
        <Metric
          icon={<Check size={18} />}
          label="Solved this week"
          value={s.solvedThisWeek}
          detail="New challenges completed"
          tone="green"
        />
        <Metric
          icon={<Clock3 size={18} />}
          label="Problems attempted"
          value={s.attempted}
          detail="Every attempt is a step forward"
          tone="blue"
        />
      </div>
      <div className="dashboard-middle">
        <section className="card activity-card">
          <div className="section-heading">
            <div>
              <h3>Practice activity</h3>
              <p>
                {s.activity.reduce((a, b) => a + b.count, 0)} accepted
                submissions in your workspace
              </p>
            </div>
            <span className="small muted">
              Last 26 weeks <ChevronRight size={13} />
            </span>
          </div>
          <Heatmap activity={s.activity} />
        </section>
        <section className="card difficulty-card">
          <div className="section-heading">
            <div>
              <h3>By difficulty</h3>
              <p>Expand your comfort zone</p>
            </div>
            <Trophy size={17} className="muted" />
          </div>
          {["easy", "medium", "hard"].map((d) => {
            const total = all.filter((p) => p.difficulty === d).length;
            return (
              <div className="difficulty-row" key={d}>
                <div>
                  <span className={`difficulty-dot ${d}`} />
                  <span className="capitalize">{d}</span>
                  <span className="muted">
                    {s.difficulty[d] ?? 0}
                    <span> / {total}</span>
                  </span>
                </div>
                <div className="progress-track">
                  <span
                    className={d}
                    style={{
                      width: `${total ? ((s.difficulty[d] ?? 0) / total) * 100 : 0}%`,
                    }}
                  />
                </div>
              </div>
            );
          })}
        </section>
      </div>
      <div className="dashboard-bottom">
        <section className="card">
          <div className="section-heading">
            <div>
              <h3>
                {s.recent.length
                  ? "Recent submissions"
                  : "A good place to start"}
              </h3>
              <p>
                {s.recent.length
                  ? "Your latest problem-solving sessions"
                  : "A few foundations worth getting comfortable with"}
              </p>
            </div>
            <Link
              className="text-link"
              to={s.recent.length ? "/submissions" : "/problems"}
            >
              View all <ArrowUpRight size={13} />
            </Link>
          </div>
          {s.recent.length
            ? s.recent.slice(0, 4).map((r) => (
                <Link
                  className="recent-row"
                  to={`/practice/${r.problemId}`}
                  key={r.id}
                >
                  <div className="row-icon">
                    <Code2 size={17} />
                  </div>
                  <div>
                    <strong>
                      {all.find((p) => p.id === r.problemId)?.title ??
                        r.problemId}
                    </strong>
                    <span>{formatDate(r.createdAt)} · Python</span>
                  </div>
                  <Status value={r.status} />
                  <ChevronRight size={14} />
                </Link>
              ))
            : all.slice(0, 3).map((p, i) => (
                <Link
                  className="recent-row"
                  to={`/practice/${p.id}`}
                  key={p.id}
                >
                  <div className="row-number">0{i + 1}</div>
                  <div>
                    <strong>{p.title}</strong>
                    <span>
                      {p.organization.topic} ·{" "}
                      {p.organization.tags.slice(0, 2).join(", ")}
                    </span>
                  </div>
                  <Badge kind={p.difficulty}>{p.difficulty}</Badge>
                  <ArrowUpRight size={14} />
                </Link>
              ))}
        </section>
        <section className="card practice-note">
          <div className="note-icon">
            <BookOpen size={21} />
          </div>
          <span className="eyebrow">MADE FOR YOUR FLOW</span>
          <h3>
            Less setup.
            <br />
            More solving.
          </h3>
          <p>
            Your problems, solutions, and progress live right here. No accounts.
            No distractions.
          </p>
          <Link to="/settings" className="text-link">
            Make this workspace yours <ArrowRight size={14} />
          </Link>
          <ArrowDownRight className="note-arrow" size={74} />
        </section>
      </div>
      {s.total === 0 ? (
        <Empty title="Your library is waiting">
          Create a problem or import a collection in Settings.
        </Empty>
      ) : null}
    </>
  );
}
function Metric({
  icon,
  label,
  value,
  suffix,
  detail,
  tone,
}: {
  icon: React.ReactNode;
  label: string;
  value: number;
  suffix?: string;
  detail: string;
  tone: string;
}) {
  return (
    <section className="card metric">
      <div className="metric-top">
        <span>{label}</span>
        <span className={`metric-icon ${tone}`}>{icon}</span>
      </div>
      <div className="metric-value">
        {value}
        <span>{suffix}</span>
      </div>
      <p>{detail}</p>
    </section>
  );
}
export function Heatmap({ activity }: { activity: Stats["activity"] }) {
  const byDate = new Map(activity.map((a) => [a.date, a.count]));
  const today = new Date();
  today.setHours(12, 0, 0, 0);
  const start = new Date(today);
  start.setDate(start.getDate() - 175 - start.getDay());
  const days = Array.from({ length: 182 }, (_, i) => {
    const d = new Date(start);
    d.setDate(d.getDate() + i);
    const iso = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
    return {
      date: iso,
      count: byDate.get(iso) ?? 0,
      month: d.toLocaleString(undefined, { month: "short" }),
      day: d.getDate(),
    };
  });
  return (
    <div className="heatmap-wrap">
      <div className="heatmap-months">
        {Array.from({ length: 6 }, (_, i) => (
          <span key={i}>{days[Math.min(i * 30, 181)].month}</span>
        ))}
      </div>
      <div className="heatmap-body">
        <div className="heatmap-weekdays">
          <span>Mon</span>
          <span>Wed</span>
          <span>Fri</span>
        </div>
        <div className="heatmap-grid">
          {days.map((d) => (
            <div
              key={d.date}
              title={`${d.date}: ${d.count} accepted submissions`}
              className={`heatmap-cell level-${Math.min(d.count, 4)}`}
            />
          ))}
        </div>
      </div>
      <div className="heatmap-legend">
        <span>
          <span className="online-dot" /> Consistency over intensity.
        </span>
        <span>
          Less <i className="heatmap-cell" />
          {[1, 2, 3, 4].map((n) => (
            <i className={`heatmap-cell level-${n}`} key={n} />
          ))}{" "}
          More
        </span>
      </div>
    </div>
  );
}
