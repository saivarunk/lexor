package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrecedenceAndIdempotentInitialization(t *testing.T) {
	root := t.TempDir()
	c := Defaults()
	c.Root = root
	c.Server.Port = 9000
	if err := c.Initialize(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LEXOR_PORT", "9001")
	loaded, err := Load(root)
	if err != nil || loaded.Server.Port != 9001 {
		t.Fatalf("env must override file: %v", err)
	}
	if err = loaded.Initialize(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "lexor.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) == "" {
		t.Fatal("config erased")
	}
	t.Setenv("LEXOR_PORT", "invalid")
	if _, err = Load(root); err == nil {
		t.Fatal("bad env accepted")
	}
}
