package server

import "github.com/varun/lexor/internal/languages/registry"

type SessionResponse struct {
	CSRFToken string `json:"csrfToken"`
}
type HealthResponse struct {
	Status string `json:"status"`
}
type RescanResponse struct {
	Issues []string `json:"issues"`
}
type InstalledResponse struct {
	Installed []string `json:"installed"`
}
type PluginsResponse struct {
	APIVersion string          `json:"apiVersion"`
	Languages  []registry.Info `json:"languages"`
	Sandboxes  []string        `json:"sandboxes"`
	Evaluators []string        `json:"evaluators"`
	Sources    []string        `json:"sources"`
}
