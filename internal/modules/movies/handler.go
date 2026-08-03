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
		sub.Post("/bulk-delete", m.bulkDeleteMovies)
		sub.Post("/bulk-status", m.bulkStatusMovies)
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

	qParam := strings.TrimSpace(r.URL.Query().Get("q"))
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	sortByParam := strings.TrimSpace(r.URL.Query().Get("sort_by"))
	orderParam := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("order")))
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	whereClauses := []string{"user_id = ?"}
	args := []interface{}{user.ID}

	if statusFilter != "" {
		whereClauses = append(whereClauses, "status = ?")
		args = append(args, statusFilter)
	}

	if qParam != "" {
		whereClauses = append(whereClauses, "(LOWER(title) LIKE ? OR LOWER(director) LIKE ? OR LOWER(notes) LIKE ?)")
		pattern := "%" + strings.ToLower(qParam) + "%"
		args = append(args, pattern, pattern, pattern)
	}

	whereStmt := strings.Join(whereClauses, " AND ")

	// Validate sorting column
	validSortColumns := map[string]string{
		"id":           "id",
		"title":        "title",
		"release_year": "release_year",
		"director":     "director",
		"status":       "status",
		"rating":       "rating",
		"created_at":   "created_at",
		"updated_at":   "updated_at",
	}

	sortCol, valid := validSortColumns[sortByParam]
	if !valid {
		sortCol = "updated_at"
	}

	if orderParam != "asc" && orderParam != "desc" {
		orderParam = "desc"
	}

	isPaginated := pageStr != "" || limitStr != ""
	page := 1
	limit := 50

	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
			if limit > 100 {
				limit = 100
			}
		}
	}

	var total int
	if isPaginated {
		countQuery := "SELECT COUNT(*) FROM movies WHERE " + whereStmt
		_ = m.db.QueryRowContext(r.Context(), countQuery, args...).Scan(&total)
	}

	query := "SELECT id, user_id, title, release_year, director, status, rating, notes, created_at, updated_at FROM movies WHERE " + whereStmt + " ORDER BY " + sortCol + " " + strings.ToUpper(orderParam)

	if isPaginated {
		offset := (page - 1) * limit
		query += " LIMIT " + strconv.Itoa(limit) + " OFFSET " + strconv.Itoa(offset)
	}

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

	if isPaginated {
		totalPages := (total + limit - 1) / limit
		if totalPages < 0 {
			totalPages = 0
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"data": movies,
			"pagination": map[string]interface{}{
				"total":       total,
				"page":        page,
				"limit":       limit,
				"total_pages": totalPages,
			},
		})
		return
	}

	respondJSON(w, http.StatusOK, movies)
}

func (m *Module) bulkDeleteMovies(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		respondError(w, http.StatusBadRequest, "Invalid request body: ids array required")
		return
	}

	placeholders := make([]string, len(req.IDs))
	args := make([]interface{}, 0, len(req.IDs)+1)
	args = append(args, user.ID)
	for i, id := range req.IDs {
		placeholders[i] = "?"
		args = append(args, id)
	}

	query := "DELETE FROM movies WHERE user_id = ? AND id IN (" + strings.Join(placeholders, ",") + ")"
	res, err := m.db.ExecContext(r.Context(), query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to bulk delete movies: "+err.Error())
		return
	}

	rowsAffected, _ := res.RowsAffected()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "Movies deleted successfully",
		"deleted_count": rowsAffected,
	})
}

func (m *Module) bulkStatusMovies(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		IDs    []int64 `json:"ids"`
		Status string  `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 || strings.TrimSpace(req.Status) == "" {
		respondError(w, http.StatusBadRequest, "Invalid request body: ids array and status required")
		return
	}

	placeholders := make([]string, len(req.IDs))
	now := time.Now()
	args := make([]interface{}, 0, len(req.IDs)+3)
	args = append(args, req.Status, now, user.ID)
	for i, id := range req.IDs {
		placeholders[i] = "?"
		args = append(args, id)
	}

	query := "UPDATE movies SET status = ?, updated_at = ? WHERE user_id = ? AND id IN (" + strings.Join(placeholders, ",") + ")"
	res, err := m.db.ExecContext(r.Context(), query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to bulk update status: "+err.Error())
		return
	}

	rowsAffected, _ := res.RowsAffected()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "Movies status updated successfully",
		"updated_count": rowsAffected,
	})
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
