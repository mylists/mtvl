package tvshows

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

func TestTVShowsModuleCRUD(t *testing.T) {
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

	// 1. Create TV Show
	mock.ExpectExec("INSERT INTO tv_shows").
		WithArgs(int64(1), "Breaking Bad", 5, 16, 62, "completed", 10, "Goat show", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := []byte(`{"title":"Breaking Bad","current_season":5,"current_episode":16,"total_episodes":62,"status":"completed","rating":10,"notes":"Goat show"}`)
	req := httptest.NewRequest("POST", "/api/v1/tvshows", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	// 2. List TV Shows
	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "user_id", "title", "current_season", "current_episode", "total_episodes", "status", "rating", "notes", "created_at", "updated_at"}).
		AddRow(1, 1, "Breaking Bad", 5, 16, 62, "completed", 10, "Goat show", now, now)

	mock.ExpectQuery("SELECT id, user_id, title, current_season, current_episode, total_episodes, status, rating, notes, created_at, updated_at FROM tv_shows").
		WithArgs(int64(1)).
		WillReturnRows(rows)

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
