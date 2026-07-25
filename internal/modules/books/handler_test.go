package books

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	_ "modernc.org/sqlite"
	"mtvl/internal/auth"
)

func setupTestDB(t *testing.T) (*sql.DB, *auth.User) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}

	schemas := []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, username VARCHAR(100) NOT NULL, email VARCHAR(255) NOT NULL, password_hash VARCHAR(255) NOT NULL, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`,
		`CREATE TABLE books (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, title VARCHAR(255) NOT NULL, status VARCHAR(50) DEFAULT 'plan_to_watch', rating INTEGER DEFAULT 0, notes TEXT DEFAULT '', created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`,
	}

	for _, s := range schemas {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("failed to create table: %v", err)
		}
	}

	user := &auth.User{ID: 1, Username: "testuser", Email: "test@example.com"}
	return db, user
}

func TestModuleCRUD(t *testing.T) {
	db, user := setupTestDB(t)
	defer db.Close()

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
	body := []byte(`{"title":"Sample Item","status":"completed","rating":9}`)
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
	if created.ID == 0 || created.Title != "Sample Item" {
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
