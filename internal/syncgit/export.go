package syncgit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/varun/lexor/internal/persistence"
)

type Exporter struct {
	Store     *persistence.Store
	Directory string
}
type Preview struct {
	Directory string   `json:"directory"`
	Files     []string `json:"files"`
}

func (e *Exporter) files(ctx context.Context) (map[string][]byte, error) {
	progress, err := e.Store.Progress(ctx)
	if err != nil {
		return nil, err
	}
	activity, err := e.Store.Activity(ctx)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	add := func(name string, v any) { b, _ := json.MarshalIndent(v, "", "  "); files[name] = append(b, '\n') }
	add("manifest.json", map[string]any{"schemaVersion": "1.0.0", "format": "lexor-progress", "exportedAt": time.Now().UTC(), "timezone": e.Store.Location.String()})
	add("progress/progress.json", progress)
	add("stats/activity.json", activity)
	// Only the latest accepted Python submission for each problem is exported.
	rows, err := e.Store.DB.QueryContext(ctx, `SELECT problem_id,language,source FROM executions e WHERE mode='submit' AND status='Accepted' AND NOT EXISTS(SELECT 1 FROM executions n WHERE n.problem_id=e.problem_id AND n.language=e.language AND n.mode='submit' AND n.status='Accepted' AND n.created_at>e.created_at) ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, lang, source string
		if err = rows.Scan(&id, &lang, &source); err != nil {
			return nil, err
		}
		if lang != "python" {
			continue
		}
		if strings.ContainsAny(id, "/\\") || strings.Contains(id, "..") {
			return nil, fmt.Errorf("unsafe problem ID")
		}
		name := "solutions/" + strings.ReplaceAll(id, ".", "/") + "/python.py"
		if _, ok := files[name]; !ok {
			files[name] = []byte(source)
		}
	}
	return files, rows.Err()
}
func (e *Exporter) Preview(ctx context.Context) (Preview, error) {
	files, err := e.files(ctx)
	p := Preview{Directory: e.Directory, Files: []string{}}
	for name := range files {
		p.Files = append(p.Files, name)
	}
	sort.Strings(p.Files)
	return p, err
}
func (e *Exporter) Init(ctx context.Context) error {
	if err := os.MkdirAll(e.Directory, 0700); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "git", "-c", "core.hooksPath=/dev/null", "init", e.Directory)
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git init: %s: %w", b, err)
	}
	return nil
}
func (e *Exporter) Export(ctx context.Context) (Preview, error) {
	files, err := e.files(ctx)
	if err != nil {
		return Preview{}, err
	}
	if err = os.MkdirAll(e.Directory, 0700); err != nil {
		return Preview{}, err
	}
	root, err := os.OpenRoot(e.Directory)
	if err != nil {
		return Preview{}, err
	}
	defer root.Close()
	preview := Preview{Directory: e.Directory, Files: []string{}}
	for name, b := range files {
		dir := ""
		for _, part := range strings.Split(filepath.ToSlash(filepath.Dir(name)), "/") {
			if part == "." {
				continue
			}
			dir = filepath.Join(dir, part)
			if err = root.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
				return preview, err
			}
			st, e := root.Lstat(dir)
			if e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
				return preview, fmt.Errorf("unsafe export directory")
			}
		}
		if st, e := root.Lstat(name); e == nil && st.Mode()&os.ModeSymlink != 0 {
			return preview, fmt.Errorf("symlink export destination rejected")
		}
		tmp := name + ".tmp"
		f, e := root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return preview, e
		}
		_, err = f.Write(b)
		if err == nil {
			err = f.Sync()
		}
		f.Close()
		if err != nil {
			root.Remove(tmp)
			return preview, err
		}
		if err = root.Rename(tmp, name); err != nil {
			root.Remove(tmp)
			return preview, err
		}
		preview.Files = append(preview.Files, name)
	}
	sort.Strings(preview.Files)
	return preview, nil
}
func (e *Exporter) Commit(ctx context.Context, files []string, push bool) error {
	if len(files) == 0 {
		return nil
	}
	args := []string{"-C", e.Directory, "-c", "core.hooksPath=/dev/null", "add", "--"}
	args = append(args, files...)
	if b, err := exec.CommandContext(ctx, "git", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %s: %w", b, err)
	}
	// Commit only the allowlist; unrelated staged files must never be included.
	args = []string{"-C", e.Directory, "-c", "core.hooksPath=/dev/null", "commit", "--only", "-m", "Export Lexor progress", "--"}
	args = append(args, files...)
	if b, err := exec.CommandContext(ctx, "git", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("git commit: %s: %w", b, err)
	}
	if push {
		if b, err := exec.CommandContext(ctx, "git", "-C", e.Directory, "-c", "core.hooksPath=/dev/null", "push").CombinedOutput(); err != nil {
			return fmt.Errorf("git push: %s: %w", b, err)
		}
	}
	return nil
}
