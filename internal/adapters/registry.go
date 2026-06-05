package adapters

import (
	"sort"
	"sync"
)

var defaultRegistry = newRegistry()

type Registry struct {
	mu       sync.RWMutex
	adapters map[string]Adapter
}

func newRegistry() *Registry {
	return &Registry{adapters: map[string]Adapter{}}
}

func Register(adapter Adapter) {
	defaultRegistry.Register(adapter)
}

func Get(name string) (Adapter, bool) {
	return defaultRegistry.Get(name)
}

func List() []Adapter {
	return defaultRegistry.List()
}

func (r *Registry) Register(adapter Adapter) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := adapter.Name()
	if _, exists := r.adapters[name]; exists {
		panic("adapter already registered: " + name)
	}
	r.adapters[name] = adapter
}

func (r *Registry) Get(name string) (Adapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	adapter, ok := r.adapters[name]
	return adapter, ok
}

func (r *Registry) List() []Adapter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.adapters))
	for name := range r.adapters {
		names = append(names, name)
	}
	sort.Strings(names)

	adapters := make([]Adapter, 0, len(names))
	for _, name := range names {
		adapters = append(adapters, r.adapters[name])
	}
	return adapters
}
