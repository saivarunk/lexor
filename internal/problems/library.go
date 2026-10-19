package problems

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	p "github.com/varun/lexor/pkg/pluginapi"
	"github.com/varun/lexor/schemas"
)

var ErrNotFound = errors.New("problem not found")
var ErrConflict = errors.New("problem already exists or was modified; reload before saving")

type Library struct {
	mu     sync.RWMutex
	root   *os.Root
	schema *jsonschema.Schema
	items  map[string]p.ProblemView
	Issues []string
}

func New(directory string) (*Library, error) {
	r, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	var doc any
	if err = json.Unmarshal(schemas.ProblemV1, &doc); err != nil {
		r.Close()
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	if err = compiler.AddResource("https://lexor.dev/schemas/problem/v1.schema.json", doc); err != nil {
		r.Close()
		return nil, err
	}
	s, err := compiler.Compile("https://lexor.dev/schemas/problem/v1.schema.json")
	if err != nil {
		r.Close()
		return nil, err
	}
	l := &Library{root: r, schema: s}
	l.Rescan()
	return l, nil
}
func (l *Library) Close() error { return l.root.Close() }
func (l *Library) Validate(b []byte) (p.Problem, error) {
	var pr p.Problem
	var v any
	if len(b) > 2<<20 {
		return pr, fmt.Errorf("problem exceeds 2 MB")
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return pr, fmt.Errorf("invalid JSON: %w", err)
	}
	if err := l.schema.Validate(v); err != nil {
		return pr, fmt.Errorf("schema validation: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	if err := decoder.Decode(&pr); err != nil {
		return pr, err
	}
	for lang, spec := range pr.Languages {
		seen := map[string]bool{}
		for _, param := range spec.Entrypoint.Parameters {
			if seen[param.Name] {
				return pr, fmt.Errorf("%s: duplicate parameter %s", lang, param.Name)
			}
			seen[param.Name] = true
		}
		for _, tc := range append(append([]p.TestCase{}, pr.Tests.Public...), pr.Tests.Hidden...) {
			if len(tc.Input) != len(seen) {
				return pr, fmt.Errorf("test %q: input keys must match %s parameters", tc.Name, lang)
			}
			for k := range seen {
				if _, ok := tc.Input[k]; !ok {
					return pr, fmt.Errorf("test %q: missing parameter %s", tc.Name, k)
				}
			}
		}
	}
	return pr, nil
}
func revision(b []byte) string      { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func (l *Library) Rescan() []string { l.mu.Lock(); defer l.mu.Unlock(); return l.scan() }
func (l *Library) scan() []string {
	l.items = map[string]p.ProblemView{}
	l.Issues = []string{}
	err := fs.WalkDir(l.root.FS(), ".", func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			l.Issues = append(l.Issues, fmt.Sprintf("%s: %v", file, err))
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			l.Issues = append(l.Issues, file+": symlinks are not allowed")
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(file, ".json") {
			return nil
		}
		b, e := readProblem(l.root.FS(), file)
		if e != nil {
			l.Issues = append(l.Issues, file+": "+e.Error())
			return nil
		}
		pr, e := l.Validate(b)
		if e != nil {
			l.Issues = append(l.Issues, file+": "+e.Error())
			return nil
		}
		if prev, ok := l.items[pr.ID]; ok {
			l.Issues = append(l.Issues, fmt.Sprintf("duplicate ID %s in %s and %s", pr.ID, prev.File, file))
			return nil
		}
		l.items[pr.ID] = p.ProblemView{Problem: pr, File: file, Revision: revision(b), Status: "unsolved"}
		return nil
	})
	if err != nil {
		l.Issues = append(l.Issues, err.Error())
	}
	for _, issue := range l.Issues {
		slog.Warn("problem validation", "issue", issue)
	}
	return append([]string{}, l.Issues...)
}
func (l *Library) Diagnostics() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return append([]string{}, l.Issues...)
}
func (l *Library) List() []p.ProblemView {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]p.ProblemView, 0, len(l.items))
	for _, pr := range l.items {
		out = append(out, pr)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out
}
func (l *Library) Get(id string) (p.ProblemView, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	pr, ok := l.items[id]
	if !ok {
		return pr, ErrNotFound
	}
	return pr, nil
}
func (l *Library) Save(b []byte, id, expectedRevision string) (p.ProblemView, error) {
	pr, err := l.Validate(b)
	if err != nil {
		return p.ProblemView{}, err
	}
	if id != "" && id != pr.ID {
		return p.ProblemView{}, fmt.Errorf("problem ID is immutable")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	file := strings.ReplaceAll(pr.ID, ".", "/") + ".json"
	old, exists := l.items[pr.ID]
	if id == "" && exists {
		return p.ProblemView{}, ErrConflict
	}
	if id != "" && !exists {
		return old, ErrNotFound
	}
	if exists {
		file = old.File
		disk, e := readProblem(l.root.FS(), file)
		if e != nil {
			return old, e
		}
		if expectedRevision == "" || revision(disk) != expectedRevision {
			return old, ErrConflict
		}
	}
	// Every component is checked under the root; symlinks are rejected even inside it.
	parts := strings.Split(path.Dir(file), "/")
	dir := ""
	for _, part := range parts {
		if part == "." {
			continue
		}
		dir = path.Join(dir, part)
		if e := l.root.Mkdir(dir, 0700); e != nil && !os.IsExist(e) {
			return old, e
		}
		st, e := l.root.Lstat(dir)
		if e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return old, fmt.Errorf("unsafe problem directory")
		}
	}
	if !exists {
		if _, e := l.root.Lstat(file); e == nil {
			return old, ErrConflict
		} else if !os.IsNotExist(e) {
			return old, e
		}
	}
	if st, e := l.root.Lstat(file); e == nil && st.Mode()&os.ModeSymlink != 0 {
		return old, fmt.Errorf("symlinks are not allowed")
	}
	pretty, _ := json.MarshalIndent(pr, "", "  ")
	pretty = append(pretty, '\n')
	tmp := file + ".tmp"
	f, e := l.root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return old, e
	}
	defer l.root.Remove(tmp)
	_, e = f.Write(pretty)
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e != nil {
		return old, e
	}
	if closeErr != nil {
		return old, closeErr
	}
	if e = l.root.Rename(tmp, file); e != nil {
		return old, e
	}
	l.scan()
	return l.items[pr.ID], nil
}
func (l *Library) Delete(id, rev string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	pr, ok := l.items[id]
	if !ok {
		return ErrNotFound
	}
	b, err := readProblem(l.root.FS(), pr.File)
	if err != nil {
		return err
	}
	if rev == "" || revision(b) != rev {
		return ErrConflict
	}
	if err = l.root.Remove(pr.File); err != nil {
		return err
	}
	l.scan()
	return nil
}

func readProblem(root fs.FS, file string) ([]byte, error) {
	f, err := root.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, (2<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(b) > 2<<20 {
		return nil, fmt.Errorf("problem exceeds 2 MB")
	}
	return b, nil
}
