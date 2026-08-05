package books

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

func TestModuleCRUD(t *testing.T) {
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

	// 1. Create
	mock.ExpectExec("INSERT INTO books").
		WithArgs(int64(1), "Sample Item", "completed", 9, "", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

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
	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "user_id", "title", "status", "rating", "notes", "created_at", "updated_at"}).
		AddRow(1, 1, "Sample Item", "completed", 9, "", now, now)

	mock.ExpectQuery("SELECT id, user_id, title, status, rating, notes, created_at, updated_at FROM books").
		WithArgs(int64(1)).
		WillReturnRows(rows)

	req = httptest.NewRequest("GET", "/api/v1/books", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}
}
