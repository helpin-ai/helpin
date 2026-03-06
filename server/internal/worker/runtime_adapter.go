package worker

import (
	"fmt"

	"github.com/d4interactive/teampulse/server/internal/model"
)

// RuntimeAdapter executes an agent run for a specific runtime kind.
type RuntimeAdapter interface {
	Kind() string
	Execute(execCtx *ExecutionContext, run *model.AgentRun) error
}

// RuntimeRegistry resolves runtime adapters by kind.
type RuntimeRegistry struct {
	adapters map[string]RuntimeAdapter
}

// NewRuntimeRegistry creates a registry for runtime adapters.
func NewRuntimeRegistry(adapters ...RuntimeAdapter) *RuntimeRegistry {
	registry := &RuntimeRegistry{adapters: make(map[string]RuntimeAdapter, len(adapters))}
	for _, adapter := range adapters {
		registry.adapters[adapter.Kind()] = adapter
	}
	return registry
}

// Get returns the runtime adapter for the given kind.
func (r *RuntimeRegistry) Get(kind string) (RuntimeAdapter, error) {
	adapter, ok := r.adapters[kind]
	if !ok {
		return nil, fmt.Errorf("runtime adapter %q is not configured", kind)
	}
	return adapter, nil
}
