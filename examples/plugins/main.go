// A compile-time extension that reuses Python preparation and decoding while
// adding instrumentation. Run with go run ./examples/plugins.
package main

import (
	"context"
	"fmt"
	"github.com/varun/lexor/internal/languages/python"
	"github.com/varun/lexor/internal/languages/registry"
	p "github.com/varun/lexor/pkg/pluginapi"
	"log/slog"
)

type tracingPython struct{ *python.Plugin }

func (tracingPython) ID() string          { return "python-trace" }
func (tracingPython) DisplayName() string { return "Python (traced preparation)" }
func (l tracingPython) Prepare(ctx context.Context, request p.PrepareRequest) (p.Artifact, error) {
	slog.Info("preparing solution", "language", l.ID(), "test", request.Test.Name)
	return l.Plugin.Prepare(ctx, request)
}

var _ p.LanguagePlugin = tracingPython{}

func main() {
	r := registry.New()
	if err := r.Register(tracingPython{python.New(64 << 10)}); err != nil {
		panic(err)
	}
	for _, info := range r.List() {
		fmt.Printf("%s: %s\n", info.ID, info.Name)
	}
}
