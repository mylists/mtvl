package movies

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mtvl/internal/idgen"

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

	if err := db.AutoMigrate(&auth.UserModel{}, &Movie{}, &UserMovie{}); err != nil {
		t.Fatalf("failed to migrate tables: %v", err)
	}

	user := &auth.User{ID: idgen.New(), Username: "testuser", Email: "test@example.com"}
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
	body := []byte(`{"title":"Inception","release_year":2010,"director":"Christopher Nolan"}`)
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
	if _, ok := idgen.Parse(list[0].ID); !ok {
		t.Errorf("expected unique UUID id, got %q", list[0].ID)
	}
}

func TestMoviesSharedAcrossUsers(t *testing.T) {
	db, creator := setupTestDB(t)
	other := &auth.User{ID: idgen.New(), Username: "other", Email: "other@example.com"}

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

	body := []byte(`{"title":"Shared Movie"}`)
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

	req = httptest.NewRequest("GET", "/api/v1/movies/"+created.ID, nil)
	req.Header.Set("X-User", "other")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for shared item id, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestMovieCatalogReadsArePublicAndWritesRequireAuthentication(t *testing.T) {
	db, _ := setupTestDB(t)
	created := Movie{ID: idgen.New(), Title: "Public Catalogue Movie"}
	if err := db.Create(&created).Error; err != nil {
		t.Fatalf("seed movie: %v", err)
	}

	mod := NewModule(db)
	router := chi.NewRouter()
	denied := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
		})
	}
	mod.RegisterRoutes(router, denied)

	for _, path := range []string{"/api/v1/movies?q=catalogue", "/api/v1/movies/" + created.ID} {
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != http.StatusOK {
			t.Errorf("GET %s: expected public 200, got %d: %s", path, rr.Code, rr.Body.String())
		}
	}

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/movies", bytes.NewBufferString(`{"title":"New Movie"}`)))
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("POST /api/v1/movies: expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMoviesUserListIsolation(t *testing.T) {
	db, creator := setupTestDB(t)
	other := &auth.User{ID: idgen.New(), Username: "other", Email: "other@example.com"}

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

	body := []byte(`{"title":"List Movie"}`)
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

	addBody, _ := json.Marshal(map[string]interface{}{
		"id":     created.ID,
		"status": "completed",
		"rating": 9,
	})
	req = httptest.NewRequest("POST", "/api/v1/movies/list", bytes.NewBuffer(addBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for list add, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest("GET", "/api/v1/movies/list", nil)
	req.Header.Set("X-User", "other")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var otherList []MovieListItem
	if err := json.Unmarshal(rr.Body.Bytes(), &otherList); err != nil {
		t.Fatalf("failed to unmarshal other user list: %v", err)
	}
	if len(otherList) != 0 {
		t.Fatalf("expected other user list to be empty, got %+v", otherList)
	}

	req = httptest.NewRequest("GET", "/api/v1/movies/list", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var creatorList []MovieListItem
	if err := json.Unmarshal(rr.Body.Bytes(), &creatorList); err != nil {
		t.Fatalf("failed to unmarshal creator list: %v", err)
	}
	if len(creatorList) != 1 || creatorList[0].ID != created.ID || creatorList[0].Status != "completed" {
		t.Fatalf("expected creator list to include the movie, got %+v", creatorList)
	}
}
