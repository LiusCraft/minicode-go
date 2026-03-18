package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

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
