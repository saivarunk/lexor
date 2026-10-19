import { lazy, Suspense, useEffect, useState } from "react";
import {
  NavLink,
  Navigate,
  Route,
  Routes,
  useLocation,
} from "react-router-dom";
import {
  ArrowUpRight,
  BookOpen,
  Braces,
  ChevronDown,
  ChevronRight,
  CircleHelp,
  Code2,
  Flame,
  LayoutDashboard,
  ListChecks,
  Moon,
  PanelLeftClose,
  Settings2,
  Sun,
  Terminal,
  TrendingUp,
  WifiOff,
} from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { api, type Stats } from "../services/api";
import { ErrorBox, Loading } from "../components/ui";
import { useAppearance } from "../hooks/useAppearance";
const Dashboard = lazy(() => import("../features/Dashboard"));
const Problems = lazy(() => import("../features/Problems"));
const ProblemEditor = lazy(() => import("../features/ProblemEditor"));
const Practice = lazy(() => import("../features/Practice"));
const Submissions = lazy(() => import("../features/Submissions"));
const Settings = lazy(() => import("../features/Settings"));
const nav = [
  { to: "/", name: "Overview", icon: LayoutDashboard, end: true },
  { to: "/problems", name: "Problem library", icon: BookOpen },
  { to: "/practice", name: "Practice", icon: Code2 },
  { to: "/submissions", name: "Submissions", icon: ListChecks },
  { to: "/progress", name: "Progress", icon: TrendingUp },
];
export default function App() {
  const location = useLocation();
  const [collapsed, setCollapsed] = useState(false);
  const { data: stats } = useQuery({
    queryKey: ["stats"],
    queryFn: () => api<Stats>("/stats"),
  });
  const { preferences, appearance, theme } = useAppearance();
  useEffect(() => {
    if (preferences.data) {
      document.documentElement.dataset.theme = preferences.data.theme;
      localStorage.setItem("lexor:theme", preferences.data.theme);
      document
        .querySelector('meta[name="theme-color"]')
        ?.setAttribute(
          "content",
          preferences.data.theme === "light" ? "#f7f8fa" : "#111214",
        );
    }
  }, [preferences.data]);
  const active = location.pathname.startsWith("/practice/")
    ? "Practice"
    : location.pathname.startsWith("/settings")
      ? "Settings"
      : (nav.find((n) => n.to !== "/" && location.pathname.startsWith(n.to))
          ?.name ?? "Overview");
  return (
    <div className={`app ${collapsed ? "sidebar-collapsed" : ""}`}>
      <aside className="sidebar">
        <NavLink to="/" className="brand" aria-label="Lexor home">
          <div className="brand-mark">
            <Braces size={24} />
          </div>
          <span>
            lexor<span className="brand-dot">.</span>
          </span>
          <span className="version">BETA</span>
        </NavLink>
        <div className="workspace-selector">
          <div className="workspace-avatar">L</div>
          <div>
            <strong>Local workspace</strong>
            <span>Personal · Offline ready</span>
          </div>
          <ChevronDown size={14} />
        </div>
        <div className="nav-label">WORKSPACE</div>
        <nav>
          {nav.map((n) => (
            <NavLink
              to={n.to}
              end={n.end}
              key={n.to}
              className={({ isActive }) =>
                `nav-item ${isActive ? "active" : ""}`
              }
            >
              <n.icon size={18} />
              <span>{n.name}</span>
              {n.name === "Problem library" ? (
                <span className="nav-count">{stats?.total ?? "—"}</span>
              ) : null}
            </NavLink>
          ))}
        </nav>
        <div className="sidebar-rule" />
        <div className="nav-label">YOUR TOOLBOX</div>
        <NavLink className="nav-item" to="/settings">
          <Settings2 size={18} />
          <span>Settings & integrations</span>
        </NavLink>
        <a
          className="nav-item"
          href="/api/v1/openapi.json"
          target="_blank"
          rel="noreferrer"
        >
          <Terminal size={18} />
          <span>API reference</span>
          <ArrowUpRight size={13} />
        </a>
        <div className="sidebar-spacer" />
        <div className="daily-card">
          <div>
            <Flame size={17} />
            <strong>Build your momentum</strong>
          </div>
          <p>
            One problem. A little progress.
            <br />
            Your next streak starts here.
          </p>
          <NavLink to="/problems">
            Find your next challenge <ChevronRight size={14} />
          </NavLink>
        </div>
        <div className="local-status">
          <span className="online-dot" />
          <span>Everything stays on your machine</span>
          <WifiOff size={13} />
        </div>
        <div className="sidebar-footer">
          <div className="user-avatar">Y</div>
          <div>
            <strong>Your workspace</strong>
            <span>Local-first. Always yours.</span>
          </div>
          <CircleHelp size={16} />
        </div>
      </aside>
      <div className="main-shell">
        <header className="topbar">
          <div className="breadcrumbs">
            <button
              className="icon-button"
              aria-label="Toggle sidebar"
              onClick={() => setCollapsed((v) => !v)}
            >
              <PanelLeftClose size={17} />
            </button>
            <span>Workspace</span>
            <ChevronRight size={13} />
            <strong>{active}</strong>
          </div>
          <div className="topbar-right">
            <span className="local-chip">
              <span className="online-dot" />
              Local environment
            </span>
            <span className="topbar-divider" />
            <Flame size={16} className="flame" />
            <span className="streak-count">
              {stats?.currentStreak ?? 0} day streak
            </span>
            <button
              className="icon-button theme-toggle"
              aria-label={
                theme === "dark"
                  ? "Switch to light theme"
                  : "Switch to dark theme"
              }
              title={
                theme === "dark"
                  ? "Switch to light theme"
                  : "Switch to dark theme"
              }
              disabled={preferences.isPending || appearance.isPending}
              onClick={() =>
                appearance.mutate(theme === "dark" ? "light" : "dark")
              }
            >
              {theme === "dark" ? <Sun size={18} /> : <Moon size={18} />}
            </button>
            <NavLink
              to="/settings"
              title="Workspace settings"
              className="icon-button"
            >
              <Settings2 size={17} />
            </NavLink>
          </div>
        </header>
        {appearance.error ? <ErrorBox error={appearance.error} /> : null}
        <main
          className={
            location.pathname.startsWith("/practice/")
              ? "practice-main"
              : "page"
          }
        >
          <Suspense fallback={<Loading />}>
            <Routes>
              <Route path="/" element={<Dashboard />} />
              <Route path="/problems" element={<Problems />} />
              <Route path="/problems/new" element={<ProblemEditor />} />
              <Route path="/problems/:id/edit" element={<ProblemEditor />} />
              <Route
                path="/practice"
                element={<Navigate to="/problems" replace />}
              />
              <Route
                path="/practice/:id"
                element={<Practice key={location.pathname} />}
              />
              <Route path="/submissions" element={<Submissions />} />
              <Route path="/progress" element={<Dashboard progressOnly />} />
              <Route path="/settings" element={<Settings />} />
              <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
          </Suspense>
        </main>
        <footer className="page-footer">
          <span>
            <span className="online-dot" /> All systems local
          </span>
          <span>
            Craft your craft. <span className="muted">Lexor v0.1.0</span>
          </span>
        </footer>
      </div>
    </div>
  );
}
