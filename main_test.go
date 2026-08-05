package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"mtvl/internal/auth"
	"mtvl/internal/core"
	"mtvl/internal/docs"
	"mtvl/internal/modules/books"
	"mtvl/internal/modules/movies"
	"mtvl/internal/modules/tvshows"
	"mtvl/internal/services"
)

func TestFullServerIntegrationFlow(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock database: %v", err)
	}
	defer database.Close()
	_ = mock

	authProvider := auth.NewJWTAuthProvider(database, "test-secret-key")

	registry := core.NewRegistry()
	registry.Register(movies.NewModule(database))
	registry.Register(tvshows.NewModule(database))
	registry.Register(books.NewModule(database))

	router := chi.NewRouter()
	router.Use(corsMiddleware("*"))
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "up"})
	})

	registry.RegisterAllRoutes(router, auth.Middleware(authProvider))
	services.NewServiceHandler(database).RegisterRoutes(router, auth.Middleware(authProvider))
	docs.NewDocsHandler().RegisterRoutes(router)

	// 1. CORS Preflight
	req := httptest.NewRequest("OPTIONS", "/api/v1/movies", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("CORS preflight failed: %d", rr.Code)
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("expected CORS origin header, got %s", rr.Header().Get("Access-Control-Allow-Origin"))
	}

	// 2. Healthcheck
	req = httptest.NewRequest("GET", "/health", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("healthcheck failed: %d", rr.Code)
	}

	// 3. Category discovery
	req = httptest.NewRequest("GET", "/api/v1/categories", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("categories query failed: %d", rr.Code)
	}
	var categories []core.CategoryInfo
	_ = json.Unmarshal(rr.Body.Bytes(), &categories)
	if len(categories) != 3 {
		t.Errorf("expected 3 registered categories, got %d", len(categories))
	}

	// 4. OpenAPI Spec & Docs
	req = httptest.NewRequest("GET", "/api/v1/openapi.json", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("openapi.json failed: %d", rr.Code)
	}

	req = httptest.NewRequest("GET", "/api/v1/docs", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("docs UI failed: %d", rr.Code)
	}
}
