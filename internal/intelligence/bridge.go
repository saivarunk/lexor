// Package intelligence bridges a small, explicitly allowed LSP surface to an
// optional trusted local Pyright executable. It never accepts process arguments,
// workspace paths, commands or configuration from the browser.
package intelligence

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const virtualURI = "file:///lexor/solution.py"

type Bridge struct {
	Command string
	Enabled bool
	sem     chan struct{}
}

func New(command string, enabled bool) *Bridge {
	return &Bridge{command, enabled, make(chan struct{}, 1)}
}
func (b *Bridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !b.Enabled {
		http.Error(w, "Language intelligence disabled", http.StatusServiceUnavailable)
		return
	}
	path, err := exec.LookPath(b.Command)
	if err != nil {
		http.Error(w, "Install Pyright: npm install -g pyright", http.StatusServiceUnavailable)
		return
	}
	select {
	case b.sem <- struct{}{}:
		defer func() { <-b.sem }()
	default:
		http.Error(w, "Language server already in use", http.StatusTooManyRequests)
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(256 << 10)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	dir, err := os.MkdirTemp("", "lexor-lsp-")
	if err != nil {
		return
	}
	defer os.RemoveAll(dir)
	uri := "file://" + (&url.URL{Path: dir + "/solution.py"}).EscapedPath()
	rootURI := "file://" + (&url.URL{Path: dir}).EscapedPath()
	cmd := exec.CommandContext(ctx, path, "--stdio")
	cmd.Dir = dir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	cmd.Stderr = io.Discard
	if err = cmd.Start(); err != nil {
		return
	}
	defer cmd.Wait()
	defer cmd.Process.Kill()
	var writeMu sync.Mutex
	send := func(message any) error {
		raw, e := json.Marshal(message)
		if e != nil {
			return e
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		_, e = fmt.Fprintf(stdin, "Content-Length: %d\r\n\r\n%s", len(raw), raw)
		return e
	}
	go func() {
		defer cancel()
		reader := bufio.NewReader(stdout)
		for {
			raw, e := readFrame(reader)
			if e != nil {
				return
			}
			var msg map[string]any
			if json.Unmarshal(raw, &msg) != nil {
				return
			}
			if method, _ := msg["method"].(string); method != "" {
				if method == "workspace/configuration" {
					params, _ := msg["params"].(map[string]any)
					items, _ := params["items"].([]any)
					values := make([]any, len(items))
					for i, item := range items {
						entry, _ := item.(map[string]any)
						switch entry["section"] {
						case "python":
							values[i] = map[string]any{}
						case "python.analysis":
							values[i] = map[string]any{"typeCheckingMode": "basic", "autoSearchPaths": false, "diagnosticMode": "openFilesOnly", "useLibraryCodeForTypes": false}
						default:
							values[i] = nil
						}
					}
					_ = send(map[string]any{"jsonrpc": "2.0", "id": msg["id"], "result": values})
					continue
				}
				if method == "client/registerCapability" || method == "window/workDoneProgress/create" {
					_ = send(map[string]any{"jsonrpc": "2.0", "id": msg["id"], "result": nil})
					continue
				}
				if method == "textDocument/publishDiagnostics" {
					if params, ok := msg["params"].(map[string]any); ok {
						params["uri"] = virtualURI
					}
				}
			}
			raw, _ = json.Marshal(msg)
			writeCtx, c := context.WithTimeout(ctx, 5*time.Second)
			e = conn.Write(writeCtx, websocket.MessageText, raw)
			c()
			if e != nil {
				return
			}
		}
	}()
	for {
		_, raw, e := conn.Read(ctx)
		if e != nil {
			return
		}
		var msg map[string]any
		if json.Unmarshal(raw, &msg) != nil {
			return
		}
		method, _ := msg["method"].(string)
		params, _ := msg["params"].(map[string]any)
		switch method {
		case "initialize":
			msg["params"] = map[string]any{"processId": nil, "rootUri": rootURI, "capabilities": map[string]any{"workspace": map[string]any{"configuration": true}, "textDocument": map[string]any{"completion": map[string]any{"completionItem": map[string]any{"snippetSupport": true}}, "hover": map[string]any{"contentFormat": []string{"markdown", "plaintext"}}, "publishDiagnostics": map[string]any{}}}, "workspaceFolders": []any{map[string]any{"uri": rootURI, "name": "Lexor"}}}
		case "initialized", "shutdown", "exit":
		case "textDocument/didOpen", "textDocument/didChange", "textDocument/didClose", "textDocument/completion", "textDocument/hover", "textDocument/signatureHelp":
			if params == nil {
				return
			}
			doc, ok := params["textDocument"].(map[string]any)
			if !ok {
				return
			}
			doc["uri"] = uri
			if method == "textDocument/didOpen" {
				doc["languageId"] = "python"
			}
		case "completionItem/resolve":
			// Resolution can contain server-issued opaque data; completions work without
			// exposing an unrestricted server method surface. Return the item unchanged.
			_ = conn.Write(ctx, websocket.MessageText, mustJSON(map[string]any{"jsonrpc": "2.0", "id": msg["id"], "result": params}))
			continue
		default:
			return
		}
		if e = send(msg); e != nil {
			return
		}
	}
}
func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }
func readFrame(r *bufio.Reader) ([]byte, error) {
	length := 0
	headerBytes := 0
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		headerBytes += len(line)
		if headerBytes > 4096 {
			return nil, fmt.Errorf("oversized LSP header")
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length:") {
			length, err = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:")))
			if err != nil {
				return nil, err
			}
		}
	}
	if length <= 0 || length > 2<<20 {
		return nil, fmt.Errorf("invalid LSP frame length")
	}
	b := make([]byte, length)
	_, err := io.ReadFull(r, b)
	return b, err
}
