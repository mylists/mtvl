package tvshows

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

func setupTestTVShowsDB(t *testing.T) (*sql.DB, *auth.User) {
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
		`CREATE TABLE tv_shows (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			title VARCHAR(255) NOT NULL,
			current_season INTEGER DEFAULT 1,
			current_episode INTEGER DEFAULT 0,
			total_episodes INTEGER DEFAULT 0,
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

func TestTVShowsModuleCRUD(t *testing.T) {
	db, user := setupTestTVShowsDB(t)
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

	// 1. Create TV Show
	body := []byte(`{"title":"Breaking Bad","current_season":5,"current_episode":16,"total_episodes":62,"status":"completed","rating":10,"notes":"Goat show"}`)
	req := httptest.NewRequest("POST", "/api/v1/tvshows", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var created TVShow
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created TV show: %v", err)
	}
	if created.ID == 0 || created.Title != "Breaking Bad" || created.CurrentSeason != 5 {
		t.Errorf("unexpected created TV show: %+v", created)
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

	// 3. Get TV Show
	req = httptest.NewRequest("GET", "/api/v1/tvshows/1", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	// 4. Update TV Show
	updateBody := []byte(`{"title":"Breaking Bad","current_season":5,"current_episode":16,"total_episodes":62,"status":"completed","rating":10,"notes":"Goat show rewatched"}`)
	req = httptest.NewRequest("PUT", "/api/v1/tvshows/1", bytes.NewBuffer(updateBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on update, got %d", rr.Code)
	}

	// 5. Delete TV Show
	req = httptest.NewRequest("DELETE", "/api/v1/tvshows/1", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on delete, got %d", rr.Code)
	}

	// Verify Deleted
	req = httptest.NewRequest("GET", "/api/v1/tvshows/1", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found after deletion, got %d", rr.Code)
	}
}
