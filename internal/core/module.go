package core

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// CategoryInfo contains metadata describing a tracking category plugin.
type CategoryInfo struct {
	Category    string `json:"category"`     // Unique identifier, e.g. "movies", "tv_shows"
	DisplayName string `json:"display_name"` // Human-readable name, e.g. "Movies"
	Description string `json:"description"`  // Short summary of what this module tracks
	Endpoint    string `json:"endpoint"`     // API base endpoint, e.g. "/api/v1/movies"
}

// CategoryModule is the interface that all category plugins/extensions must implement.
// Adding a new category requires creating a struct implementing this interface and registering it.
type CategoryModule interface {
	Info() CategoryInfo
	RegisterRoutes(r chi.Router, authMw func(http.Handler) http.Handler)
}
