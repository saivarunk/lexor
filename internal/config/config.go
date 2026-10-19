package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Workspace struct {
		Name string `yaml:"name"`
	} `yaml:"workspace"`
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
	Problems struct {
		Directory string `yaml:"directory"`
	} `yaml:"problems"`
	Database struct {
		Path string `yaml:"path"`
	} `yaml:"database"`
	Execution struct {
		DefaultLanguage string `yaml:"default_language"`
		TimeoutSeconds  int    `yaml:"timeout_seconds"`
		MemoryMB        int    `yaml:"memory_mb"`
		MaxOutputKB     int    `yaml:"max_output_kb"`
	} `yaml:"execution"`
	Sandbox struct {
		Provider string `yaml:"provider"`
		Runtime  string `yaml:"runtime"`
		Image    string `yaml:"image"`
	} `yaml:"sandbox"`
	Intelligence struct {
		Command string `yaml:"command"`
		Enabled bool   `yaml:"enabled"`
	} `yaml:"intelligence"`
	Git struct {
		Enabled    bool   `yaml:"enabled"`
		Directory  string `yaml:"directory"`
		AutoCommit bool   `yaml:"auto_commit"`
	} `yaml:"git"`
	Root string `yaml:"-"`
}

func Defaults() Config {
	var c Config
	c.Workspace.Name = "default"
	c.Server.Host = "127.0.0.1"
	c.Server.Port = 8080
	c.Problems.Directory = "problems"
	c.Database.Path = ".lexor/lexor.db"
	c.Execution.DefaultLanguage = "python"
	c.Execution.TimeoutSeconds = 5
	c.Execution.MemoryMB = 256
	c.Execution.MaxOutputKB = 256
	c.Sandbox.Provider = "container"
	c.Sandbox.Runtime = "docker"
	c.Sandbox.Image = "python:3.12.15-alpine3.23"
	c.Intelligence.Enabled = true
	c.Intelligence.Command = "pyright-langserver"
	c.Git.Directory = ".lexor/sync"
	return c
}
func Load(root string) (Config, error) {
	c := Defaults()
	absolute, err := filepath.Abs(root)
	if err != nil {
		return c, err
	}
	c.Root = absolute
	b, err := os.ReadFile(filepath.Join(absolute, "lexor.yaml"))
	if err == nil {
		if err = yaml.Unmarshal(b, &c); err != nil {
			return c, fmt.Errorf("lexor.yaml: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return c, err
	}
	for key, target := range map[string]*string{"HOST": &c.Server.Host, "PROBLEMS_DIRECTORY": &c.Problems.Directory, "DATABASE_PATH": &c.Database.Path, "SANDBOX_RUNTIME": &c.Sandbox.Runtime, "SANDBOX_IMAGE": &c.Sandbox.Image, "PYRIGHT_COMMAND": &c.Intelligence.Command, "SYNC_DIRECTORY": &c.Git.Directory} {
		if v, ok := os.LookupEnv("LEXOR_" + key); ok {
			*target = v
		}
	}
	for key, target := range map[string]*int{"PORT": &c.Server.Port, "TIMEOUT_SECONDS": &c.Execution.TimeoutSeconds, "MEMORY_MB": &c.Execution.MemoryMB, "MAX_OUTPUT_KB": &c.Execution.MaxOutputKB} {
		if v, ok := os.LookupEnv("LEXOR_" + key); ok {
			n, e := strconv.Atoi(v)
			if e != nil {
				return c, fmt.Errorf("LEXOR_%s must be an integer", key)
			}
			*target = n
		}
	}
	if v, ok := os.LookupEnv("LEXOR_INTELLIGENCE_ENABLED"); ok {
		c.Intelligence.Enabled, err = strconv.ParseBool(v)
		if err != nil {
			return c, err
		}
	}
	if c.Intelligence.Command == "pyright-langserver" {
		local := filepath.Join(c.Root, "web", "node_modules", ".bin", "pyright-langserver")
		if _, e := os.Stat(local); e == nil {
			c.Intelligence.Command = local
		}
	}
	return c, nil
}
func (c Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server port must be between 1 and 65535")
	}
	if c.Execution.TimeoutSeconds < 1 || c.Execution.TimeoutSeconds > 60 || c.Execution.MemoryMB < 32 || c.Execution.MemoryMB > 4096 || c.Execution.MaxOutputKB < 1 || c.Execution.MaxOutputKB > 1024 {
		return fmt.Errorf("execution limits require timeout 1–60 seconds, memory 32–4096 MB, output 1–1024 KB")
	}
	if c.Sandbox.Provider != "container" {
		return fmt.Errorf("unsupported sandbox provider %q", c.Sandbox.Provider)
	}
	if strings.TrimSpace(c.Problems.Directory) == "" || strings.TrimSpace(c.Database.Path) == "" {
		return fmt.Errorf("problem directory and database path cannot be empty")
	}
	return nil
}
func (c Config) Path(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(c.Root, p)
}
func (c Config) Initialize() error {
	if err := os.MkdirAll(c.Root, 0700); err != nil {
		return err
	}
	for _, p := range []string{c.Path(c.Problems.Directory), filepath.Dir(c.Path(c.Database.Path)), c.Path(".lexor")} {
		if err := os.MkdirAll(p, 0700); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(c.Path("lexor.yaml"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	return yaml.NewEncoder(f).Encode(c)
}
