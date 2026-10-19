package python

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	p "github.com/varun/lexor/pkg/pluginapi"
)

//go:embed harness.py
var harness []byte

//go:embed supervisor.py
var supervisor []byte

type Plugin struct{ OutputLimit int }

func New(outputLimit int) *Plugin    { return &Plugin{OutputLimit: outputLimit} }
func (*Plugin) ID() string           { return "python" }
func (*Plugin) DisplayName() string  { return "Python 3.12" }
func (*Plugin) Extensions() []string { return []string{".py"} }
func (l *Plugin) Prepare(ctx context.Context, r p.PrepareRequest) (p.Artifact, error) {
	if err := ctx.Err(); err != nil {
		return p.Artifact{}, err
	}
	b, err := json.Marshal(struct {
		Entrypoint  p.Entrypoint   `json:"entrypoint"`
		Input       map[string]any `json:"input"`
		OutputLimit int            `json:"outputLimit"`
	}{r.Spec.Entrypoint, r.Test.Input, l.OutputLimit})
	return p.Artifact{Files: map[string][]byte{"solution.py": []byte(r.Source), "harness.py": harness, "supervisor.py": supervisor, "request.json": b}, Command: []string{"python", "-I", "-B", "/work/supervisor.py"}}, err
}

type ResultError struct{ Kind, Message string }

func (e *ResultError) Error() string { return e.Message }
func (*Plugin) Decode(r p.SandboxResult) (any, string, string, error) {
	var value struct {
		Actual any    `json:"actual"`
		Stdout string `json:"stdout"`
		Stderr string `json:"stderr"`
		Error  string `json:"error"`
		Kind   string `json:"kind"`
	}
	decoder := json.NewDecoder(bytes.NewReader(r.Output))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, "", r.Stderr, fmt.Errorf("invalid runner response: %w", err)
	}
	if value.Error != "" {
		return nil, value.Stdout, value.Stderr, &ResultError{value.Kind, value.Error}
	}
	return value.Actual, value.Stdout, value.Stderr, nil
}

func (e *ResultError) ExecutionStatus() string { return e.Kind }

func (*Plugin) PeakMemory(r p.SandboxResult) int64 {
	var v struct {
		MemoryBytes int64 `json:"memoryBytes"`
	}
	if json.Unmarshal(r.Output, &v) != nil {
		return 0
	}
	return v.MemoryBytes
}
