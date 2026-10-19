package execution_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/varun/lexor/internal/execution"
	"github.com/varun/lexor/internal/testutil"
	p "github.com/varun/lexor/pkg/pluginapi"
	"testing"
)

type fakeBox struct {
	calls       int
	unavailable bool
}

func (*fakeBox) ID() string { return "fake" }
func (f *fakeBox) Available(context.Context) error {
	if f.unavailable {
		return fmt.Errorf("missing runtime")
	}
	return nil
}
func (f *fakeBox) Execute(ctx context.Context, a p.Artifact, l p.Limits) (p.SandboxResult, error) {
	f.calls++
	var req struct {
		Input map[string]any `json:"input"`
	}
	json.Unmarshal(a.Files["request.json"], &req)
	var result []int
	if f.calls == 2 {
		result = []int{1, 2}
	} else {
		result = []int{0, 1}
	}
	b, _ := json.Marshal(map[string]any{"actual": result, "stdout": "test stdout", "stderr": ""})
	return p.SandboxResult{Output: b}, nil
}
func TestOrchestrationAndHiddenRedaction(t *testing.T) {
	a := testutil.App(t)
	box := &fakeBox{}
	e := execution.New(a.Library, a.Languages, box, a.Store, a.Engine.Limits)
	r, err := e.Execute(context.Background(), p.ExecutionRequest{ProblemID: "arrays.two-sum", Language: "python", Source: "fake"}, "submit")
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "Wrong Answer" || box.calls != 4 || r.Passed != 3 {
		t.Fatalf("unexpected result: %+v", r)
	}
	for _, v := range r.Results {
		if v.Hidden && (v.Actual != nil || v.Expected != nil || v.Error != "") {
			t.Fatal("hidden output leaked")
		}
	}
	if r.Stdout != "test stdouttest stdout" {
		t.Fatal("hidden console output leaked")
	}
	stored, err := a.Store.Execution(context.Background(), r.ID)
	if err != nil || stored.Source != "fake" {
		t.Fatal("execution not persisted")
	}
}
func TestUnavailableFailsClosed(t *testing.T) {
	a := testutil.App(t)
	box := &fakeBox{unavailable: true}
	e := execution.New(a.Library, a.Languages, box, a.Store, a.Engine.Limits)
	r, err := e.Execute(context.Background(), p.ExecutionRequest{ProblemID: "arrays.two-sum", Language: "python", Source: "anything"}, "submit")
	if err != nil || r.Status != "Sandbox Unavailable" || box.calls != 0 {
		t.Fatalf("unrestricted fallback: %+v %v", r, err)
	}
	stats, err := a.Store.Stats(context.Background(), a.Library.List())
	if err != nil || stats.Attempted != 0 {
		t.Fatal("unavailable runtime should not count as attempt")
	}
}
