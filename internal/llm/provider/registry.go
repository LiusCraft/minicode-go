package provider

type Registry struct {
	adapters map[string]Adapter
}

func NewRegistry() *Registry {
	return &Registry{adapters: make(map[string]Adapter)}
}

func (r *Registry) Register(key string, adapter Adapter) {
	r.adapters[key] = adapter
}

func (r *Registry) Get(key string) (Adapter, bool) {
	adapter, ok := r.adapters[key]
	return adapter, ok
}
