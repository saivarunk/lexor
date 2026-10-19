package main

import (
	"github.com/varun/lexor/internal/cli"
	"log/slog"
	"os"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	if err := cli.New().Execute(); err != nil {
		slog.Error("lexor", "error", err)
		os.Exit(1)
	}
}
