package movies

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

func setupTestMoviesDB(t *testing.T) (*sql.DB, *auth.User) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}

	schemas := []string{
		`CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username VARCHAR(100) NOT NULL,
			email VARCHAR(255) NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE movies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			title VARCHAR(255) NOT NULL,
			release_year INTEGER DEFAULT 0,
			director VARCHAR(255) DEFAULT '',
			status VARCHAR(50) DEFAULT 'plan_to_watch',
			rating INTEGER DEFAULT 0,
			notes TEXT DEFAULT '',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	for _, s := range schemas {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("failed to create table: %v", err)
		}
	}

	user := &auth.User{ID: 1, Username: "testuser", Email: "test@example.com"}
	return db, user
}

func TestMoviesModuleCRUD(t *testing.T) {
	db, user := setupTestMoviesDB(t)
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

	// 1. Create Movie
	body := []byte(`{"title":"Inception","release_year":2010,"director":"Christopher Nolan","status":"completed","rating":10,"notes":"Masterpiece"}`)
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
	if created.ID == 0 || created.Title != "Inception" || created.Rating != 10 {
		t.Errorf("unexpected created movie: %+v", created)
	}

	// 2. List Movies
	req = httptest.NewRequest("GET", "/api/v1/movies", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	var list []Movie
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatalf("failed to unmarshal movie list: %v", err)
	}
	if len(list) != 1 || list[0].Title != "Inception" {
		t.Errorf("unexpected list: %+v", list)
	}

	// 3. Get Movie
	req = httptest.NewRequest("GET", "/api/v1/movies/1", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	// 4. Update Movie
	updateBody := []byte(`{"title":"Inception (Updated)","release_year":2010,"director":"Christopher Nolan","status":"completed","rating":9,"notes":"Rewatched"}`)
	req = httptest.NewRequest("PUT", "/api/v1/movies/1", bytes.NewBuffer(updateBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on update, got %d", rr.Code)
	}

	// 5. Delete Movie
	req = httptest.NewRequest("DELETE", "/api/v1/movies/1", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on delete, got %d", rr.Code)
	}

	// Verify Deleted
	req = httptest.NewRequest("GET", "/api/v1/movies/1", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found after deletion, got %d", rr.Code)
	}
}
