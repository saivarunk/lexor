package registry

import (
	"fmt"
	p "github.com/varun/lexor/pkg/pluginapi"
	"sort"
)

type Registry struct{ languages map[string]p.LanguagePlugin }

func New() *Registry { return &Registry{languages: map[string]p.LanguagePlugin{}} }
func (r *Registry) Register(l p.LanguagePlugin) error {
	if l == nil || l.ID() == "" {
		return fmt.Errorf("language ID required")
	}
	if _, ok := r.languages[l.ID()]; ok {
		return fmt.Errorf("language %s already registered", l.ID())
	}
	r.languages[l.ID()] = l
	return nil
}
func (r *Registry) Resolve(id string) (p.LanguagePlugin, error) {
	l, ok := r.languages[id]
	if !ok {
		return nil, fmt.Errorf("unsupported language %q", id)
	}
	return l, nil
}

type Info struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Extensions []string `json:"extensions"`
}

func (r *Registry) List() []Info {
	out := []Info{}
	for _, l := range r.languages {
		out = append(out, Info{l.ID(), l.DisplayName(), l.Extensions()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
