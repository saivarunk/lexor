package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/varun/lexor/internal/config"
	"github.com/varun/lexor/internal/execution"
	"github.com/varun/lexor/internal/execution/sandbox"
	"github.com/varun/lexor/internal/languages/python"
	"github.com/varun/lexor/internal/languages/registry"
	"github.com/varun/lexor/internal/persistence"
	"github.com/varun/lexor/internal/problems"
	p "github.com/varun/lexor/pkg/pluginapi"
	seeds "github.com/varun/lexor/problems"
)

type App struct {
	Config    config.Config
	Library   *problems.Library
	Store     *persistence.Store
	Languages *registry.Registry
	Sandbox   *sandbox.Container
	Engine    *execution.Engine
}

func Open(c config.Config) (*App, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if err := c.Initialize(); err != nil {
		return nil, err
	}
	lib, err := problems.New(c.Path(c.Problems.Directory))
	if err != nil {
		return nil, err
	}
	store, err := persistence.Open(c.Path(c.Database.Path))
	if err != nil {
		lib.Close()
		return nil, err
	}
	langs := registry.New()
	if err = langs.Register(python.New(c.Execution.MaxOutputKB * 1024 / 4)); err != nil {
		lib.Close()
		store.Close()
		return nil, err
	}
	box := &sandbox.Container{Runtime: c.Sandbox.Runtime, Image: c.Sandbox.Image}
	engine := execution.New(lib, langs, box, store, p.Limits{Timeout: time.Duration(c.Execution.TimeoutSeconds) * time.Second, MemoryMB: c.Execution.MemoryMB, OutputBytes: c.Execution.MaxOutputKB * 1024})
	return &App{c, lib, store, langs, box, engine}, nil
}
func (a *App) Close() { a.Library.Close(); a.Store.Close() }
func (a *App) Seed() error {
	return fs.WalkDir(seeds.Files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := seeds.Files.ReadFile(path)
		if err != nil {
			return err
		}
		var pr p.Problem
		if err = json.Unmarshal(b, &pr); err != nil {
			return err
		}
		if _, err = a.Library.Get(pr.ID); err == nil {
			return nil
		}
		_, err = a.Library.Save(b, "", "")
		return err
	})
}
func (a *App) Problems(ctx context.Context) ([]p.ProblemView, error) {
	items := a.Library.List()
	progress, err := a.Store.Progress(ctx)
	if err != nil {
		return nil, err
	}
	index := map[string]p.Progress{}
	for _, v := range progress {
		index[v.ProblemID] = v
	}
	for i := range items {
		if v, ok := index[items[i].ID]; ok {
			items[i].Status = v.Status
			items[i].LastAttemptedAt = &v.LastAttemptedAt
		}
	}
	return items, nil
}

type Diagnostic struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}
type SystemStatus struct {
	Workspace           string       `json:"workspace"`
	Diagnostics         []Diagnostic `json:"diagnostics"`
	ProblemCount        int          `json:"problemCount"`
	Issues              []string     `json:"issues"`
	Image               string       `json:"image"`
	Timezone            string       `json:"timezone"`
	IntelligenceEnabled bool         `json:"intelligenceEnabled"`
	SyncDirectory       string       `json:"syncDirectory"`
}

func (a *App) Status(ctx context.Context) SystemStatus {
	result := SystemStatus{Workspace: a.Config.Root, ProblemCount: len(a.Library.List()), Issues: a.Library.Diagnostics(), Image: a.Config.Sandbox.Image, Timezone: a.Store.Location.String(), IntelligenceEnabled: a.Config.Intelligence.Enabled, SyncDirectory: a.Config.Path(a.Config.Git.Directory)}
	add := func(name string, err error, success string) {
		d := Diagnostic{Name: name, OK: err == nil, Message: success}
		if err != nil {
			d.Message = err.Error()
		}
		result.Diagnostics = append(result.Diagnostics, d)
	}
	add("Workspace", a.Config.Validate(), "Configuration valid; directories initialized")
	add("SQLite", a.Store.DB.PingContext(ctx), "Connected; WAL and migrations ready")
	var schemaErr error
	if len(result.Issues) > 0 {
		schemaErr = fmt.Errorf("%d invalid problem files; use lexor problem validate", len(result.Issues))
	}
	add("Problem schema", schemaErr, "All problem files are valid")
	add("Container sandbox", a.Sandbox.Available(ctx), "Runtime ready; network disabled for execution")
	if _, err := exec.LookPath(a.Config.Intelligence.Command); err != nil {
		add("Pyright", fmt.Errorf("optional: install Node.js and npm install -g pyright (or set LEXOR_PYRIGHT_COMMAND)"), "")
	} else {
		add("Pyright", nil, "Language server installed; optional semantic intelligence ready")
	}
	_, err := exec.LookPath("git")
	add("Git", err, "Git available")
	return result
}
func WorkspaceExists(root string) bool {
	_, err := os.Stat(filepath.Join(root, "lexor.yaml"))
	return err == nil
}
