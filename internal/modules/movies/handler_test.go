package movies

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"mtvl/internal/auth"
)

func TestMoviesModuleCRUD(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	user := &auth.User{ID: 1, Username: "testuser", Email: "test@example.com"}
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
	mock.ExpectExec("INSERT INTO movies").
		WithArgs(int64(1), "Inception", 2010, "Christopher Nolan", "completed", 10, "Masterpiece", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := []byte(`{"title":"Inception","release_year":2010,"director":"Christopher Nolan","status":"completed","rating":10,"notes":"Masterpiece"}`)
	req := httptest.NewRequest("POST", "/api/v1/movies", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	// 2. List Movies
	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "user_id", "title", "release_year", "director", "status", "rating", "notes", "created_at", "updated_at"}).
		AddRow(1, 1, "Inception", 2010, "Christopher Nolan", "completed", 10, "Masterpiece", now, now)

	mock.ExpectQuery("SELECT id, user_id, title, release_year, director, status, rating, notes, created_at, updated_at FROM movies").
		WithArgs(int64(1)).
		WillReturnRows(rows)

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
