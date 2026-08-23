package tvshows

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

	if err := db.AutoMigrate(&auth.UserModel{}, &TVShow{}); err != nil {
		t.Fatalf("failed to migrate tables: %v", err)
	}

	user := &auth.User{ID: 1, Username: "testuser", Email: "test@example.com"}
	return db, user
}

func TestTVShowsModuleCRUD(t *testing.T) {
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

	// 1. Create TV Show
	body := []byte(`{"title":"Breaking Bad","current_season":5,"current_episode":16,"total_episodes":62,"status":"completed","rating":10,"notes":"Goat show"}`)
	req := httptest.NewRequest("POST", "/api/v1/tvshows", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	// 2. List TV Shows
	req = httptest.NewRequest("GET", "/api/v1/tvshows", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	var list []TVShow
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatalf("failed to unmarshal list: %v", err)
	}
	if len(list) != 1 || list[0].Title != "Breaking Bad" {
		t.Errorf("unexpected list: %+v", list)
	}
}

func TestTVShowsSharedAcrossUsers(t *testing.T) {
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

	body := []byte(`{"title":"Shared Show","status":"watching"}`)
	req := httptest.NewRequest("POST", "/api/v1/tvshows", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var created TVShow
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created show: %v", err)
	}

	req = httptest.NewRequest("GET", "/api/v1/tvshows", nil)
	req.Header.Set("X-User", "other")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var list []TVShow
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatalf("failed to unmarshal list: %v", err)
	}
	if len(list) != 1 || list[0].ID != created.ID || list[0].Title != "Shared Show" {
		t.Fatalf("expected other user to see the same show, got %+v", list)
	}

	req = httptest.NewRequest("GET", "/api/v1/tvshows/"+strconv.FormatInt(created.ID, 10), nil)
	req.Header.Set("X-User", "other")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for shared item id, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

