package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"mtvl/internal/auth"
	"mtvl/internal/core"
	"mtvl/internal/docs"
	"mtvl/internal/health"
	"mtvl/internal/modules/books"
	"mtvl/internal/modules/movies"
	"mtvl/internal/modules/tvshows"
	"mtvl/internal/services"
)

func TestFullServerIntegrationFlow(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}

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

	health.NewHandler(database).RegisterRoutes(router)

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

func TestAPITokenEndpointsIntegration(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}

	if err := database.AutoMigrate(&auth.UserModel{}, &auth.APITokenModel{}, &movies.Movie{}, &movies.UserMovie{}); err != nil {
		t.Fatalf("failed to auto migrate: %v", err)
	}

	authProvider := auth.NewJWTAuthProvider(database, "test-secret-key")
	user, err := authProvider.RegisterUser(context.Background(), "tokenuser", "tokenuser@example.com", "password")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	jwtToken, _, err := authProvider.AuthenticateUser(context.Background(), "tokenuser", "password")
	if err != nil {
		t.Fatalf("failed to authenticate: %v", err)
	}

	registry := core.NewRegistry()
	registry.Register(movies.NewModule(database))

	router := chi.NewRouter()
	router.Route("/api/v1/auth", func(r chi.Router) {
		r.Group(func(sub chi.Router) {
			sub.Use(auth.Middleware(authProvider))

			sub.Get("/me", func(w http.ResponseWriter, r *http.Request) {
				u, _ := auth.GetUserFromContext(r.Context())
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(u)
			})

			sub.Post("/tokens", func(w http.ResponseWriter, r *http.Request) {
				u, ok := auth.GetUserFromContext(r.Context())
				if !ok {
					http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
					return
				}
				var req struct {
					Name string `json:"name"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				tok, err := authProvider.CreateAPIToken(r.Context(), u.ID, req.Name)
				if err != nil {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(tok)
			})

			sub.Get("/tokens", func(w http.ResponseWriter, r *http.Request) {
				u, ok := auth.GetUserFromContext(r.Context())
				if !ok {
					http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
					return
				}
				toks, err := authProvider.ListAPITokens(r.Context(), u.ID)
				if err != nil {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(toks)
			})

			sub.Delete("/tokens/{id}", func(w http.ResponseWriter, r *http.Request) {
				u, ok := auth.GetUserFromContext(r.Context())
				if !ok {
					http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
					return
				}
				tokenID := chi.URLParam(r, "id")
				if err := authProvider.RevokeAPIToken(r.Context(), u.ID, tokenID); err != nil {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusNotFound)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]string{"message": "API token revoked successfully"})
			})
		})
	})

	registry.RegisterAllRoutes(router, auth.Middleware(authProvider))

	// 1. Create API Token using JWT Auth
	createReq := httptest.NewRequest("POST", "/api/v1/auth/tokens", bytes.NewBufferString(`{"name":"Automation Token"}`))
	createReq.Header.Set("Authorization", "Bearer "+jwtToken)
	createReq.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, createReq)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for token creation, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var createdToken auth.APIToken
	if err := json.Unmarshal(rr.Body.Bytes(), &createdToken); err != nil {
		t.Fatalf("failed to decode created token: %v", err)
	}
	if len(createdToken.Token) != 128 {
		t.Fatalf("unexpected token: length %d", len(createdToken.Token))
	}
	if createdToken.UserID != user.ID {
		t.Errorf("expected user_id %s, got %s", user.ID, createdToken.UserID)
	}

	// 2. Use API Token with X-API-Key to query /api/v1/auth/me
	meReq := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	meReq.Header.Set("X-API-Key", createdToken.Token)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, meReq)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK via X-API-Key, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var meUser auth.User
	_ = json.Unmarshal(rr.Body.Bytes(), &meUser)
	if meUser.ID != user.ID || meUser.Username != "tokenuser" {
		t.Errorf("unexpected user from API token: %+v", meUser)
	}

	// 3. Use API Token to add movie to list
	addMovieReq := httptest.NewRequest("POST", "/api/v1/movies", bytes.NewBufferString(`{"title":"Matrix"}`))
	addMovieReq.Header.Set("Authorization", "Bearer "+createdToken.Token)
	addMovieReq.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, addMovieReq)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created via Bearer API Token, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	// 4. Delete API Token
	delReq := httptest.NewRequest("DELETE", "/api/v1/auth/tokens/"+createdToken.ID, nil)
	delReq.Header.Set("Authorization", "Bearer "+jwtToken)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, delReq)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for token revocation, got %d", rr.Code)
	}

	// 5. Subsequent request with revoked token should fail (401)
	meReq2 := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	meReq2.Header.Set("X-API-Key", createdToken.Token)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, meReq2)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized after token revocation, got %d", rr.Code)
	}
}