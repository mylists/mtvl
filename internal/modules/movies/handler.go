package movies

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"mtvl/internal/auth"
	"mtvl/internal/core"
)

// Module implements core.CategoryModule for Movies.
type Module struct {
	db *sql.DB
}

// NewModule initializes a new Movies CategoryModule.
func NewModule(db *sql.DB) *Module {
	return &Module{db: db}
}

func (m *Module) Info() core.CategoryInfo {
	return core.CategoryInfo{
		Category:    "movies",
		DisplayName: "Movies",
		Description: "Track movies you have watched or plan to watch",
		Endpoint:    "/api/v1/movies",
	}
}

func (m *Module) RegisterRoutes(r chi.Router, authMw func(http.Handler) http.Handler) {
	r.Route("/api/v1/movies", func(sub chi.Router) {
		sub.Use(authMw)

		sub.Get("/", m.listMovies)
		sub.Post("/", m.createMovie)
		sub.Get("/{id}", m.getMovie)
		sub.Put("/{id}", m.updateMovie)
		sub.Delete("/{id}", m.deleteMovie)
	})
}

func (m *Module) listMovies(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	statusFilter := r.URL.Query().Get("status")

	query := `SELECT id, user_id, title, release_year, director, status, rating, notes, created_at, updated_at FROM movies WHERE user_id = ?`
	args := []interface{}{user.ID}

	if statusFilter != "" {
		query += ` AND status = ?`
		args = append(args, statusFilter)
	}

	query += ` ORDER BY updated_at DESC`

	rows, err := m.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to fetch movies: "+err.Error())
		return
	}
	defer rows.Close()

	movies := make([]Movie, 0)
	for rows.Next() {
		var mov Movie
		if err := rows.Scan(&mov.ID, &mov.UserID, &mov.Title, &mov.ReleaseYear, &mov.Director, &mov.Status, &mov.Rating, &mov.Notes, &mov.CreatedAt, &mov.UpdatedAt); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to scan movie: "+err.Error())
			return
		}
		movies = append(movies, mov)
	}

	respondJSON(w, http.StatusOK, movies)
}

func (m *Module) createMovie(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		Title       string `json:"title"`
		ReleaseYear int    `json:"release_year"`
		Director    string `json:"director"`
		Status      string `json:"status"`
		Rating      int    `json:"rating"`
		Notes       string `json:"notes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		respondError(w, http.StatusBadRequest, "Title is required")
		return
	}

	if req.Status == "" {
		req.Status = "plan_to_watch"
	}

	now := time.Now()
	query := `INSERT INTO movies (user_id, title, release_year, director, status, rating, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := m.db.ExecContext(r.Context(), query, user.ID, req.Title, req.ReleaseYear, req.Director, req.Status, req.Rating, req.Notes, now, now)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to insert movie: "+err.Error())
		return
	}

	id, _ := res.LastInsertId()
	movie := Movie{
		ID:          id,
		UserID:      user.ID,
		Title:       req.Title,
		ReleaseYear: req.ReleaseYear,
		Director:    req.Director,
		Status:      req.Status,
		Rating:      req.Rating,
		Notes:       req.Notes,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	respondJSON(w, http.StatusCreated, movie)
}

func (m *Module) getMovie(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid movie ID")
		return
	}

	var mov Movie
	query := `SELECT id, user_id, title, release_year, director, status, rating, notes, created_at, updated_at FROM movies WHERE id = ? AND user_id = ?`
	err = m.db.QueryRowContext(r.Context(), query, id, user.ID).Scan(&mov.ID, &mov.UserID, &mov.Title, &mov.ReleaseYear, &mov.Director, &mov.Status, &mov.Rating, &mov.Notes, &mov.CreatedAt, &mov.UpdatedAt)
	if err == sql.ErrNoRows {
		respondError(w, http.StatusNotFound, "Movie not found")
		return
	} else if err != nil {
		respondError(w, http.StatusInternalServerError, "Database query error: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, mov)
}

func (m *Module) updateMovie(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid movie ID")
		return
	}

	var req struct {
		Title       string `json:"title"`
		ReleaseYear int    `json:"release_year"`
		Director    string `json:"director"`
		Status      string `json:"status"`
		Rating      int    `json:"rating"`
		Notes       string `json:"notes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	now := time.Now()
	query := `UPDATE movies SET title = ?, release_year = ?, director = ?, status = ?, rating = ?, notes = ?, updated_at = ? WHERE id = ? AND user_id = ?`
	res, err := m.db.ExecContext(r.Context(), query, req.Title, req.ReleaseYear, req.Director, req.Status, req.Rating, req.Notes, now, id, user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update movie: "+err.Error())
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		respondError(w, http.StatusNotFound, "Movie not found")
		return
	}

	m.getMovie(w, r)
}

func (m *Module) deleteMovie(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid movie ID")
		return
	}

	query := `DELETE FROM movies WHERE id = ? AND user_id = ?`
	res, err := m.db.ExecContext(r.Context(), query, id, user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete movie: "+err.Error())
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		respondError(w, http.StatusNotFound, "Movie not found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Movie deleted successfully"})
}

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
