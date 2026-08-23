package books

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

	if err := db.AutoMigrate(&auth.UserModel{}, &Book{}, &UserBook{}); err != nil {
		t.Fatalf("failed to migrate tables: %v", err)
	}

	user := &auth.User{ID: 1, Username: "testuser", Email: "test@example.com"}
	return db, user
}

func TestModuleCRUD(t *testing.T) {
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

	// 1. Create
	body := []byte(`{"title":"Sample Item"}`)
	req := httptest.NewRequest("POST", "/api/v1/books", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var created Book
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created item: %v", err)
	}
	if created.ID == "" || created.Title != "Sample Item" {
		t.Errorf("unexpected item: %+v", created)
	}

	// 2. List
	req = httptest.NewRequest("GET", "/api/v1/books", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}
}

func TestBooksSharedAcrossUsers(t *testing.T) {
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

	body := []byte(`{"title":"Shared Book"}`)
	req := httptest.NewRequest("POST", "/api/v1/books", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var created Book
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created book: %v", err)
	}

	req = httptest.NewRequest("GET", "/api/v1/books", nil)
	req.Header.Set("X-User", "other")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var list []Book
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatalf("failed to unmarshal list: %v", err)
	}
	if len(list) != 1 || list[0].ID != created.ID || list[0].Title != "Shared Book" {
		t.Fatalf("expected other user to see the same book, got %+v", list)
	}

	req = httptest.NewRequest("GET", "/api/v1/books/"+created.ID, nil)
	req.Header.Set("X-User", "other")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for shared item id, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}
