package registry

import (
	"github.com/varun/lexor/internal/languages/python"
	"testing"
)

func TestRegistryRejectsDuplicateAndUnknown(t *testing.T) {
	r := New()
	if err := r.Register(python.New(1024)); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(python.New(1024)); err == nil {
		t.Fatal("duplicate language accepted")
	}
	if _, err := r.Resolve("rust"); err == nil {
		t.Fatal("unknown runner accepted")
	}
	if _, err := r.Resolve("python"); err != nil {
		t.Fatal(err)
	}
}
