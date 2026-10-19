// Package pluginapi defines Lexor's compile-time extension contracts (API version 1).
package pluginapi

import (
	"context"
	"encoding/json"
	"time"
)

const Version = "1.0.0"

type Parameter struct {
	Name string `json:"name"`
	Type string `json:"type"`
}
type Entrypoint struct {
	Type       string      `json:"type"`
	Name       string      `json:"name"`
	Parameters []Parameter `json:"parameters"`
	Returns    string      `json:"returns"`
}
type LanguageSpec struct {
	Entrypoint  Entrypoint `json:"entrypoint"`
	StarterCode string     `json:"starterCode"`
}
type Organization struct {
	Topic     string   `json:"topic"`
	Path      []string `json:"path"`
	Tags      []string `json:"tags"`
	Companies []string `json:"companies"`
}
type Example struct {
	Input       map[string]any `json:"input"`
	Output      any            `json:"output"`
	Explanation string         `json:"explanation"`
}
type Comparison struct {
	Mode              string  `json:"mode"`
	AbsoluteTolerance float64 `json:"absoluteTolerance,omitempty"`
	RelativeTolerance float64 `json:"relativeTolerance,omitempty"`
}
type TestCase struct {
	Name       string         `json:"name"`
	Input      map[string]any `json:"input"`
	Expected   any            `json:"expected"`
	Comparison *Comparison    `json:"comparison,omitempty"`
}
type Tests struct {
	Public []TestCase `json:"public"`
	Hidden []TestCase `json:"hidden"`
}
type Metadata struct {
	Source  string `json:"source"`
	Author  string `json:"author"`
	Version string `json:"version"`
}
type Problem struct {
	SchemaVersion string                  `json:"schemaVersion"`
	ID            string                  `json:"id"`
	Slug          string                  `json:"slug"`
	Title         string                  `json:"title"`
	Description   string                  `json:"description"`
	Difficulty    string                  `json:"difficulty"`
	Organization  Organization            `json:"organization"`
	Constraints   []string                `json:"constraints"`
	Examples      []Example               `json:"examples"`
	Languages     map[string]LanguageSpec `json:"languages"`
	Tests         Tests                   `json:"tests"`
	Comparison    *Comparison             `json:"comparison,omitempty"`
	Metadata      Metadata                `json:"metadata"`
}
type ProblemView struct {
	Problem
	Revision        string     `json:"revision"`
	File            string     `json:"file"`
	Status          string     `json:"status"`
	LastAttemptedAt *time.Time `json:"lastAttemptedAt,omitempty"`
}
type Limits struct {
	Timeout     time.Duration
	MemoryMB    int
	OutputBytes int
}
type Artifact struct {
	Files   map[string][]byte
	Command []string
}
type PrepareRequest struct {
	Source string
	Spec   LanguageSpec
	Test   TestCase
}
type SandboxResult struct {
	Output         []byte
	Stderr         string
	Duration       time.Duration
	MemoryBytes    int64
	ExitCode       int
	TimedOut       bool
	OutputExceeded bool
	OOMKilled      bool
}
type LanguagePlugin interface {
	ID() string
	DisplayName() string
	Extensions() []string
	Prepare(context.Context, PrepareRequest) (Artifact, error)
	Decode(SandboxResult) (actual any, stdout string, stderr string, err error)
}
type SandboxProvider interface {
	ID() string
	Available(context.Context) error
	Execute(context.Context, Artifact, Limits) (SandboxResult, error)
}
type Evaluator interface {
	Evaluate(context.Context, any, any, Comparison) bool
}
type ExecutionRequest struct {
	ProblemID string `json:"problemId"`
	Language  string `json:"language"`
	Source    string `json:"source"`
}
type TestResult struct {
	Name       string          `json:"name"`
	Status     string          `json:"status"`
	Hidden     bool            `json:"hidden"`
	Actual     json.RawMessage `json:"actual,omitempty"`
	Expected   json.RawMessage `json:"expected,omitempty"`
	DurationMS float64         `json:"durationMs"`
	Error      string          `json:"error,omitempty"`
}
type ExecutionResult struct {
	ID          string       `json:"id"`
	ProblemID   string       `json:"problemId"`
	Language    string       `json:"language"`
	Source      string       `json:"source,omitempty"`
	Mode        string       `json:"mode"`
	Status      string       `json:"status"`
	Passed      int          `json:"passed"`
	Total       int          `json:"total"`
	DurationMS  float64      `json:"durationMs"`
	MemoryBytes int64        `json:"memoryBytes"`
	Stdout      string       `json:"stdout"`
	Stderr      string       `json:"stderr"`
	Results     []TestResult `json:"results"`
	CreatedAt   time.Time    `json:"createdAt"`
}
type Progress struct {
	ProblemID        string     `json:"problemId"`
	Status           string     `json:"status"`
	Attempts         int        `json:"attempts"`
	BestDurationMS   float64    `json:"bestDurationMs"`
	FirstAttemptedAt time.Time  `json:"firstAttemptedAt"`
	LastAttemptedAt  time.Time  `json:"lastAttemptedAt"`
	FirstSolvedAt    *time.Time `json:"firstSolvedAt,omitempty"`
	LastSolvedAt     *time.Time `json:"lastSolvedAt,omitempty"`
}
type Draft struct {
	ProblemID string     `json:"problemId"`
	Language  string     `json:"language"`
	Source    string     `json:"source"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
}
type Activity struct {
	Date      string `json:"date"`
	Count     int    `json:"count"`
	NewSolves int    `json:"newSolves"`
}
type Stats struct {
	Total          int               `json:"total"`
	Solved         int               `json:"solved"`
	Attempted      int               `json:"attempted"`
	CurrentStreak  int               `json:"currentStreak"`
	LongestStreak  int               `json:"longestStreak"`
	SolvedThisWeek int               `json:"solvedThisWeek"`
	Difficulty     map[string]int    `json:"difficulty"`
	Activity       []Activity        `json:"activity"`
	Recent         []ExecutionResult `json:"recent"`
}

type RunnerError interface {
	error
	ExecutionStatus() string
}

// MeasuredRunner optionally reports language-process peak resident memory.
type MeasuredRunner interface{ PeakMemory(SandboxResult) int64 }

type Preferences struct {
	Theme          string `json:"theme"`
	EditorTheme    string `json:"editorTheme"`
	EditorFontSize int    `json:"editorFontSize"`
}
