package problems_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/varun/lexor/internal/problems"
	"github.com/varun/lexor/internal/testutil"
)

func TestSchemaValidationAndNull(t *testing.T) {
	a := testutil.App(t)
	pr := testutil.Problem(t)
	for _, mutation := range []struct {
		name   string
		change func()
	}{
		{"unsupported version", func() { pr.SchemaVersion = "2.0.0" }},
		{"path traversal", func() { pr.ID = "../../escape" }},
		{"entrypoint injection", func() {
			pr.Languages["python"] = pr.Languages["python"]
			spec := pr.Languages["python"]
			spec.Entrypoint.Name = "solve(); print(1)"
			pr.Languages["python"] = spec
		}},
		{"missing inputs", func() { pr.Tests.Public[0].Input = map[string]any{} }},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			pr = testutil.Problem(t)
			mutation.change()
			if _, err := a.Library.Validate(testutil.JSON(t, pr)); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	pr = testutil.Problem(t)
	pr.Tests.Public[0].Expected = nil
	pr.Tests.Public[0].Input["nums"] = nil
	if _, err := a.Library.Validate(testutil.JSON(t, pr)); err != nil {
		t.Fatal(err)
	}
}
func TestCRUDRevisionAndExternalChanges(t *testing.T) {
	a := testutil.App(t)
	pr := testutil.Problem(t)
	pr.ID = "custom.example"
	pr.Slug = "example"
	saved, err := a.Library.Save(testutil.JSON(t, pr), "", "")
	if err != nil {
		t.Fatal(err)
	}
	pr.Title = "Updated"
	if _, err = a.Library.Save(testutil.JSON(t, pr), pr.ID, ""); !errors.Is(err, problems.ErrConflict) {
		t.Fatalf("expected missing-revision conflict: %v", err)
	}
	updated, err := a.Library.Save(testutil.JSON(t, pr), pr.ID, saved.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Updated" || updated.Revision == saved.Revision {
		t.Fatal("update not reflected")
	}
	if err = a.Library.Delete(pr.ID, saved.Revision); !errors.Is(err, problems.ErrConflict) {
		t.Fatal("stale delete must fail")
	}
	disk := filepath.Join(a.Config.Path(a.Config.Problems.Directory), filepath.FromSlash(updated.File))
	if err = os.WriteFile(disk, append(testutil.JSON(t, pr), '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = a.Library.Save(testutil.JSON(t, pr), pr.ID, updated.Revision); !errors.Is(err, problems.ErrConflict) {
		t.Fatal("external mutation must conflict")
	}
	a.Library.Rescan()
	current, _ := a.Library.Get(pr.ID)
	if err = a.Library.Delete(pr.ID, current.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err = a.Library.Get(pr.ID); !errors.Is(err, problems.ErrNotFound) {
		t.Fatal("deleted problem remains indexed")
	}
}
func TestDiscoveryDuplicateMalformedAndSymlink(t *testing.T) {
	a := testutil.App(t)
	root := a.Config.Path(a.Config.Problems.Directory)
	if err := os.WriteFile(filepath.Join(root, "bad.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "duplicate.json"), testutil.JSON(t, testutil.Problem(t)), 0600); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	issues := strings.Join(a.Library.Rescan(), "\n")
	for _, expected := range []string{"invalid JSON", "duplicate ID", "symlinks"} {
		if !strings.Contains(issues, expected) {
			t.Fatalf("missing %q in %s", expected, issues)
		}
	}
	pr := testutil.Problem(t)
	pr.ID = "escape.new"
	if _, err := a.Library.Save(testutil.JSON(t, pr), "", ""); err == nil {
		t.Fatal("symlink directory accepted")
	}
	files, _ := os.ReadDir(outside)
	if len(files) != 0 {
		t.Fatal("escaped root")
	}
}
