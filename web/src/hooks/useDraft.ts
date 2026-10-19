import { useCallback, useEffect, useRef, useState } from "react";
import { api, type Draft, type ProblemView } from "../services/api";
export function useDraft(problem: ProblemView, draft: Draft) {
  const key = `lexor:draft:v1:${problem.id}:python`;
  const starter = problem.languages.python?.starterCode ?? "";
  const [source, setSource] = useState(() => {
    try {
      const b = JSON.parse(localStorage.getItem(key) ?? "null");
      if (
        b &&
        typeof b.source === "string" &&
        (!draft.updatedAt ||
          Date.parse(b.updatedAt) > Date.parse(draft.updatedAt))
      )
        return b.source as string;
    } catch {
      /* Invalid browser backup: use SQLite. */
    }
    return draft.updatedAt ? draft.source : starter;
  });
  const [saved, setSaved] = useState("All changes saved");
  const [saveError, setSaveError] = useState<unknown>();
  const latest = useRef(source);
  latest.current = source;
  const persisted = useRef(draft.source);
  const backup = useRef<string | undefined>(undefined);
  const mounted = useRef(true);
  const queue = useRef<Promise<void>>(Promise.resolve());
  const save = useCallback(
    (text: string) => {
      const operation = queue.current
        .catch(() => {})
        .then(async () => {
          await api(`/drafts/${problem.id}?language=python`, {
            method: "PUT",
            body: JSON.stringify({
              problemId: problem.id,
              language: "python",
              source: text,
            }),
          });
          persisted.current = text;
          if (mounted.current && latest.current === text) {
            setSaved("All changes saved");
            setSaveError(undefined);
          }
        });
      queue.current = operation;
      return operation.catch((e) => {
        if (mounted.current) {
          setSaved(
            backup.current === latest.current
              ? "Saved in this browser"
              : "Draft not saved",
          );
          setSaveError(e);
        }
      });
    },
    [problem.id],
  );
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  useEffect(() => {
    if (source === persisted.current) return;
    setSaved("Saving draft…");
    try {
      localStorage.setItem(
        key,
        JSON.stringify({ source, updatedAt: new Date().toISOString() }),
      );
      backup.current = source;
    } catch {
      /* Keep unload protection if neither persistence path works. */
    }
    const timer = setTimeout(() => void save(source), 500);
    return () => clearTimeout(timer);
  }, [source, key, save]);
  useEffect(() => {
    const before = (e: BeforeUnloadEvent) => {
      if (latest.current !== persisted.current) {
        e.preventDefault();
        e.returnValue = "";
      }
    };
    const flush = () => {
      if (latest.current !== persisted.current) void save(latest.current);
    };
    window.addEventListener("beforeunload", before);
    document.addEventListener("visibilitychange", flush);
    return () => {
      window.removeEventListener("beforeunload", before);
      document.removeEventListener("visibilitychange", flush);
      flush();
    };
  }, [save]);
  return { source, setSource, latest, save, saved, saveError, starter };
}
