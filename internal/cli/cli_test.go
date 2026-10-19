package cli

import (
	"bytes"
	"context"
	"encoding/json"
	p "github.com/varun/lexor/pkg/pluginapi"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func invoke(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"--workspace", root}, args...))
	err := cmd.ExecuteContext(context.Background())
	return out.String(), err
}
func TestCLIInitExportImportAndValidation(t *testing.T) {
	root := t.TempDir()
	if _, err := invoke(t, root, "init"); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "problems/arrays/two-sum.json")
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var pr p.Problem
	json.Unmarshal(raw, &pr)
	pr.Title = "My original title"
	raw, _ = json.Marshal(pr)
	os.WriteFile(file, raw, 0600)
	if _, err = invoke(t, root, "init"); err != nil {
		t.Fatal(err)
	}
	out, err := invoke(t, root, "problem", "list")
	if err != nil || !strings.Contains(out, pr.Title) {
		t.Fatal("init overwrote user data", out, err)
	}
	out, err = invoke(t, root, "problem", "export", pr.ID)
	if err != nil || !json.Valid([]byte(out)) || strings.Contains(out, "revision") {
		t.Fatal("export is not a portable problem", out, err)
	}
	source := filepath.Join(t.TempDir(), "problem.json")
	os.WriteFile(source, []byte(out), 0600)
	destination := t.TempDir()
	out, err = invoke(t, destination, "problem", "import", source)
	if err != nil || !strings.Contains(out, "Preview only") {
		t.Fatal(out, err)
	}
	out, err = invoke(t, destination, "problem", "list")
	if err != nil || out != "" {
		t.Fatal("preview installed files")
	}
	if _, err = invoke(t, destination, "problem", "import", source, "--apply"); err != nil {
		t.Fatal(err)
	}
	out, err = invoke(t, destination, "problem", "validate")
	if err != nil || !strings.Contains(out, "Validated 1 problems") {
		t.Fatal(out, err)
	}
	if _, err = invoke(t, destination, "problem", "import", source, "--apply"); err == nil {
		t.Fatal("collision accepted")
	}
	out, err = invoke(t, destination, "stats")
	if err != nil || !strings.Contains(out, `"total": 1`) {
		t.Fatal(out, err)
	}
	out, err = invoke(t, destination, "plugin", "list")
	if err != nil || !strings.Contains(out, "python") {
		t.Fatal(out, err)
	}
}
