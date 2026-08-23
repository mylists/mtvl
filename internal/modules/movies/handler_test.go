package movies

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"mtvl/internal/auth"
)

func setupTestDB(t *testing.T) (*gorm.DB, *auth.User) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}

	if err := db.AutoMigrate(&auth.UserModel{}, &Movie{}); err != nil {
		t.Fatalf("failed to migrate tables: %v", err)
	}

	user := &auth.User{ID: 1, Username: "testuser", Email: "test@example.com"}
	return db, user
}

func TestMoviesModuleCRUD(t *testing.T) {
	db, user := setupTestDB(t)

	mod := NewModule(db)
	router := chi.NewRouter()

	authMw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithUserContext(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	mod.RegisterRoutes(router, authMw)

	// 1. Create Movie
	body := []byte(`{"title":"Inception","release_year":2010,"director":"Christopher Nolan","status":"completed","rating":10,"notes":"Masterpiece"}`)
	req := httptest.NewRequest("POST", "/api/v1/movies", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	// 2. List Movies
	req = httptest.NewRequest("GET", "/api/v1/movies", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var list []Movie
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatalf("failed to unmarshal movie list: %v", err)
	}
	if len(list) != 1 || list[0].Title != "Inception" {
		t.Errorf("unexpected list: %+v", list)
	}
}

func TestMoviesSharedAcrossUsers(t *testing.T) {
	db, creator := setupTestDB(t)
	other := &auth.User{ID: 2, Username: "other", Email: "other@example.com"}

	mod := NewModule(db)
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := creator
			if r.Header.Get("X-User") == "other" {
				user = other
			}
			next.ServeHTTP(w, r.WithContext(auth.WithUserContext(r.Context(), user)))
		})
	})
	mod.RegisterRoutes(router, func(next http.Handler) http.Handler { return next })

	body := []byte(`{"title":"Shared Movie","status":"completed"}`)
	req := httptest.NewRequest("POST", "/api/v1/movies", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var created Movie
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created movie: %v", err)
	}

	req = httptest.NewRequest("GET", "/api/v1/movies", nil)
	req.Header.Set("X-User", "other")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var list []Movie
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatalf("failed to unmarshal movie list: %v", err)
	}
	if len(list) != 1 || list[0].ID != created.ID || list[0].Title != "Shared Movie" {
		t.Fatalf("expected other user to see the same movie, got %+v", list)
	}

	req = httptest.NewRequest("GET", "/api/v1/movies/"+strconv.FormatInt(created.ID, 10), nil)
	req.Header.Set("X-User", "other")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for shared item id, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

