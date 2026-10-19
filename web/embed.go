package web

import "embed"

// Assets contains the frontend build. Run bun run build in web before go build.
//
//go:embed all:dist
var Assets embed.FS

// Fallback explains how to build assets in an unbuilt source checkout.
//
//go:embed fallback.html
var Fallback []byte
