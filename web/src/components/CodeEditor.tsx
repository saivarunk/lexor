import { useEffect, useRef } from "react";
import Editor, { loader, type OnMount } from "@monaco-editor/react";
import * as monaco from "monaco-editor/esm/vs/editor/editor.api";
import "monaco-editor/esm/vs/basic-languages/python/python.contribution";
import "monaco-editor/esm/vs/editor/contrib/suggest/browser/suggestController";
import "monaco-editor/esm/vs/editor/contrib/hover/browser/hoverContribution";
import "monaco-editor/esm/vs/editor/contrib/parameterHints/browser/parameterHints";
import "monaco-editor/esm/vs/editor/contrib/bracketMatching/browser/bracketMatching";
import "monaco-editor/esm/vs/editor/contrib/find/browser/findController";
import "monaco-editor/esm/vs/editor/contrib/comment/browser/comment";
import EditorWorker from "monaco-editor/esm/vs/editor/editor.worker?worker";
import { PyrightProvider } from "../services/intelligence";
self.MonacoEnvironment = { getWorker: () => new EditorWorker() };
loader.config({ monaco });
export default function CodeEditor({
  value,
  onChange,
  onRun,
  onSubmit,
  onIntelligence,
  theme,
  fontSize,
}: {
  value: string;
  onChange: (v: string) => void;
  onRun: () => void;
  onSubmit: () => void;
  onIntelligence: (v: string) => void;
  theme: string;
  fontSize: number;
}) {
  const provider = useRef<PyrightProvider | null>(null);
  const actions = useRef({ onRun, onSubmit });
  actions.current = { onRun, onSubmit };
  useEffect(
    () => () => {
      void provider.current?.dispose();
    },
    [],
  );
  const mounted: OnMount = (editor, m) => {
    editor.addAction({
      id: "lexor.run",
      label: "Lexor: run sample tests",
      keybindings: [m.KeyMod.CtrlCmd | m.KeyCode.Enter],
      run: () => actions.current.onRun(),
    });
    editor.addAction({
      id: "lexor.submit",
      label: "Lexor: submit solution",
      keybindings: [m.KeyMod.CtrlCmd | m.KeyMod.Shift | m.KeyCode.Enter],
      run: () => actions.current.onSubmit(),
    });
    provider.current = new PyrightProvider();
    const model = editor.getModel();
    if (model)
      provider.current
        .initialize(m, model)
        .then(() => onIntelligence("Pyright connected"))
        .catch(() => onIntelligence("Pyright unavailable · editing ready"));
  };
  return (
    <Editor
      height="100%"
      language="python"
      theme={theme === "light" ? "vs" : "vs-dark"}
      value={value}
      onChange={(v) => onChange(v ?? "")}
      onMount={mounted}
      loading={<div className="loading">Preparing your editor…</div>}
      options={{
        fontSize,
        fontFamily: '"SFMono-Regular", Consolas, "Liberation Mono", monospace',
        lineHeight: 24,
        minimap: { enabled: false },
        padding: { top: 22, bottom: 16 },
        scrollBeyondLastLine: false,
        automaticLayout: true,
        tabSize: 4,
        wordWrap: "on",
        renderLineHighlight: "line",
        overviewRulerLanes: 0,
        glyphMargin: false,
        lineNumbersMinChars: 4,
        wordBasedSuggestions: "off",
        fixedOverflowWidgets: true,
      }}
    />
  );
}
