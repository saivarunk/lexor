package execution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/varun/lexor/internal/languages/registry"
	p "github.com/varun/lexor/pkg/pluginapi"
)

type Problems interface {
	Get(string) (p.ProblemView, error)
}
type Recorder interface {
	Record(context.Context, p.ExecutionResult) error
}
type Engine struct {
	Problems  Problems
	Languages *registry.Registry
	Sandbox   p.SandboxProvider
	Evaluator p.Evaluator
	Store     Recorder
	Limits    p.Limits
	sem       chan struct{}
}

func New(pr Problems, langs *registry.Registry, sandbox p.SandboxProvider, store Recorder, limits p.Limits) *Engine {
	return &Engine{Problems: pr, Languages: langs, Sandbox: sandbox, Evaluator: JSONEvaluator{}, Store: store, Limits: limits, sem: make(chan struct{}, 1)}
}
func (e *Engine) Execute(ctx context.Context, req p.ExecutionRequest, mode string) (p.ExecutionResult, error) {
	r := p.ExecutionResult{ID: uuid.NewString(), ProblemID: req.ProblemID, Language: req.Language, Source: req.Source, Mode: mode, Status: "Accepted", CreatedAt: time.Now().UTC(), Results: []p.TestResult{}}
	if len(req.Source) == 0 || len(req.Source) > 128<<10 {
		return r, fmt.Errorf("source must contain 1–131072 bytes")
	}
	if mode != "run" && mode != "submit" {
		return r, fmt.Errorf("invalid execution mode")
	}
	pr, err := e.Problems.Get(req.ProblemID)
	if err != nil {
		return r, err
	}
	plugin, err := e.Languages.Resolve(req.Language)
	if err != nil {
		return r, err
	}
	spec, ok := pr.Languages[req.Language]
	if !ok {
		return r, fmt.Errorf("problem does not support %s", req.Language)
	}
	select {
	case e.sem <- struct{}{}:
		defer func() { <-e.sem }()
	default:
		return r, fmt.Errorf("execution busy; wait for the current job")
	}
	tests := append([]p.TestCase{}, pr.Tests.Public...)
	if mode == "submit" {
		tests = append(tests, pr.Tests.Hidden...)
	}
	r.Total = len(tests)
	if err = e.Sandbox.Available(ctx); err != nil {
		r.Status = "Sandbox Unavailable"
		r.Stderr = err.Error()
		return r, e.record(ctx, r)
	}
	job, cancel := context.WithTimeout(ctx, e.Limits.Timeout)
	defer cancel()
	start := time.Now()
	remainingOutput := e.Limits.OutputBytes
	for i, tc := range tests {
		tr := p.TestResult{Name: tc.Name, Status: "Accepted", Hidden: i >= len(pr.Tests.Public)}
		artifact, prepErr := plugin.Prepare(job, p.PrepareRequest{Source: req.Source, Spec: spec, Test: tc})
		if prepErr != nil {
			r.Status = "Internal Error"
			r.Stderr = prepErr.Error()
			break
		}
		limits := e.Limits
		limits.Timeout = time.Until(start.Add(e.Limits.Timeout))
		limits.OutputBytes = remainingOutput
		raw, runErr := e.Sandbox.Execute(job, artifact, limits)
		tr.DurationMS = float64(raw.Duration.Microseconds()) / 1000
		remainingOutput -= len(raw.Output) + len(raw.Stderr)
		if raw.MemoryBytes > r.MemoryBytes {
			r.MemoryBytes = raw.MemoryBytes
		}
		switch {
		case raw.TimedOut || errors.Is(job.Err(), context.DeadlineExceeded):
			tr.Status = "Time Limit Exceeded"
		case raw.OOMKilled:
			tr.Status = "Memory Limit Exceeded"
		case raw.OutputExceeded || remainingOutput < 0:
			tr.Status = "Output Limit Exceeded"
		case runErr != nil:
			tr.Status = "Internal Error"
			tr.Error = runErr.Error()
		case raw.ExitCode != 0:
			tr.Status = "Runtime Error"
			tr.Error = raw.Stderr
		default:
			actual, stdout, stderr, decodeErr := plugin.Decode(raw)
			if measured, ok := plugin.(p.MeasuredRunner); ok {
				if peak := measured.PeakMemory(raw); peak > r.MemoryBytes {
					r.MemoryBytes = peak
				}
			}
			if !tr.Hidden {
				r.Stdout += stdout
				r.Stderr += stderr
				tr.Actual, _ = json.Marshal(actual)
				tr.Expected, _ = json.Marshal(tc.Expected)
			}
			if decodeErr != nil {
				tr.Status = "Runtime Error"
				var resultErr p.RunnerError
				if errors.As(decodeErr, &resultErr) {
					tr.Status = resultErr.ExecutionStatus()
				}
				tr.Error = decodeErr.Error()
			} else {
				options := p.Comparison{Mode: "exact"}
				if pr.Comparison != nil {
					options = *pr.Comparison
				}
				if tc.Comparison != nil {
					options = *tc.Comparison
				}
				if !e.Evaluator.Evaluate(job, actual, tc.Expected, options) {
					tr.Status = "Wrong Answer"
				}
			}
		}
		if tr.Hidden {
			tr.Name = fmt.Sprintf("Hidden test %d", i-len(pr.Tests.Public)+1)
			tr.Error = ""
		}
		r.Results = append(r.Results, tr)
		if tr.Status == "Accepted" {
			r.Passed++
		} else {
			if r.Status == "Accepted" {
				r.Status = tr.Status
			}
			if tr.Status != "Wrong Answer" {
				break
			}
		}
	}
	r.DurationMS = float64(time.Since(start).Microseconds()) / 1000
	if err = e.record(ctx, r); err != nil {
		return r, err
	}
	slog.Info("execution complete", "id", r.ID, "problem", r.ProblemID, "mode", mode, "status", r.Status, "duration_ms", r.DurationMS)
	return r, nil
}
func (e *Engine) record(ctx context.Context, r p.ExecutionResult) error {
	if err := e.Store.Record(ctx, r); err != nil {
		return fmt.Errorf("persist execution: %w", err)
	}
	return nil
}
