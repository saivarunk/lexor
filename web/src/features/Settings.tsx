import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Check,
  ChevronRight,
  Download,
  FolderGit2,
  Globe,
  HardDrive,
  Palette,
  RefreshCw,
  ShieldCheck,
  Terminal,
} from "lucide-react";
import {
  api,
  post,
  type ImportPackage,
  type SyncPreview,
  type SystemStatus,
} from "../services/api";
import { useAppearance } from "../hooks/useAppearance";
import { Badge, ErrorBox, Loading, PageTitle } from "../components/ui";
export default function Settings() {
  const q = useQuery({
    queryKey: ["system"],
    queryFn: () => api<SystemStatus>("/system/status"),
    staleTime: 60_000,
  });
  const sync = useQuery({
    queryKey: ["sync"],
    queryFn: () => api<SyncPreview>("/sync/status"),
  });
  const [source, setSource] = useState("");
  const [git, setGit] = useState(false);
  const [pack, setPack] = useState<ImportPackage>();
  const [notice, setNotice] = useState("");
  const { preferences, appearance, theme } = useAppearance();
  const client = useQueryClient();
  const importAction = useMutation({
    mutationFn: async (digest?: string) =>
      post<any>("/registry/import", {
        source,
        git,
        confirmDigest: digest ?? "",
      }),
    onSuccess: (v) => {
      if (v.installed) {
        setPack(undefined);
        setNotice(`Imported ${v.installed.length} problems.`);
        void client.invalidateQueries({ queryKey: ["problems"] });
        void client.invalidateQueries({ queryKey: ["stats"] });
        void client.invalidateQueries({ queryKey: ["system"] });
      } else {
        setPack(v);
        setNotice("");
      }
    },
  });
  const exportAction = useMutation({
    mutationFn: () => post<SyncPreview>("/sync/export"),
    onSuccess: (r) =>
      setNotice(`Exported ${r.files.length} files to ${r.directory}`),
  });
  return (
    <>
      <PageTitle
        eyebrow="MAKE YOURSELF AT HOME"
        title="Your workspace, your way."
        description="Local by design. Configure your tools and bring your own challenges."
        action={
          <button
            className="button secondary"
            onClick={() => {
              void q.refetch();
              void sync.refetch();
            }}
          >
            <RefreshCw size={14} /> Check environment
          </button>
        }
      />
      <ErrorBox
        error={
          q.error ??
          sync.error ??
          importAction.error ??
          exportAction.error ??
          appearance.error
        }
      />
      {notice ? (
        <div className="success-box">
          <Check size={16} />
          {notice}
        </div>
      ) : null}
      <div className="settings-grid">
        <section className="card settings-card">
          <div className="settings-section-title">
            <HardDrive size={19} />
            <div>
              <h3>Workspace & environment</h3>
              <p>Ready to work without a cloud connection.</p>
            </div>
            <Badge kind="easy">LOCAL</Badge>
          </div>
          {q.isPending ? (
            <Loading />
          ) : (
            <>
              <div className="settings-path">
                <span>WORKSPACE DIRECTORY</span>
                <code>{q.data?.workspace}</code>
              </div>
              <div className="settings-path">
                <span>ACTIVITY TIMEZONE</span>
                <code>{q.data?.timezone}</code>
              </div>
              {q.data?.diagnostics.map((d) => (
                <div className="diagnostic" key={d.name}>
                  <span
                    className={`diagnostic-icon ${d.ok ? "good" : "attention"}`}
                  >
                    {d.ok ? <Check size={14} /> : <Terminal size={14} />}
                  </span>
                  <div>
                    <strong>{d.name}</strong>
                    <span>{d.message}</span>
                  </div>
                  <Badge kind={d.ok ? "easy" : "warning"}>
                    {d.ok ? "Ready" : "Setup needed"}
                  </Badge>
                </div>
              ))}
            </>
          )}
          {q.data?.issues.length ? (
            <ErrorBox error={new Error(q.data.issues.join("\n"))} />
          ) : null}
          <div className="settings-footnote">
            <Terminal size={14} /> Configuration: <code>lexor.yaml</code> ·
            Flags → environment → file → defaults.
          </div>
        </section>
        <div className="settings-side">
          <section className="card settings-card">
            <div className="settings-section-title">
              <Palette size={19} />
              <div>
                <h3>Appearance</h3>
                <p>Settle into your preferred environment.</p>
              </div>
            </div>
            <label className="field">
              <span>Workbench theme</span>
              <select
                aria-label="Workbench theme"
                value={theme}
                disabled={preferences.isPending || appearance.isPending}
                onChange={(e) =>
                  appearance.mutate(
                    e.target.value === "light" ? "light" : "dark",
                  )
                }
              >
                <option value="dark">Dark · Default</option>
                <option value="light">Light</option>
              </select>
            </label>
            <p className="small muted">
              Applies to the workbench and code editor. You can customize the
              editor theme and font size inside the practice workspace.
            </p>
          </section>
          <section className="card settings-card security-note">
            <ShieldCheck size={24} />
            <h3>A real execution boundary.</h3>
            <p>
              Your solutions execute in disposable containers with network
              access disabled, a read-only root, and strict resource limits.
            </p>
            <code>{q.data?.image ?? "python:3.12.15-alpine3.23"}</code>
            <p className="small">
              The library and editor remain available when the runtime is
              offline. Lexor never runs your solutions on the host.
            </p>
          </section>
        </div>
        <section className="card settings-card">
          <div className="settings-section-title">
            <Globe size={19} />
            <div>
              <h3>Import a problem collection</h3>
              <p>Local JSON, directories, HTTPS JSON, or a Git repository.</p>
            </div>
          </div>
          <label className="field">
            <span>Source</span>
            <input
              aria-label="Import source"
              placeholder="/path/to/collection or https://…/problems.json"
              value={source}
              onChange={(e) => {
                setSource(e.target.value);
                setPack(undefined);
              }}
            />
          </label>
          <label className="checkbox-field">
            <input
              type="checkbox"
              checked={git}
              onChange={(e) => {
                setGit(e.target.checked);
                setPack(undefined);
              }}
            />{" "}
            Source is a Git repository
          </label>
          <button
            className="button secondary"
            disabled={!source || importAction.isPending}
            onClick={() => importAction.mutate(undefined)}
          >
            <Download size={14} /> Preview import <ChevronRight size={14} />
          </button>
          {pack ? (
            <div className="import-preview">
              <strong>{pack.problems.length} validated problems</strong>
              {pack.problems.map((p) => (
                <div key={p.id}>
                  {p.title}
                  <code>{p.id}</code>
                </div>
              ))}
              <p className="small muted">
                ID collisions are rejected. Existing files will never be
                overwritten.
              </p>
              <button
                className="button primary"
                disabled={importAction.isPending}
                onClick={() => importAction.mutate(pack.digest)}
              >
                Confirm & import
              </button>
            </div>
          ) : null}
        </section>
        <section className="card settings-card">
          <div className="settings-section-title">
            <FolderGit2 size={19} />
            <div>
              <h3>Portable progress</h3>
              <p>Export your progress to a private Git working tree.</p>
            </div>
          </div>
          <div className="settings-path">
            <span>EXPORT DIRECTORY</span>
            <code>{sync.data?.directory}</code>
          </div>
          <div className="sync-files">
            {sync.data?.files.map((f) => (
              <div key={f}>
                <Check size={12} />
                <code>{f}</code>
              </div>
            ))}
          </div>
          <button
            className="button secondary"
            disabled={exportAction.isPending}
            onClick={() => exportAction.mutate()}
          >
            <Download size={14} /> Export {sync.data?.files.length ?? 0} files
          </button>
          <p className="small muted">
            Exports accepted solutions, progress, and activity. Drafts,
            database, credentials, and workspace settings stay private. To
            initialize Git, commit, or push, use <code>lexor sync init</code>{" "}
            and <code>lexor sync --commit --push</code>.
          </p>
        </section>
      </div>
    </>
  );
}
