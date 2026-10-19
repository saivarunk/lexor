package syncgit

import (
	"context"
	"github.com/varun/lexor/internal/testutil"
	p "github.com/varun/lexor/pkg/pluginapi"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAllowlistedExportAndSymlinkDefense(t *testing.T) {
	a := testutil.App(t)
	ctx := context.Background()
	a.Store.SaveDraft(ctx, p.Draft{ProblemID: "private", Language: "python", Source: "secret-draft"})
	a.Store.Record(ctx, p.ExecutionResult{ID: "accepted", ProblemID: "arrays.two-sum", Language: "python", Source: "accepted-source", Mode: "submit", Status: "Accepted", CreatedAt: time.Now().UTC()})
	e := Exporter{Store: a.Store, Directory: t.TempDir()}
	preview, err := e.Preview(ctx)
	if err != nil || len(preview.Files) != 4 {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	entries, _ := os.ReadDir(e.Directory)
	if len(entries) != 0 {
		t.Fatal("preview wrote files")
	}
	if _, err = e.Export(ctx); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join(e.Directory, "solutions/arrays/two-sum/python.py"))
	if err != nil || string(source) != "accepted-source" {
		t.Fatal("source export missing")
	}
	for _, f := range preview.Files {
		b, err := os.ReadFile(filepath.Join(e.Directory, f))
		if err != nil || strings.Contains(string(b), "secret-draft") {
			t.Fatal("export leaked drafts")
		}
		if strings.Contains(f, "db") || strings.Contains(f, "yaml") {
			t.Fatal("unsafe export")
		}
	}
	other := t.TempDir()
	e.Directory = t.TempDir()
	if err = os.Symlink(other, filepath.Join(e.Directory, "solutions")); err != nil {
		t.Fatal(err)
	}
	if _, err = e.Export(ctx); err == nil {
		t.Fatal("symlink accepted")
	}
	files, _ := os.ReadDir(other)
	if len(files) != 0 {
		t.Fatal("export escaped root")
	}
}

func TestGitCommitExcludesUnrelatedStagedFiles(t *testing.T) {
	a := testutil.App(t)
	ctx := context.Background()
	e := Exporter{Store: a.Store, Directory: t.TempDir()}
	if err := e.Init(ctx); err != nil {
		t.Fatal(err)
	}
	for _, kv := range [][2]string{{"user.email", "test@example.invalid"}, {"user.name", "Lexor test"}} {
		if err := exec.Command("git", "-C", e.Directory, "config", kv[0], kv[1]).Run(); err != nil {
			t.Fatal(err)
		}
	}
	secret := filepath.Join(e.Directory, "unrelated-secret.txt")
	os.WriteFile(secret, []byte("must not be committed"), 0600)
	if err := exec.Command("git", "-C", e.Directory, "add", "unrelated-secret.txt").Run(); err != nil {
		t.Fatal(err)
	}
	preview, err := e.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.Commit(ctx, preview.Files, false); err != nil {
		t.Fatal(err)
	}
	b, err := exec.Command("git", "-C", e.Directory, "ls-tree", "-r", "--name-only", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "unrelated-secret") {
		t.Fatal("committed unrelated staged content")
	}
	if !strings.Contains(string(b), "manifest.json") {
		t.Fatal("export not committed")
	}
}

func TestOptionalPushToLocalBareRepository(t *testing.T) {
	a := testutil.App(t)
	ctx := context.Background()
	e := Exporter{Store: a.Store, Directory: t.TempDir()}
	remote := filepath.Join(t.TempDir(), "backup.git")
	if b, err := exec.Command("git", "init", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatal(string(b), err)
	}
	if err := e.Init(ctx); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		t.Helper()
		b, err := exec.Command("git", append([]string{"-C", e.Directory}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal(string(b), err)
		}
		return strings.TrimSpace(string(b))
	}
	run("config", "user.email", "test@example.invalid")
	run("config", "user.name", "Lexor test")
	plan, err := e.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.Commit(ctx, plan.Files, false); err != nil {
		t.Fatal(err)
	}
	run("remote", "add", "origin", remote)
	run("push", "--set-upstream", "origin", "HEAD")
	plan, err = e.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.Commit(ctx, plan.Files, true); err != nil {
		t.Fatal(err)
	}
	local := run("rev-parse", "HEAD")
	b, err := exec.Command("git", "-C", remote, "log", "--all", "-1", "--format=%H").Output()
	if err != nil || strings.TrimSpace(string(b)) != local {
		t.Fatalf("push did not reach local backup: %s %v", b, err)
	}
}
