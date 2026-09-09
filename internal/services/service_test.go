package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"mtvl/internal/auth"
	"mtvl/internal/idgen"
	"mtvl/internal/modules/books"
	"mtvl/internal/modules/movies"
	"mtvl/internal/modules/tvshows"
)

func TestGetStatsUsesUUIDUserIDs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	if err := db.AutoMigrate(&auth.UserModel{}, &movies.Movie{}, &movies.UserMovie{}, &tvshows.TVShow{}, &tvshows.UserTVShow{}, &books.Book{}, &books.UserBook{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	user := &auth.User{ID: idgen.New(), Username: "testuser", Email: "test@example.com"}
	handler := NewServiceHandler(db)
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(auth.WithUserContext(r.Context(), user)))
		})
	})
	handler.RegisterRoutes(router, func(next http.Handler) http.Handler { return next })

	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode stats: %v", err)
	}
	if _, ok := payload["total_items"]; !ok {
		t.Fatalf("expected total_items in stats payload, got %+v", payload)
	}
}

func TestGlobalSearchIsPublic(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&movies.Movie{}, &tvshows.TVShow{}, &books.Book{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	if err := db.Create(&movies.Movie{ID: idgen.New(), Title: "Public Search Result"}).Error; err != nil {
		t.Fatalf("seed movie: %v", err)
	}

	handler := NewServiceHandler(db)
	router := chi.NewRouter()
	denied := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
		})
	}
	handler.RegisterRoutes(router, denied)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/search?q=public", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected public search to return 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
