package core

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
)

// Registry manages all registered CategoryModule plugins.
type Registry struct {
	mu      sync.RWMutex
	modules map[string]CategoryModule
}

// NewRegistry initializes an empty plugin Registry.
func NewRegistry() *Registry {
	return &Registry{
		modules: make(map[string]CategoryModule),
	}
}

// Register registers a CategoryModule with the central registry.
func (r *Registry) Register(m CategoryModule) {
	r.mu.Lock()
	defer r.mu.Unlock()

	info := m.Info()
	r.modules[info.Category] = m
}

// GetModules returns metadata for all registered CategoryModules.
func (r *Registry) GetModules() []CategoryInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]CategoryInfo, 0, len(r.modules))
	for _, m := range r.modules {
		list = append(list, m.Info())
	}
	return list
}

// RegisterAllRoutes attaches category routes and discovery endpoint to the router.
func (r *Registry) RegisterAllRoutes(router chi.Router, authMw func(http.Handler) http.Handler) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// GET /api/v1/categories - Returns metadata of all active modules
	router.Get("/api/v1/categories", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(r.GetModules())
	})

	// Register sub-routes for each category module
	for _, m := range r.modules {
		m.RegisterRoutes(router, authMw)
	}
}
