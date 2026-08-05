package movies

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
	"mtvl/internal/auth"
	"mtvl/internal/core"
)

// Module implements core.CategoryModule for Movies.
type Module struct {
	db *gorm.DB
}

// NewModule initializes a new Movies CategoryModule.
func NewModule(db *gorm.DB) *Module {
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

	query := m.db.WithContext(r.Context()).Model(&Movie{}).Where("user_id = ?", user.ID)

	if statusFilter != "" {
		query = query.Where("status = ?", statusFilter)
	}

	if qParam != "" {
		pattern := "%" + strings.ToLower(qParam) + "%"
		query = query.Where("(LOWER(title) LIKE ? OR LOWER(director) LIKE ? OR LOWER(notes) LIKE ?)", pattern, pattern, pattern)
	}

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

	var total int64
	if isPaginated {
		if err := query.Count(&total).Error; err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to count movies: "+err.Error())
			return
		}
	}

	query = query.Order(sortCol + " " + strings.ToUpper(orderParam))

	if isPaginated {
		offset := (page - 1) * limit
		query = query.Offset(offset).Limit(limit)
	}

	movies := make([]Movie, 0)
	if err := query.Find(&movies).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to fetch movies: "+err.Error())
		return
	}

	if isPaginated {
		totalPages := (int(total) + limit - 1) / limit
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

	res := m.db.WithContext(r.Context()).Where("user_id = ? AND id IN ?", user.ID, req.IDs).Delete(&Movie{})
	if res.Error != nil {
		respondError(w, http.StatusInternalServerError, "Failed to bulk delete movies: "+res.Error.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "Movies deleted successfully",
		"deleted_count": res.RowsAffected,
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

	now := time.Now()
	res := m.db.WithContext(r.Context()).Model(&Movie{}).
		Where("user_id = ? AND id IN ?", user.ID, req.IDs).
		Updates(map[string]interface{}{
			"status":     req.Status,
			"updated_at": now,
		})

	if res.Error != nil {
		respondError(w, http.StatusInternalServerError, "Failed to bulk update status: "+res.Error.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "Movies status updated successfully",
		"updated_count": res.RowsAffected,
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
	movie := Movie{
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

	if err := m.db.WithContext(r.Context()).Create(&movie).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to insert movie: "+err.Error())
		return
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
	err = m.db.WithContext(r.Context()).Where("id = ? AND user_id = ?", id, user.ID).First(&mov).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
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
	res := m.db.WithContext(r.Context()).Model(&Movie{}).
		Where("id = ? AND user_id = ?", id, user.ID).
		Updates(map[string]interface{}{
			"title":        req.Title,
			"release_year": req.ReleaseYear,
			"director":     req.Director,
			"status":       req.Status,
			"rating":       req.Rating,
			"notes":        req.Notes,
			"updated_at":   now,
		})

	if res.Error != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update movie: "+res.Error.Error())
		return
	}

	if res.RowsAffected == 0 {
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

	res := m.db.WithContext(r.Context()).Where("id = ? AND user_id = ?", id, user.ID).Delete(&Movie{})
	if res.Error != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete movie: "+res.Error.Error())
		return
	}

	if res.RowsAffected == 0 {
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

