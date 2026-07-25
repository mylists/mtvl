package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"mtvl/internal/auth"
	"mtvl/internal/core"
	"mtvl/internal/db"
	"mtvl/internal/modules/movies"
	"mtvl/internal/modules/tvshows"
)

func TestFullServerIntegrationFlow(t *testing.T) {
	database, err := db.OpenDB("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	if err := db.RunMigrations(database, "sqlite3", migrationFS, "migrations"); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	authProvider := auth.NewJWTAuthProvider(database, "test-secret-key")

	registry := core.NewRegistry()
	registry.Register(movies.NewModule(database))
	registry.Register(tvshows.NewModule(database))

	router := chi.NewRouter()

	// Health Endpoint
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "up"})
	})

	// Auth Endpoints
	router.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/register", func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Username string `json:"username"`
				Email    string `json:"email"`
				Password string `json:"password"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			user, err := authProvider.RegisterUser(r.Context(), req.Username, req.Email, req.Password)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(user)
		})

		r.Post("/login", func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				UsernameOrEmail string `json:"username_or_email"`
				Password        string `json:"password"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			token, user, err := authProvider.AuthenticateUser(r.Context(), req.UsernameOrEmail, req.Password)
			if err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"token": token, "user": user})
		})

		r.With(auth.Middleware(authProvider)).Get("/me", func(w http.ResponseWriter, r *http.Request) {
			user, _ := auth.GetUserFromContext(r.Context())
			_ = json.NewEncoder(w).Encode(user)
		})
	})

	registry.RegisterAllRoutes(router, auth.Middleware(authProvider))

	// 1. Healthcheck
	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("healthcheck failed: %d", rr.Code)
	}

	// 2. User Registration
	regBody := []byte(`{"username":"bob","email":"bob@example.com","password":"password123"}`)
	req = httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(regBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("registration failed: %d, body: %s", rr.Code, rr.Body.String())
	}

	// 3. User Login
	loginBody := []byte(`{"username_or_email":"bob","password":"password123"}`)
	req = httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("login failed: %d, body: %s", rr.Code, rr.Body.String())
	}

	var loginResp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &loginResp); err != nil || loginResp.Token == "" {
		t.Fatalf("failed to parse login token: %v", err)
	}

	// 4. Query Registered Categories (/api/v1/categories)
	req = httptest.NewRequest("GET", "/api/v1/categories", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("categories query failed: %d", rr.Code)
	}

	var categories []core.CategoryInfo
	_ = json.Unmarshal(rr.Body.Bytes(), &categories)
	if len(categories) != 2 {
		t.Errorf("expected 2 registered categories, got %d", len(categories))
	}

	// 5. Add Movie with Bearer Auth Token
	movieBody := []byte(`{"title":"Interstellar","release_year":2014,"director":"Christopher Nolan","status":"completed","rating":10}`)
	req = httptest.NewRequest("POST", "/api/v1/movies", bytes.NewBuffer(movieBody))
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("movie creation failed: %d, body: %s", rr.Code, rr.Body.String())
	}

	// 6. Add TV Show with Bearer Auth Token
	tvBody := []byte(`{"title":"The Wire","current_season":5,"current_episode":10,"total_episodes":60,"status":"completed","rating":10}`)
	req = httptest.NewRequest("POST", "/api/v1/tvshows", bytes.NewBuffer(tvBody))
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("TV show creation failed: %d, body: %s", rr.Code, rr.Body.String())
	}
}
