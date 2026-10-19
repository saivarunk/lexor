package registry

import (
	"context"
	"github.com/varun/lexor/internal/testutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestImportPreviewValidationAndCollision(t *testing.T) {
	a := testutil.App(t)
	r := &Resolver{Library: a.Library}
	source := t.TempDir()
	pr := testutil.Problem(t)
	pr.ID = "imported.two-sum"
	file := filepath.Join(source, "two-sum.json")
	if err := os.WriteFile(file, testutil.JSON(t, pr), 0600); err != nil {
		t.Fatal(err)
	}
	pack, err := r.Resolve(context.Background(), source)
	if err != nil || len(pack.Problems) != 1 || pack.Digest == "" {
		t.Fatalf("resolve: %+v %v", pack, err)
	}
	if _, err = a.Library.Get(pr.ID); err == nil {
		t.Fatal("preview installed data")
	}
	ids, err := r.Install(pack)
	if err != nil || len(ids) != 1 {
		t.Fatal(err)
	}
	if _, err = r.Install(pack); err == nil {
		t.Fatal("collision must fail")
	}
	pr.SchemaVersion = "99"
	os.WriteFile(file, testutil.JSON(t, pr), 0600)
	if _, err = r.Resolve(context.Background(), source); err == nil {
		t.Fatal("invalid version accepted")
	}
}
func TestManifestTraversalAndNetworkPolicy(t *testing.T) {
	a := testutil.App(t)
	r := &Resolver{Library: a.Library}
	source := t.TempDir()
	os.WriteFile(filepath.Join(source, "lexor-pack.json"), []byte(`{"schemaVersion":"1.0.0","problems":["../escape.json"]}`), 0600)
	if _, err := r.Resolve(context.Background(), source); err == nil {
		t.Fatal("manifest traversal accepted")
	}
	for _, ip := range []string{"127.0.0.1", "::1", "10.0.0.1", "169.254.169.254", "100.64.1.1", "192.168.1.1", "::ffff:127.0.0.1"} {
		if publicIP(ip) {
			t.Fatalf("unsafe IP %s", ip)
		}
	}
	if !publicIP("1.1.1.1") {
		t.Fatal("public IP rejected")
	}
	for _, source := range []string{"http://example.com/a.json", "https://127.0.0.1/a.json", "https://user:pass@example.com/a.json"} {
		if _, err := r.Resolve(context.Background(), source); err == nil {
			t.Fatalf("unsafe source accepted: %s", source)
		}
	}
}
func TestLocalGitManifest(t *testing.T) {
	a := testutil.App(t)
	r := &Resolver{Library: a.Library}
	dir := t.TempDir()
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Fatal(err)
	}
	pr := testutil.Problem(t)
	pr.ID = "pack.example"
	os.WriteFile(filepath.Join(dir, "example.json"), testutil.JSON(t, pr), 0600)
	os.WriteFile(filepath.Join(dir, "lexor-pack.json"), []byte(`{"schemaVersion":"1.0.0","name":"Test","version":"1","problems":["example.json"]}`), 0600)
	pack, err := r.ResolveGit(context.Background(), dir)
	if err != nil || len(pack.Problems) != 1 {
		t.Fatalf("local git: %+v %v", pack, err)
	}
}
