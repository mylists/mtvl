package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

type mockModule struct {
	info CategoryInfo
}

func (m *mockModule) Info() CategoryInfo {
	return m.info
}

func (m *mockModule) RegisterRoutes(r chi.Router, authMw func(http.Handler) http.Handler) {
	r.Get(m.info.Endpoint, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("mock list"))
	})
}

func TestRegistryRegistrationAndRoutes(t *testing.T) {
	reg := NewRegistry()

	mod1 := &mockModule{info: CategoryInfo{Category: "movies", DisplayName: "Movies", Description: "Track movies", Endpoint: "/api/v1/movies"}}
	mod2 := &mockModule{info: CategoryInfo{Category: "books", DisplayName: "Books", Description: "Track books", Endpoint: "/api/v1/books"}}

	reg.Register(mod1)
	reg.Register(mod2)

	modules := reg.GetModules()
	if len(modules) != 2 {
		t.Fatalf("expected 2 registered modules, got %d", len(modules))
	}

	router := chi.NewRouter()
	noAuthMw := func(next http.Handler) http.Handler { return next }
	reg.RegisterAllRoutes(router, noAuthMw)

	// Test GET /api/v1/categories
	req := httptest.NewRequest("GET", "/api/v1/categories", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK from /api/v1/categories, got %d", rr.Code)
	}

	var infos []CategoryInfo
	if err := json.Unmarshal(rr.Body.Bytes(), &infos); err != nil {
		t.Fatalf("failed to parse categories response: %v", err)
	}
	if len(infos) != 2 {
		t.Errorf("expected 2 categories in JSON, got %d", len(infos))
	}

	// Test GET /api/v1/movies
	req = httptest.NewRequest("GET", "/api/v1/movies", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || rr.Body.String() != "mock list" {
		t.Errorf("expected mock list response from /api/v1/movies, got %d / %s", rr.Code, rr.Body.String())
	}
}
