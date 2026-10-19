package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/varun/lexor/internal/server"
	"github.com/varun/lexor/internal/testutil"
	p "github.com/varun/lexor/pkg/pluginapi"
)

type client struct {
	t     *testing.T
	s     http.Handler
	token string
}

func (c client) call(method, path string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	c.t.Helper()
	req := httptest.NewRequest(method, "http://localhost:8080"+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("X-Lexor-CSRF", c.token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	c.s.ServeHTTP(w, req)
	return w
}
func newClient(t *testing.T) client {
	a := testutil.App(t)
	c := client{t: t, s: server.New(a, "http://localhost:8080")}
	w := c.call("GET", "/api/v1/session", nil, nil)
	var v map[string]string
	json.Unmarshal(w.Body.Bytes(), &v)
	c.token = v["csrfToken"]
	return c
}
func TestAPIProblemLifecycleAndFiltering(t *testing.T) {
	c := newClient(t)
	pr := testutil.Problem(t)
	pr.ID = "custom.api"
	pr.Organization.Companies = []string{"ExampleCo"}
	w := c.call("POST", "/api/v1/problems", testutil.JSON(t, pr), nil)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var saved p.ProblemView
	json.Unmarshal(w.Body.Bytes(), &saved)
	if saved.Revision == "" {
		t.Fatal("missing revision")
	}
	w = c.call("GET", "/api/v1/problems?search=Two&company=ExampleCo&difficulty=easy&status=unsolved", nil, nil)
	var items []p.ProblemView
	json.Unmarshal(w.Body.Bytes(), &items)
	if w.Code != 200 || len(items) != 1 || items[0].ID != pr.ID {
		t.Fatal("filter failure", w.Body.String())
	}
	pr.Title = "Modified"
	w = c.call("PUT", "/api/v1/problems/"+pr.ID, testutil.JSON(t, pr), nil)
	if w.Code != 409 {
		t.Fatal("missing If-Match must conflict", w.Code)
	}
	w = c.call("PUT", "/api/v1/problems/"+pr.ID, testutil.JSON(t, pr), map[string]string{"If-Match": saved.Revision})
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	json.Unmarshal(w.Body.Bytes(), &saved)
	w = c.call("DELETE", "/api/v1/problems/"+pr.ID, []byte(`{}`), map[string]string{"If-Match": saved.Revision})
	if w.Code != 204 {
		t.Fatal(w.Body.String())
	}
	if c.call("GET", "/api/v1/problems/"+pr.ID, nil, nil).Code != 404 {
		t.Fatal("deleted problem returned")
	}
}
func TestLocalhostSecurityAndAPIFallback(t *testing.T) {
	c := newClient(t)
	for _, tc := range []struct {
		name, method, path string
		headers            map[string]string
		token              bool
		want               int
	}{
		{"CSRF required", "POST", "/api/v1/problems/rescan", nil, false, 403},
		{"origin blocked", "POST", "/api/v1/problems/rescan", map[string]string{"Origin": "https://evil.example"}, true, 403},
		{"rebinding blocked", "GET", "/api/v1/problems", map[string]string{"Origin": "http://evil.example"}, true, 403},
		{"cross site blocked", "GET", "/api/v1/session", map[string]string{"Sec-Fetch-Site": "cross-site"}, true, 403},
		{"API fallback stays JSON", "GET", "/api/v1/not-found", nil, true, 404},
		{"SPA fallback", "GET", "/practice/arrays.two-sum", nil, true, 200},
		{"request limit", "POST", "/api/v1/problems", nil, true, 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cc := c
			if !tc.token {
				cc.token = ""
			}
			body := []byte(`{}`)
			if tc.name == "request limit" {
				body = bytes.Repeat([]byte("a"), (2<<20)+1)
			}
			w := cc.call(tc.method, tc.path, body, tc.headers)
			if w.Code != tc.want {
				t.Fatalf("got %d want %d: %s", w.Code, tc.want, w.Body.String())
			}
			if tc.name == "API fallback stays JSON" && !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
				t.Fatal("API returned SPA")
			}
		})
	}
	req := httptest.NewRequest("GET", "http://evil.example:8080/api/v1/session", nil)
	w := httptest.NewRecorder()
	c.s.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatal("unsafe Host accepted")
	}
}
func TestDraftAndRequestValidation(t *testing.T) {
	c := newClient(t)
	draft := p.Draft{ProblemID: "arrays.two-sum", Language: "python", Source: "def solve(): pass"}
	w := c.call("PUT", "/api/v1/drafts/arrays.two-sum", testutil.JSON(t, draft), nil)
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = c.call("GET", "/api/v1/drafts/arrays.two-sum", nil, nil)
	var saved p.Draft
	json.Unmarshal(w.Body.Bytes(), &saved)
	if saved.Source != draft.Source {
		t.Fatal("draft not persisted")
	}
	w = c.call("POST", "/api/v1/executions/run", []byte(`{"problemId":"arrays.two-sum","language":"bash","source":"echo nope"}`), nil)
	if w.Code != 400 {
		t.Fatal("unknown language accepted")
	}
	w = c.call("POST", "/api/v1/executions/run", []byte(`{} {}`), nil)
	if w.Code != 400 {
		t.Fatal("trailing JSON accepted")
	}
}
