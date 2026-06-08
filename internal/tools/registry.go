package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"minioc/internal/llm/provider"
)

type Registry struct {
	tools map[string]Spec
	order []string
}

func NewRegistry(specs ...Spec) *Registry {
	registry := &Registry{
		tools: make(map[string]Spec, len(specs)),
		order: make([]string, 0, len(specs)),
	}
	for _, spec := range specs {
		registry.tools[spec.Name] = spec
		registry.order = append(registry.order, spec.Name)
	}
	sort.Strings(registry.order)
	return registry
}

func (r *Registry) Definitions() []provider.ToolDefinition {
	definitions := make([]provider.ToolDefinition, 0, len(r.order))
	for _, name := range r.order {
		spec := r.tools[name]
		definitions = append(definitions, provider.ToolDefinition{
			Name:        spec.Name,
			Description: spec.Description,
			Parameters:  spec.Parameters,
		})
	}
	return definitions
}

func (r *Registry) Execute(ctx context.Context, name string, arguments json.RawMessage, callCtx CallContext) (Result, error) {
	spec, ok := r.tools[name]
	if !ok {
		return Result{}, fmt.Errorf("unknown tool %q", name)
	}
	return spec.Execute(ctx, callCtx, arguments)
}

func (r *Registry) IsParallelSafe(name string) bool {
	spec, ok := r.tools[name]
	return ok && spec.ParallelSafe
}

func (r *Registry) Register(spec Spec) {
	r.tools[spec.Name] = spec
	r.order = append(r.order, spec.Name)
	sort.Strings(r.order)
}

func (r *Registry) Unregister(name string) {
	delete(r.tools, name)
	for i, n := range r.order {
		if n == name {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
}

func (r *Registry) UnregisterAll(prefix string) {
	for _, name := range r.order {
		if strings.HasPrefix(name, prefix) {
			delete(r.tools, name)
		}
	}
	var keep []string
	for _, name := range r.order {
		if _, ok := r.tools[name]; ok {
			keep = append(keep, name)
		}
	}
	r.order = keep
}

func (r *Registry) ReloadTools(prefix string, newTools []Spec) {
	r.UnregisterAll(prefix)
	for _, spec := range newTools {
		r.Register(spec)
	}
}
