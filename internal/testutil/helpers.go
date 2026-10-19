package testutil

import (
	"encoding/json"
	"github.com/varun/lexor/internal/app"
	"github.com/varun/lexor/internal/config"
	p "github.com/varun/lexor/pkg/pluginapi"
	seeds "github.com/varun/lexor/problems"
	"testing"
)

func App(t *testing.T) *app.App {
	t.Helper()
	c := config.Defaults()
	c.Root = t.TempDir()
	c.Intelligence.Enabled = false
	a, err := app.Open(c)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	if err = a.Seed(); err != nil {
		t.Fatal(err)
	}
	return a
}
func Problem(t *testing.T) p.Problem {
	t.Helper()
	b, err := seeds.Files.ReadFile("arrays/two-sum.json")
	if err != nil {
		t.Fatal(err)
	}
	var pr p.Problem
	if err = json.Unmarshal(b, &pr); err != nil {
		t.Fatal(err)
	}
	return pr
}
func JSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
