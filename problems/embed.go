// Package seeds holds the original starter problem library.
package seeds

import "embed"

//go:embed */*.json
var Files embed.FS
