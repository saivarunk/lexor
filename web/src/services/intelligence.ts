import type * as Monaco from "monaco-editor";
export interface LanguageIntelligenceProvider {
  languageId: string;
  initialize(
    monaco: typeof Monaco,
    model: Monaco.editor.ITextModel,
  ): Promise<void>;
  dispose(): Promise<void>;
}
// LSP is a separate standard protocol, intentionally independent of the REST models.
type Pending = {
  resolve: (v: any) => void;
  reject: (e: Error) => void;
  timer: ReturnType<typeof setTimeout>;
};
export class PyrightProvider implements LanguageIntelligenceProvider {
  languageId = "python";
  private socket?: WebSocket;
  private next = 1;
  private pending = new Map<number, Pending>();
  private disposables: Monaco.IDisposable[] = [];
  private uri = "file:///lexor/solution.py";
  private timer?: ReturnType<typeof setTimeout>;
  private version = 1;
  private send(method: string, params?: unknown) {
    if (this.socket?.readyState === WebSocket.OPEN)
      this.socket.send(JSON.stringify({ jsonrpc: "2.0", method, params }));
  }
  private request(method: string, params: unknown): Promise<any> {
    return new Promise((resolve, reject) => {
      const id = this.next++;
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error("Language server request timed out"));
      }, 12_000);
      this.pending.set(id, { resolve, reject, timer });
      this.socket?.send(JSON.stringify({ jsonrpc: "2.0", id, method, params }));
    });
  }
  async initialize(monaco: typeof Monaco, model: Monaco.editor.ITextModel) {
    const ws = new WebSocket(
      `${location.protocol === "https:" ? "wss:" : "ws:"}//${location.host}/api/v1/intelligence`,
    );
    this.socket = ws;
    ws.onmessage = (e) => {
      const msg = JSON.parse(e.data);
      if (typeof msg.id === "number" && this.pending.has(msg.id)) {
        const p = this.pending.get(msg.id)!;
        clearTimeout(p.timer);
        this.pending.delete(msg.id);
        if (msg.error) p.reject(new Error(msg.error.message));
        else p.resolve(msg.result);
      }
      if (
        msg.method === "textDocument/publishDiagnostics" &&
        msg.params?.uri === this.uri
      ) {
        monaco.editor.setModelMarkers(
          model,
          "pyright",
          (msg.params.diagnostics ?? []).map((d: any) => ({
            startLineNumber: d.range.start.line + 1,
            startColumn: d.range.start.character + 1,
            endLineNumber: d.range.end.line + 1,
            endColumn: d.range.end.character + 1,
            message: d.message,
            severity:
              d.severity === 1
                ? monaco.MarkerSeverity.Error
                : monaco.MarkerSeverity.Warning,
            source: "Pyright",
          })),
        );
      }
    };
    await new Promise<void>((resolve, reject) => {
      ws.onopen = () => resolve();
      ws.onerror = () => reject(new Error("Pyright unavailable"));
      ws.onclose = () => {
        for (const p of this.pending.values()) {
          clearTimeout(p.timer);
          p.reject(new Error("Language server disconnected"));
        }
        this.pending.clear();
      };
    });
    await this.request("initialize", {});
    this.send("initialized", {});
    this.send("textDocument/didOpen", {
      textDocument: {
        uri: this.uri,
        languageId: "python",
        version: this.version,
        text: model.getValue(),
      },
    });
    const flush = () => {
      clearTimeout(this.timer);
      this.send("textDocument/didChange", {
        textDocument: { uri: this.uri, version: ++this.version },
        contentChanges: [{ text: model.getValue() }],
      });
    };
    this.disposables.push(
      model.onDidChangeContent(() => {
        clearTimeout(this.timer);
        this.timer = setTimeout(flush, 300);
      }),
    );
    const params = (p: Monaco.Position) => ({
      textDocument: { uri: this.uri },
      position: { line: p.lineNumber - 1, character: p.column - 1 },
    });
    this.disposables.push(
      monaco.languages.registerCompletionItemProvider("python", {
        triggerCharacters: ["."],
        provideCompletionItems: async (m, pos) => {
          if (m !== model) return { suggestions: [] };
          flush();
          try {
            const response = await this.request(
              "textDocument/completion",
              params(pos),
            );
            const items = Array.isArray(response)
              ? response
              : (response?.items ?? []);
            const word = m.getWordUntilPosition(pos);
            return {
              suggestions: items.map((i: any) => ({
                label: i.label,
                kind: completionKind(monaco, i.kind),
                insertText: i.textEdit?.newText ?? i.insertText ?? i.label,
                insertTextRules:
                  i.insertTextFormat === 2
                    ? monaco.languages.CompletionItemInsertTextRule
                        .InsertAsSnippet
                    : undefined,
                detail: i.detail,
                documentation: i.documentation,
                sortText: i.sortText,
                filterText: i.filterText,
                range: i.textEdit?.range
                  ? range(i.textEdit.range)
                  : {
                      startLineNumber: pos.lineNumber,
                      endLineNumber: pos.lineNumber,
                      startColumn: word.startColumn,
                      endColumn: word.endColumn,
                    },
              })),
            };
          } catch {
            return { suggestions: [] };
          }
        },
      }),
    );
    this.disposables.push(
      monaco.languages.registerHoverProvider("python", {
        provideHover: async (m, pos) => {
          if (m !== model) return null;
          try {
            const h = await this.request("textDocument/hover", params(pos));
            if (!h) return null;
            const content = Array.isArray(h.contents)
              ? h.contents
              : [h.contents];
            return {
              contents: content.map((v: any) => ({
                value: typeof v === "string" ? v : v.value,
              })),
              range: h.range ? range(h.range) : undefined,
            };
          } catch {
            return null;
          }
        },
      }),
    );
    this.disposables.push(
      monaco.languages.registerSignatureHelpProvider("python", {
        signatureHelpTriggerCharacters: ["(", ","],
        provideSignatureHelp: async (m, pos) => {
          if (m !== model) return null;
          flush();
          try {
            const h = await this.request(
              "textDocument/signatureHelp",
              params(pos),
            );
            if (!h) return null;
            return {
              value: {
                signatures: h.signatures,
                activeSignature: h.activeSignature ?? 0,
                activeParameter: h.activeParameter ?? 0,
              },
              dispose() {},
            };
          } catch {
            return null;
          }
        },
      }),
    );
  }
  async dispose() {
    clearTimeout(this.timer);
    this.disposables.forEach((d) => d.dispose());
    this.send("textDocument/didClose", { textDocument: { uri: this.uri } });
    this.socket?.close();
    for (const p of this.pending.values()) {
      clearTimeout(p.timer);
      p.reject(new Error("Editor closed"));
    }
    this.pending.clear();
  }
}
function range(r: any) {
  return {
    startLineNumber: r.start.line + 1,
    startColumn: r.start.character + 1,
    endLineNumber: r.end.line + 1,
    endColumn: r.end.character + 1,
  };
}
function completionKind(m: typeof Monaco, k: number) {
  const kinds: Record<number, Monaco.languages.CompletionItemKind> = {
    2: m.languages.CompletionItemKind.Method,
    3: m.languages.CompletionItemKind.Function,
    6: m.languages.CompletionItemKind.Variable,
    7: m.languages.CompletionItemKind.Class,
    9: m.languages.CompletionItemKind.Module,
    10: m.languages.CompletionItemKind.Property,
    14: m.languages.CompletionItemKind.Keyword,
  };
  return kinds[k] ?? m.languages.CompletionItemKind.Text;
}
