//go:build integration

package intelligence

import (
	"context"
	"encoding/json"
	"github.com/coder/websocket"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRealPyrightCompletionsHoverSignatureAndDiagnostics(t *testing.T) {
	if os.Getenv("LEXOR_PYRIGHT_TESTS") != "1" {
		t.Skip("set LEXOR_PYRIGHT_TESTS=1 after make setup")
	}
	command, err := filepath.Abs("../../web/node_modules/.bin/pyright-langserver")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(command, true))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, strings.Replace(server.URL, "http:", "ws:", 1), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	send := func(v any) {
		t.Helper()
		b, _ := json.Marshal(v)
		if err := conn.Write(ctx, websocket.MessageText, b); err != nil {
			t.Fatal(err)
		}
	}
	receive := func(id float64) map[string]any {
		t.Helper()
		for {
			_, b, err := conn.Read(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var msg map[string]any
			if err = json.Unmarshal(b, &msg); err != nil {
				t.Fatal(err)
			}
			if got, ok := msg["id"].(float64); ok && got == id {
				if msg["error"] != nil {
					t.Fatal(msg["error"])
				}
				return msg
			}
		}
	}
	send(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
	receive(1)
	send(map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}})
	send(map[string]any{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": map[string]any{"textDocument": map[string]any{"uri": virtualURI, "languageId": "python", "version": 1, "text": "import math\nmath.sqrt(4)\nvalue: int = 'wrong'\n"}}})
	request := func(id int, method string, line, char int) {
		send(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": map[string]any{"textDocument": map[string]any{"uri": virtualURI}, "position": map[string]int{"line": line, "character": char}}})
	}
	request(2, "textDocument/completion", 1, 5)
	b, _ := json.Marshal(receive(2))
	if !strings.Contains(string(b), "sqrt") {
		t.Fatalf("missing semantic stdlib completion: %s", b)
	}
	request(3, "textDocument/hover", 1, 7)
	b, _ = json.Marshal(receive(3))
	if !strings.Contains(string(b), "sqrt") {
		t.Fatalf("hover missing: %s", b)
	}
	request(4, "textDocument/signatureHelp", 1, 10)
	b, _ = json.Marshal(receive(4))
	if !strings.Contains(string(b), "signatures") {
		t.Fatalf("signature missing: %s", b)
	}
	// Re-send a changed document to guarantee a new diagnostics notification after
	// request/response messages above have been consumed.
	send(map[string]any{"jsonrpc": "2.0", "method": "textDocument/didChange", "params": map[string]any{"textDocument": map[string]any{"uri": virtualURI, "version": 2}, "contentChanges": []any{map[string]any{"text": "value: int = 'wrong again'\n"}}}})
	for {
		_, raw, e := conn.Read(ctx)
		if e != nil {
			t.Fatal(e)
		}
		if strings.Contains(string(raw), "publishDiagnostics") && strings.Contains(string(raw), "not assignable") {
			break
		}
	}
}
