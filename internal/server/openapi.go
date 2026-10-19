package server

import (
	"encoding/json"
	"github.com/varun/lexor/internal/app"
	"github.com/varun/lexor/internal/languages/registry"
	source "github.com/varun/lexor/internal/registry"
	"github.com/varun/lexor/internal/syncgit"
	p "github.com/varun/lexor/pkg/pluginapi"
	"reflect"
	"strings"
	"time"
)

// OpenAPI derives API models from the same Go types handlers actually use. The
// checked-in TypeScript client models are generated from this document.
func OpenAPI() map[string]any {
	schemas := map[string]any{}
	var schema func(reflect.Type) map[string]any
	schema = func(t reflect.Type) map[string]any {
		if t.Kind() == reflect.Pointer {
			return schema(t.Elem())
		}
		if t == reflect.TypeOf(time.Time{}) {
			return map[string]any{"type": "string", "format": "date-time"}
		}
		if t == reflect.TypeOf(json.RawMessage{}) || t.Kind() == reflect.Interface {
			return map[string]any{}
		}
		switch t.Kind() {
		case reflect.String:
			return map[string]any{"type": "string"}
		case reflect.Bool:
			return map[string]any{"type": "boolean"}
		case reflect.Int, reflect.Int64:
			return map[string]any{"type": "integer"}
		case reflect.Float64:
			return map[string]any{"type": "number"}
		case reflect.Slice:
			return map[string]any{"type": "array", "items": schema(t.Elem())}
		case reflect.Map:
			return map[string]any{"type": "object", "additionalProperties": schema(t.Elem())}
		case reflect.Struct:
			name := t.Name()
			if _, ok := schemas[name]; !ok {
				schemas[name] = map[string]any{}
				properties := map[string]any{}
				required := []string{}
				var fields func(reflect.Type)
				fields = func(typ reflect.Type) {
					for i := 0; i < typ.NumField(); i++ {
						f := typ.Field(i)
						if f.Anonymous {
							fields(f.Type)
							continue
						}
						tag := strings.Split(f.Tag.Get("json"), ",")
						if len(tag) == 0 || tag[0] == "" || tag[0] == "-" {
							continue
						}
						properties[tag[0]] = schema(f.Type)
						if len(tag) == 1 {
							required = append(required, tag[0])
						}
					}
				}
				fields(t)
				schemas[name] = map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
			}
			return map[string]any{"$ref": "#/components/schemas/" + name}
		}
		return map[string]any{}
	}
	for _, v := range []any{SessionResponse{}, HealthResponse{}, RescanResponse{}, PluginsResponse{}, InstalledResponse{}, registry.Info{}, p.Preferences{}, p.Problem{}, p.ProblemView{}, p.ExecutionRequest{}, p.ExecutionResult{}, p.Draft{}, p.Stats{}, p.Progress{}, p.Activity{}, ErrorResponse{}, app.SystemStatus{}, source.Package{}, syncgit.Preview{}, ImportRequest{}} {
		schema(reflect.TypeOf(v))
	}
	paths := map[string]any{}
	add := func(path, method, model string, array bool, body string) {
		resp := map[string]any{"$ref": "#/components/schemas/" + model}
		if array {
			resp = map[string]any{"type": "array", "items": resp}
		}
		op := map[string]any{"responses": map[string]any{"200": map[string]any{"description": "Success", "content": map[string]any{"application/json": map[string]any{"schema": resp}}}, "default": map[string]any{"description": "Error", "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/ErrorResponse"}}}}}}
		if strings.Contains(path, "{id}") {
			op["parameters"] = []any{map[string]any{"name": "id", "in": "path", "required": true, "schema": map[string]any{"type": "string"}}}
		}
		if body != "" {
			op["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/" + body}}}}
		}
		m, ok := paths[path].(map[string]any)
		if !ok {
			m = map[string]any{}
			paths[path] = m
		}
		m[method] = op
	}
	add("/api/v1/settings", "get", "Preferences", false, "")
	add("/api/v1/settings", "put", "Preferences", false, "Preferences")
	add("/api/v1/problems", "get", "ProblemView", true, "")
	add("/api/v1/problems", "post", "ProblemView", false, "Problem")
	add("/api/v1/problems/{id}", "get", "ProblemView", false, "")
	add("/api/v1/problems/{id}", "put", "ProblemView", false, "Problem")
	add("/api/v1/problems/{id}", "delete", "Problem", false, "")
	add("/api/v1/executions/run", "post", "ExecutionResult", false, "ExecutionRequest")
	add("/api/v1/executions/submit", "post", "ExecutionResult", false, "ExecutionRequest")
	add("/api/v1/submissions", "get", "ExecutionResult", true, "")
	add("/api/v1/submissions/{id}", "get", "ExecutionResult", false, "")
	add("/api/v1/drafts/{id}", "get", "Draft", false, "")
	add("/api/v1/drafts/{id}", "put", "Draft", false, "Draft")
	add("/api/v1/stats", "get", "Stats", false, "")
	add("/api/v1/stats/activity", "get", "Activity", true, "")
	add("/api/v1/progress", "get", "Progress", true, "")
	add("/api/v1/progress/{id}", "get", "Progress", false, "")
	add("/api/v1/system/status", "get", "SystemStatus", false, "")
	add("/api/v1/registry/import", "post", "Package", false, "ImportRequest")
	add("/api/v1/sync/status", "get", "Preview", false, "")
	add("/api/v1/sync/export", "post", "Preview", false, "")
	add("/health", "get", "HealthResponse", false, "")
	add("/api/v1/session", "get", "SessionResponse", false, "")
	add("/api/v1/problems/validate", "post", "Problem", false, "Problem")
	add("/api/v1/problems/rescan", "post", "RescanResponse", false, "")
	add("/api/v1/executions", "get", "ExecutionResult", true, "")
	add("/api/v1/languages", "get", "Info", true, "")
	add("/api/v1/plugins", "get", "PluginsResponse", false, "")
	for _, route := range []string{"topics", "tags", "companies"} {
		paths["/api/v1/"+route] = map[string]any{"get": map[string]any{"responses": map[string]any{"200": map[string]any{"description": "Unique sorted values", "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}}}}}}
	}
	paths["/api/v1/intelligence"] = map[string]any{"get": map[string]any{"description": "Same-origin WebSocket upgrade to the restricted LSP bridge. Only initialize, document synchronization, completion, hover and signature requests are allowed.", "responses": map[string]any{"101": map[string]any{"description": "LSP WebSocket connection"}, "503": map[string]any{"description": "Pyright is unavailable or disabled"}}}}
	operation := func(path, method string) map[string]any { return paths[path].(map[string]any)[method].(map[string]any) }
	for _, row := range [][2]string{{"/api/v1/problems/{id}", "delete"}, {"/api/v1/drafts/{id}", "put"}} {
		op := operation(row[0], row[1])
		op["responses"] = map[string]any{"204": map[string]any{"description": "Saved; no response body"}, "default": map[string]any{"description": "Error", "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/ErrorResponse"}}}}}
	}
	op := operation("/api/v1/problems", "post")
	responses := op["responses"].(map[string]any)
	responses["201"] = responses["200"]
	delete(responses, "200")
	op = operation("/api/v1/registry/import", "post")
	op["responses"].(map[string]any)["201"] = map[string]any{"description": "Installed", "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/InstalledResponse"}}}}
	for _, method := range []string{"put", "delete"} {
		op := operation("/api/v1/problems/{id}", method)
		op["parameters"] = append(op["parameters"].([]any), map[string]any{"in": "header", "name": "If-Match", "required": true, "schema": map[string]any{"type": "string"}, "description": "Revision returned by GET; stale revisions return 409"})
	}
	for _, name := range []string{"search", "difficulty", "topic", "tag", "company", "status", "sort", "folder"} {
		op := operation("/api/v1/problems", "get")
		params, _ := op["parameters"].([]any)
		op["parameters"] = append(params, map[string]any{"in": "query", "name": name, "schema": map[string]any{"type": "string"}})
	}
	for _, path := range []string{"/api/v1/submissions", "/api/v1/executions"} {
		operation(path, "get")["parameters"] = []any{map[string]any{"in": "query", "name": "problemId", "schema": map[string]any{"type": "string"}}, map[string]any{"in": "query", "name": "limit", "schema": map[string]any{"type": "integer", "maximum": 200, "default": 100}}}
	}
	for _, path := range []string{"/api/v1/drafts/{id}"} {
		for _, method := range []string{"get", "put"} {
			op := operation(path, method)
			op["parameters"] = append(op["parameters"].([]any), map[string]any{"in": "query", "name": "language", "schema": map[string]any{"type": "string", "default": "python"}})
		}
	}

	return map[string]any{"openapi": "3.0.3", "info": map[string]any{"title": "Lexor API", "version": "1.0.0", "description": "Local single-user API. GET /api/v1/session for the X-Lexor-CSRF token required by mutations. PUT/DELETE problems require If-Match revision. All request bodies are capped at 2 MB."}, "paths": paths, "components": map[string]any{"schemas": schemas}}
}
