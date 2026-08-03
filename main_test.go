package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"mtvl/internal/auth"
	"mtvl/internal/core"
	"mtvl/internal/db"
	"mtvl/internal/docs"
	"mtvl/internal/modules/books"
	"mtvl/internal/modules/movies"
	"mtvl/internal/modules/tvshows"
	"mtvl/internal/services"
)

func setupTestRouter(t *testing.T) (*chi.Mux, string, auth.AuthProvider) {
	database, err := db.OpenDB("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	if err := db.RunMigrations(database, "sqlite3", migrationFS, "migrations"); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
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

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "up"})
	})

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

		r.Group(func(sub chi.Router) {
			sub.Use(auth.Middleware(authProvider))

			sub.Get("/me", func(w http.ResponseWriter, r *http.Request) {
				user, _ := auth.GetUserFromContext(r.Context())
				_ = json.NewEncoder(w).Encode(user)
			})

			sub.Put("/me", func(w http.ResponseWriter, r *http.Request) {
				user, _ := auth.GetUserFromContext(r.Context())
				var req struct {
					Username string `json:"username"`
					Email    string `json:"email"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				updatedUser, err := authProvider.UpdateUser(r.Context(), user.ID, req.Username, req.Email)
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
				_ = json.NewEncoder(w).Encode(updatedUser)
			})

			sub.Put("/password", func(w http.ResponseWriter, r *http.Request) {
				user, _ := auth.GetUserFromContext(r.Context())
				var req struct {
					OldPassword string `json:"old_password"`
					NewPassword string `json:"new_password"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				if err := authProvider.ChangePassword(r.Context(), user.ID, req.OldPassword, req.NewPassword); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]string{"message": "Password updated successfully"})
			})

			sub.Delete("/me", func(w http.ResponseWriter, r *http.Request) {
				user, _ := auth.GetUserFromContext(r.Context())
				if err := authProvider.DeleteUser(r.Context(), user.ID); err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]string{"message": "Account deleted successfully"})
			})
		})
	})

	registry.RegisterAllRoutes(router, auth.Middleware(authProvider))

	services.NewServiceHandler(database).RegisterRoutes(router, auth.Middleware(authProvider))
	docs.NewDocsHandler().RegisterRoutes(router)

	// Register test user
	regBody := []byte(`{"username":"alice","email":"alice@example.com","password":"password123"}`)
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(regBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Login test user
	loginBody := []byte(`{"username_or_email":"alice","password":"password123"}`)
	req = httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	var loginResp struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &loginResp)

	return router, loginResp.Token, authProvider
}

func TestFullServerIntegrationFlow(t *testing.T) {
	router, token, _ := setupTestRouter(t)

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

	// 4. Update Profile
	updateBody := []byte(`{"username":"alice_updated","email":"alice_new@example.com"}`)
	req = httptest.NewRequest("PUT", "/api/v1/auth/me", bytes.NewBuffer(updateBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("update profile failed: %d, body: %s", rr.Code, rr.Body.String())
	}

	// 5. Change Password
	pwBody := []byte(`{"old_password":"password123","new_password":"newpassword456"}`)
	req = httptest.NewRequest("PUT", "/api/v1/auth/password", bytes.NewBuffer(pwBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("change password failed: %d, body: %s", rr.Code, rr.Body.String())
	}

	// Re-login with new password
	reloginBody := []byte(`{"username_or_email":"alice_updated","password":"newpassword456"}`)
	req = httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(reloginBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("re-login with new password failed: %d", rr.Code)
	}
	var newLoginResp struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &newLoginResp)
	newToken := newLoginResp.Token

	// 6. Create Movies, TV Show, Book
	movie1 := []byte(`{"title":"Inception","release_year":2010,"director":"Nolan","status":"completed","rating":9}`)
	req = httptest.NewRequest("POST", "/api/v1/movies", bytes.NewBuffer(movie1))
	req.Header.Set("Authorization", "Bearer "+newToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("movie 1 creation failed: %d", rr.Code)
	}
	var createdMov1 struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &createdMov1)

	movie2 := []byte(`{"title":"Interstellar","release_year":2014,"director":"Nolan","status":"completed","rating":10}`)
	req = httptest.NewRequest("POST", "/api/v1/movies", bytes.NewBuffer(movie2))
	req.Header.Set("Authorization", "Bearer "+newToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("movie 2 creation failed: %d", rr.Code)
	}
	var createdMov2 struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &createdMov2)

	book1 := []byte(`{"title":"Dune","status":"completed","rating":10,"notes":"Sci-Fi classic"}`)
	req = httptest.NewRequest("POST", "/api/v1/books", bytes.NewBuffer(book1))
	req.Header.Set("Authorization", "Bearer "+newToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("book creation failed: %d", rr.Code)
	}

	// 7. Test Search, Sorting, and Pagination
	req = httptest.NewRequest("GET", "/api/v1/movies?q=Inter&sort_by=release_year&order=desc&page=1&limit=10", nil)
	req.Header.Set("Authorization", "Bearer "+newToken)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("movies list query failed: %d", rr.Code)
	}
	var paginatedResp struct {
		Data       []movies.Movie `json:"data"`
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &paginatedResp)
	if len(paginatedResp.Data) != 1 || paginatedResp.Data[0].Title != "Interstellar" {
		t.Errorf("expected 1 paginated search match 'Interstellar', got %v", paginatedResp.Data)
	}

	// 8. Test Bulk Operations
	bulkStatusBody, _ := json.Marshal(map[string]interface{}{
		"ids":    []int64{createdMov1.ID, createdMov2.ID},
		"status": "plan_to_watch",
	})
	req = httptest.NewRequest("POST", "/api/v1/movies/bulk-status", bytes.NewBuffer(bulkStatusBody))
	req.Header.Set("Authorization", "Bearer "+newToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("bulk status failed: %d", rr.Code)
	}

	// 9. Test Dashboard Stats
	req = httptest.NewRequest("GET", "/api/v1/stats", nil)
	req.Header.Set("Authorization", "Bearer "+newToken)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("stats query failed: %d", rr.Code)
	}
	var stats map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &stats)
	if int(stats["total_items"].(float64)) != 3 {
		t.Errorf("expected 3 total items in stats, got %v", stats["total_items"])
	}

	// 10. Test Global Search
	req = httptest.NewRequest("GET", "/api/v1/search?q=Dune", nil)
	req.Header.Set("Authorization", "Bearer "+newToken)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("global search failed: %d", rr.Code)
	}

	// 11. Test Export Data
	req = httptest.NewRequest("GET", "/api/v1/export", nil)
	req.Header.Set("Authorization", "Bearer "+newToken)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("export failed: %d", rr.Code)
	}

	// 12. Test OpenAPI Spec & Docs
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

	// 13. Delete Account
	req = httptest.NewRequest("DELETE", "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+newToken)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("delete account failed: %d", rr.Code)
	}
}
