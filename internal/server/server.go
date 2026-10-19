package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/varun/lexor/internal/app"
	"github.com/varun/lexor/internal/intelligence"
	"github.com/varun/lexor/internal/problems"
	source "github.com/varun/lexor/internal/registry"
	"github.com/varun/lexor/internal/syncgit"
	p "github.com/varun/lexor/pkg/pluginapi"
	"github.com/varun/lexor/web"
)

type Server struct {
	App      *app.App
	mux      *http.ServeMux
	token    string
	origin   string
	exporter *syncgit.Exporter
	resolver *source.Resolver
}
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type ErrorResponse struct {
	Error APIError `json:"error"`
}

func New(a *app.App, origin string) *Server {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	s := &Server{App: a, mux: http.NewServeMux(), token: hex.EncodeToString(b), origin: origin, exporter: &syncgit.Exporter{Store: a.Store, Directory: a.Config.Path(a.Config.Git.Directory)}, resolver: &source.Resolver{Library: a.Library}}
	s.routes()
	return s
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("request panic", "error", rec)
			fail(w, 500, "internal_error", "Unexpected server error")
		}
		slog.Info("http request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	}()
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; worker-src 'self' blob:; style-src 'self' 'unsafe-inline'; font-src 'self' data:; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	expected, _ := url.Parse(s.origin)
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
		port = ""
	}
	expectedPort := expected.Port()
	if expectedPort == "" {
		expectedPort = "80"
	}
	validHost := (host == expected.Hostname() || ((expected.Hostname() == "127.0.0.1" || expected.Hostname() == "localhost") && (host == "localhost" || host == "127.0.0.1"))) && (port == expectedPort || port == "" && expectedPort == "80")
	if !validHost {
		fail(w, 403, "unsafe_host", "Host is not allowed")
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, e := url.Parse(origin)
		if e != nil || u.Scheme != expected.Scheme || u.Host != r.Host {
			fail(w, 403, "unsafe_origin", "Cross-origin requests are blocked")
			return
		}
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site == "cross-site" {
		fail(w, 403, "unsafe_origin", "Cross-site requests are blocked")
		return
	}
	if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" {
		token := r.Header.Get("X-Lexor-CSRF")
		if subtle.ConstantTimeCompare([]byte(token), []byte(s.token)) != 1 {
			fail(w, 403, "csrf", "Missing or invalid X-Lexor-CSRF token; GET /api/v1/session first")
			return
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			fail(w, 415, "content_type", "Use application/json")
			return
		}
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	s.mux.ServeHTTP(w, r)
}
func (s *Server) routes() {
	m := s.mux
	m.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, HealthResponse{Status: "ok"}) })
	m.HandleFunc("GET /api/v1/session", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		respond(w, 200, SessionResponse{CSRFToken: s.token})
	})
	m.HandleFunc("GET /api/v1/settings", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.App.Store.Preferences(r.Context())
		if err != nil {
			handleErr(w, err)
			return
		}
		respond(w, 200, v)
	})
	m.HandleFunc("PUT /api/v1/settings", func(w http.ResponseWriter, r *http.Request) {
		var v p.Preferences
		if !decode(w, r, &v) {
			return
		}
		if (v.Theme != "dark" && v.Theme != "light") || (v.EditorTheme != "dark" && v.EditorTheme != "light") || v.EditorFontSize < 10 || v.EditorFontSize > 24 {
			fail(w, 400, "invalid_settings", "Themes must be dark/light; font size 10–24")
			return
		}
		if err := s.App.Store.SavePreferences(r.Context(), v); err != nil {
			handleErr(w, err)
			return
		}
		respond(w, 200, v)
	})
	m.HandleFunc("GET /api/v1/system/status", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, s.App.Status(r.Context())) })
	m.HandleFunc("GET /api/v1/problems", s.list)
	m.HandleFunc("POST /api/v1/problems/rescan", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, RescanResponse{Issues: s.App.Library.Rescan()})
	})
	m.HandleFunc("POST /api/v1/problems/validate", func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			handleErr(w, err)
			return
		}
		pr, err := s.App.Library.Validate(raw)
		if err != nil {
			handleErr(w, err)
			return
		}
		respond(w, 200, pr)
	})
	m.HandleFunc("POST /api/v1/problems", s.save)
	m.HandleFunc("GET /api/v1/problems/{id}", s.get)
	m.HandleFunc("PUT /api/v1/problems/{id}", s.save)
	m.HandleFunc("DELETE /api/v1/problems/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.App.Library.Delete(r.PathValue("id"), strings.Trim(r.Header.Get("If-Match"), "\"")); err != nil {
			handleErr(w, err)
			return
		}
		w.WriteHeader(204)
	})
	for _, kind := range []string{"topics", "tags", "companies"} {
		m.HandleFunc("GET /api/v1/"+kind, func(w http.ResponseWriter, r *http.Request) {
			set := map[string]bool{}
			for _, pr := range s.App.Library.List() {
				values := pr.Organization.Tags
				if kind == "topics" {
					values = []string{pr.Organization.Topic}
				}
				if kind == "companies" {
					values = pr.Organization.Companies
				}
				for _, v := range values {
					set[v] = true
				}
			}
			values := []string{}
			for v := range set {
				values = append(values, v)
			}
			sort.Strings(values)
			respond(w, 200, values)
		})
	}
	for _, mode := range []string{"run", "submit"} {
		m.HandleFunc("POST /api/v1/executions/"+mode, func(w http.ResponseWriter, r *http.Request) {
			var req p.ExecutionRequest
			if !decode(w, r, &req) {
				return
			}
			if req.Language == "" {
				req.Language = s.App.Config.Execution.DefaultLanguage
			}
			result, err := s.App.Engine.Execute(r.Context(), req, mode)
			if err != nil {
				handleErr(w, err)
				return
			}
			respond(w, 200, result)
		})
	}
	m.HandleFunc("GET /api/v1/submissions", func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		out, err := s.App.Store.Executions(r.Context(), "submit", r.URL.Query().Get("problemId"), limit)
		if err != nil {
			handleErr(w, err)
			return
		}
		for i := range out {
			out[i].Source = ""
		}
		respond(w, 200, out)
	})
	m.HandleFunc("GET /api/v1/executions", func(w http.ResponseWriter, r *http.Request) {
		out, err := s.App.Store.Executions(r.Context(), "", r.URL.Query().Get("problemId"), 100)
		if err != nil {
			handleErr(w, err)
			return
		}
		for i := range out {
			out[i].Source = ""
		}
		respond(w, 200, out)
	})
	m.HandleFunc("GET /api/v1/submissions/{id}", func(w http.ResponseWriter, r *http.Request) {
		result, err := s.App.Store.Execution(r.Context(), r.PathValue("id"))
		if err != nil {
			handleErr(w, err)
			return
		}
		respond(w, 200, result)
	})
	m.HandleFunc("GET /api/v1/progress", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.App.Store.Progress(r.Context())
		if err != nil {
			handleErr(w, err)
			return
		}
		respond(w, 200, v)
	})
	m.HandleFunc("GET /api/v1/progress/{id}", func(w http.ResponseWriter, r *http.Request) {
		values, err := s.App.Store.Progress(r.Context())
		if err != nil {
			handleErr(w, err)
			return
		}
		for _, v := range values {
			if v.ProblemID == r.PathValue("id") {
				respond(w, 200, v)
				return
			}
		}
		fail(w, 404, "not_found", "No attempts yet")
	})
	m.HandleFunc("GET /api/v1/drafts/{id}", s.draft)
	m.HandleFunc("PUT /api/v1/drafts/{id}", s.draft)
	m.HandleFunc("GET /api/v1/stats", func(w http.ResponseWriter, r *http.Request) {
		stats, err := s.App.Store.Stats(r.Context(), s.App.Library.List())
		if err != nil {
			handleErr(w, err)
			return
		}
		respond(w, 200, stats)
	})
	m.HandleFunc("GET /api/v1/stats/activity", func(w http.ResponseWriter, r *http.Request) {
		values, err := s.App.Store.Activity(r.Context())
		if err != nil {
			handleErr(w, err)
			return
		}
		respond(w, 200, values)
	})
	m.HandleFunc("GET /api/v1/languages", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, s.App.Languages.List()) })
	m.HandleFunc("GET /api/v1/plugins", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, PluginsResponse{APIVersion: p.Version, Languages: s.App.Languages.List(), Sandboxes: []string{s.App.Sandbox.ID()}, Evaluators: []string{"exact", "float", "unordered"}, Sources: []string{"local", "https", "git"}})
	})
	m.Handle("GET /api/v1/intelligence", intelligence.New(s.App.Config.Intelligence.Command, s.App.Config.Intelligence.Enabled))
	m.HandleFunc("POST /api/v1/registry/import", s.importProblems)
	m.HandleFunc("GET /api/v1/sync/status", func(w http.ResponseWriter, r *http.Request) {
		preview, err := s.exporter.Preview(r.Context())
		if err != nil {
			handleErr(w, err)
			return
		}
		respond(w, 200, preview)
	})
	m.HandleFunc("POST /api/v1/sync/export", func(w http.ResponseWriter, r *http.Request) {
		preview, err := s.exporter.Export(r.Context())
		if err != nil {
			handleErr(w, err)
			return
		}
		respond(w, 200, preview)
	})
	m.HandleFunc("GET /api/v1/openapi.json", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, OpenAPI()) })
	m.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { fail(w, 404, "not_found", "API route not found") })
	m.HandleFunc("/", s.static)
}
func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	pr, err := s.App.Library.Get(r.PathValue("id"))
	if err != nil {
		handleErr(w, err)
		return
	}
	items, err := s.App.Problems(r.Context())
	if err != nil {
		handleErr(w, err)
		return
	}
	for _, item := range items {
		if item.ID == pr.ID {
			pr = item
			break
		}
	}
	w.Header().Set("ETag", "\""+pr.Revision+"\"")
	respond(w, 200, pr)
}
func (s *Server) save(w http.ResponseWriter, r *http.Request) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		handleErr(w, err)
		return
	}
	id := r.PathValue("id")
	pr, err := s.App.Library.Save(b, id, strings.Trim(r.Header.Get("If-Match"), "\""))
	if err != nil {
		handleErr(w, err)
		return
	}
	status := 200
	if id == "" {
		status = 201
	}
	w.Header().Set("ETag", "\""+pr.Revision+"\"")
	respond(w, status, pr)
}
func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	items, err := s.App.Problems(r.Context())
	if err != nil {
		handleErr(w, err)
		return
	}
	q := r.URL.Query()
	out := []p.ProblemView{}
	contains := func(values []string, needle string) bool {
		for _, v := range values {
			if strings.EqualFold(v, needle) {
				return true
			}
		}
		return false
	}
	for _, pr := range items {
		if search := q.Get("search"); search != "" && !strings.Contains(strings.ToLower(pr.Title), strings.ToLower(search)) {
			continue
		}
		if d := q.Get("difficulty"); d != "" && pr.Difficulty != d {
			continue
		}
		if topic := q.Get("topic"); topic != "" && !strings.EqualFold(topic, pr.Organization.Topic) {
			continue
		}
		if tag := q.Get("tag"); tag != "" && !contains(pr.Organization.Tags, tag) {
			continue
		}
		if company := q.Get("company"); company != "" && !contains(pr.Organization.Companies, company) {
			continue
		}
		if status := q.Get("status"); status != "" && pr.Status != status {
			continue
		}
		if folder := q.Get("folder"); folder != "" && !strings.HasPrefix(pr.File, strings.TrimSuffix(folder, "/")+"/") {
			continue
		}
		out = append(out, pr)
	}
	switch q.Get("sort") {
	case "difficulty":
		rank := map[string]int{"easy": 0, "medium": 1, "hard": 2}
		sort.SliceStable(out, func(i, j int) bool { return rank[out[i].Difficulty] < rank[out[j].Difficulty] })
	case "recent":
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].LastAttemptedAt == nil {
				return false
			}
			if out[j].LastAttemptedAt == nil {
				return true
			}
			return out[i].LastAttemptedAt.After(*out[j].LastAttemptedAt)
		})
	}
	respond(w, 200, out)
}
func (s *Server) draft(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.App.Library.Get(id); err != nil {
		handleErr(w, err)
		return
	}
	language := r.URL.Query().Get("language")
	if language == "" {
		language = "python"
	}
	if _, err := s.App.Languages.Resolve(language); err != nil {
		handleErr(w, err)
		return
	}
	if r.Method == "GET" {
		d, err := s.App.Store.Draft(r.Context(), id, language)
		if errors.Is(err, sql.ErrNoRows) {
			respond(w, 200, p.Draft{ProblemID: id, Language: language})
			return
		}
		if err != nil {
			handleErr(w, err)
			return
		}
		respond(w, 200, d)
		return
	}
	var d p.Draft
	if !decode(w, r, &d) {
		return
	}
	if d.ProblemID != id || d.Language != language || len(d.Source) > 128<<10 {
		fail(w, 400, "invalid_draft", "Draft identity/language must match URL; source limit is 128 KB")
		return
	}
	if err := s.App.Store.SaveDraft(r.Context(), d); err != nil {
		handleErr(w, err)
		return
	}
	w.WriteHeader(204)
}

type ImportRequest struct {
	Source        string `json:"source"`
	Git           bool   `json:"git"`
	ConfirmDigest string `json:"confirmDigest,omitempty"`
}

func (s *Server) importProblems(w http.ResponseWriter, r *http.Request) {
	var req ImportRequest
	if !decode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Source) == "" {
		fail(w, 400, "invalid_source", "Provide a source")
		return
	}
	var pack source.Package
	var err error
	if req.Git {
		pack, err = s.resolver.ResolveGit(r.Context(), req.Source)
	} else {
		pack, err = s.resolver.Resolve(r.Context(), req.Source)
	}
	if err != nil {
		handleErr(w, err)
		return
	}
	if req.ConfirmDigest == "" {
		respond(w, 200, pack)
		return
	}
	if req.ConfirmDigest != pack.Digest {
		fail(w, 409, "source_changed", "Source changed since preview; preview again")
		return
	}
	ids, err := s.resolver.Install(pack)
	if err != nil {
		handleErr(w, err)
		return
	}
	if err = s.App.Store.RecordInstallation(r.Context(), pack.Source, pack.Digest, ids); err != nil {
		handleErr(w, err)
		return
	}
	respond(w, 201, InstalledResponse{Installed: ids})
}
func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		fail(w, 405, "method_not_allowed", "Method not allowed")
		return
	}
	assets, _ := fs.Sub(web.Assets, "dist")
	file := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if file == "." || file == "" {
		file = "index.html"
	}
	if _, err := fs.Stat(assets, file); err != nil {
		if strings.HasPrefix(file, "assets/") || path.Ext(file) == ".ico" || path.Ext(file) == ".png" || path.Ext(file) == ".svg" {
			http.NotFound(w, r)
			return
		}
		file = "index.html"
	}
	if strings.HasPrefix(file, "assets/") {
		w.Header().Set("Cache-Control", "public,max-age=31536000,immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	f, err := assets.Open(file)
	if err != nil && file == "index.html" {
		http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(web.Fallback))
		return
	}
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	reader, ok := f.(io.ReadSeeker)
	if !ok {
		fail(w, 500, "asset_error", "Embedded asset is not seekable")
		return
	}
	http.ServeContent(w, r, st.Name(), time.Time{}, reader)
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var max *http.MaxBytesError
		if errors.As(err, &max) {
			fail(w, 413, "too_large", "Request exceeds 2 MB")
			return false
		}
		fail(w, 400, "invalid_request", err.Error())
		return false
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		fail(w, 400, "invalid_request", "Expected one JSON value")
		return false
	}
	return true
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "error", err)
	}
}
func fail(w http.ResponseWriter, status int, code, message string) {
	respond(w, status, ErrorResponse{APIError{code, message}})
}
func handleErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, problems.ErrNotFound) || errors.Is(err, sql.ErrNoRows):
		fail(w, 404, "not_found", err.Error())
	case errors.Is(err, problems.ErrConflict) || strings.Contains(err.Error(), "collision") || strings.Contains(err.Error(), "busy"):
		fail(w, 409, "conflict", err.Error())
	default:
		var max *http.MaxBytesError
		if errors.As(err, &max) {
			fail(w, 413, "too_large", "Request exceeds 2 MB")
			return
		}
		var pathErr *os.PathError
		var dbErr interface{ Code() int }
		if errors.As(err, &pathErr) || errors.As(err, &dbErr) || strings.HasPrefix(err.Error(), "persist execution:") {
			slog.Error("server operation failed", "error", err)
			fail(w, 500, "internal_error", err.Error())
			return
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			fail(w, 408, "request_timeout", "Request canceled or timed out")
			return
		}
		slog.Warn("request failed", "error", err)
		fail(w, 400, "invalid_request", err.Error())
	}
}
