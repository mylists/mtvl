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
	"mtvl/internal/idgen"
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
		// The shared catalog is deliberately public so visitors can search and
		// select an existing movie before deciding to add it to a personal list.
		sub.Get("/", m.listMovies)
		sub.Get("/{id}", m.getMovie)

		sub.Group(func(protected chi.Router) {
			protected.Use(authMw)
			protected.Post("/", m.createMovie)
			protected.Post("/bulk-delete", m.bulkDeleteMovies)

			protected.Get("/list", m.listUserMovies)
			protected.Post("/list", m.addMovieToList)
			protected.Post("/list/bulk-delete", m.bulkRemoveFromList)
			protected.Post("/list/bulk-status", m.bulkStatusMovies)
			protected.Post("/bulk-status", m.bulkStatusMovies)
			protected.Get("/list/{id}", m.getUserMovie)
			protected.Put("/list/{id}", m.updateUserMovie)
			protected.Delete("/list/{id}", m.removeMovieFromList)
			protected.Put("/{id}", m.updateMovie)
			protected.Delete("/{id}", m.deleteMovie)
		})
	})
}

func (m *Module) listMovies(w http.ResponseWriter, r *http.Request) {
	qParam := strings.TrimSpace(r.URL.Query().Get("q"))
	sortByParam := strings.TrimSpace(r.URL.Query().Get("sort_by"))
	orderParam := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("order")))
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	query := m.db.WithContext(r.Context()).Model(&Movie{})

	if qParam != "" {
		pattern := "%" + strings.ToLower(qParam) + "%"
		query = query.Where("(LOWER(title) LIKE ? OR LOWER(director) LIKE ?)", pattern, pattern)
	}

	validSortColumns := map[string]string{
		"id":           "id",
		"title":        "title",
		"release_year": "release_year",
		"director":     "director",
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
	if _, ok := auth.GetUserFromContext(r.Context()); !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		respondError(w, http.StatusBadRequest, "Invalid request body: ids array required")
		return
	}

	var deletedCount int64
	err := m.db.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("movie_id IN ?", idgen.Args(req.IDs)).Delete(&UserMovie{}).Error; err != nil {
			return err
		}
		res := tx.Where("id IN ?", idgen.Args(req.IDs)).Delete(&Movie{})
		deletedCount = res.RowsAffected
		return res.Error
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to bulk delete movies: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "Movies deleted successfully",
		"deleted_count": deletedCount,
	})
}

func (m *Module) createMovie(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.GetUserFromContext(r.Context()); !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		Title       string `json:"title"`
		ReleaseYear int    `json:"release_year"`
		Director    string `json:"director"`
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

	now := time.Now()
	movie := Movie{
		Title:       req.Title,
		ReleaseYear: req.ReleaseYear,
		Director:    req.Director,
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
	id, ok := idgen.Parse(chi.URLParam(r, "id"))
	if !ok {
		respondError(w, http.StatusBadRequest, "Invalid movie ID")
		return
	}

	var mov Movie
	err := m.db.WithContext(r.Context()).Where("id = ?", idgen.Arg(id)).First(&mov).Error
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
	if _, ok := auth.GetUserFromContext(r.Context()); !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	id, ok := idgen.Parse(chi.URLParam(r, "id"))
	if !ok {
		respondError(w, http.StatusBadRequest, "Invalid movie ID")
		return
	}

	var req struct {
		Title       string `json:"title"`
		ReleaseYear int    `json:"release_year"`
		Director    string `json:"director"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	now := time.Now()
	res := m.db.WithContext(r.Context()).Model(&Movie{}).
		Where("id = ?", idgen.Arg(id)).
		Updates(map[string]interface{}{
			"title":        req.Title,
			"release_year": req.ReleaseYear,
			"director":     req.Director,
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
	if _, ok := auth.GetUserFromContext(r.Context()); !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	id, ok := idgen.Parse(chi.URLParam(r, "id"))
	if !ok {
		respondError(w, http.StatusBadRequest, "Invalid movie ID")
		return
	}

	var deleted int64
	err := m.db.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("movie_id = ?", idgen.Arg(id)).Delete(&UserMovie{}).Error; err != nil {
			return err
		}
		res := tx.Where("id = ?", idgen.Arg(id)).Delete(&Movie{})
		deleted = res.RowsAffected
		return res.Error
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete movie: "+err.Error())
		return
	}

	if deleted == 0 {
		respondError(w, http.StatusNotFound, "Movie not found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Movie deleted successfully"})
}

func (m *Module) userMovieQuery(r *http.Request, userID string) *gorm.DB {
	return m.db.WithContext(r.Context()).
		Table("user_movies").
		Select("movies.id AS id, movies.title AS title, movies.release_year AS release_year, movies.director AS director, user_movies.status AS status, user_movies.rating AS rating, user_movies.notes AS notes, user_movies.created_at AS created_at, user_movies.updated_at AS updated_at").
		Joins("JOIN movies ON movies.id = user_movies.movie_id").
		Where("user_movies.user_id = ?", idgen.Arg(userID))
}

func (m *Module) listUserMovies(w http.ResponseWriter, r *http.Request) {
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

	query := m.userMovieQuery(r, user.ID)

	if statusFilter != "" {
		query = query.Where("user_movies.status = ?", statusFilter)
	}

	if qParam != "" {
		pattern := "%" + strings.ToLower(qParam) + "%"
		query = query.Where("(LOWER(movies.title) LIKE ? OR LOWER(movies.director) LIKE ? OR LOWER(user_movies.notes) LIKE ?)", pattern, pattern, pattern)
	}

	validSortColumns := map[string]string{
		"id":           "movies.id",
		"title":        "movies.title",
		"release_year": "movies.release_year",
		"director":     "movies.director",
		"status":       "user_movies.status",
		"rating":       "user_movies.rating",
		"created_at":   "user_movies.created_at",
		"updated_at":   "user_movies.updated_at",
	}

	sortCol, valid := validSortColumns[sortByParam]
	if !valid {
		sortCol = "user_movies.updated_at"
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
			respondError(w, http.StatusInternalServerError, "Failed to count list movies: "+err.Error())
			return
		}
	}

	query = query.Order(sortCol + " " + strings.ToUpper(orderParam))

	if isPaginated {
		offset := (page - 1) * limit
		query = query.Offset(offset).Limit(limit)
	}

	items := make([]MovieListItem, 0)
	if err := query.Scan(&items).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to fetch list movies: "+err.Error())
		return
	}

	if isPaginated {
		totalPages := (int(total) + limit - 1) / limit
		if totalPages < 0 {
			totalPages = 0
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"data": items,
			"pagination": map[string]interface{}{
				"total":       total,
				"page":        page,
				"limit":       limit,
				"total_pages": totalPages,
			},
		})
		return
	}

	respondJSON(w, http.StatusOK, items)
}

func (m *Module) addMovieToList(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Rating int    `json:"rating"`
		Notes  string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	id, ok := idgen.Parse(req.ID)
	if !ok {
		respondError(w, http.StatusBadRequest, "Invalid movie ID")
		return
	}

	var movie Movie
	err := m.db.WithContext(r.Context()).Where("id = ?", idgen.Arg(id)).First(&movie).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respondError(w, http.StatusNotFound, "Movie not found")
		return
	} else if err != nil {
		respondError(w, http.StatusInternalServerError, "Database query error: "+err.Error())
		return
	}

	if req.Status == "" {
		req.Status = "plan_to_watch"
	}

	now := time.Now()
	link := UserMovie{
		UserID:    user.ID,
		MovieID:   id,
		Status:    req.Status,
		Rating:    req.Rating,
		Notes:     req.Notes,
		CreatedAt: now,
		UpdatedAt: now,
	}

	err = m.db.WithContext(r.Context()).Where("user_id = ? AND movie_id = ?", idgen.Arg(user.ID), idgen.Arg(id)).First(&UserMovie{}).Error
	if err == nil {
		respondError(w, http.StatusConflict, "Movie is already on your list")
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		respondError(w, http.StatusInternalServerError, "Database query error: "+err.Error())
		return
	}

	if err := m.db.WithContext(r.Context()).Create(&link).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to add movie to list: "+err.Error())
		return
	}

	m.respondUserMovie(w, r, user.ID, id, http.StatusCreated)
}

func (m *Module) getUserMovie(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	id, ok := idgen.Parse(chi.URLParam(r, "id"))
	if !ok {
		respondError(w, http.StatusBadRequest, "Invalid movie ID")
		return
	}

	m.respondUserMovie(w, r, user.ID, id, http.StatusOK)
}

func (m *Module) updateUserMovie(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	id, ok := idgen.Parse(chi.URLParam(r, "id"))
	if !ok {
		respondError(w, http.StatusBadRequest, "Invalid movie ID")
		return
	}

	var req struct {
		Status string `json:"status"`
		Rating int    `json:"rating"`
		Notes  string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	now := time.Now()
	res := m.db.WithContext(r.Context()).Model(&UserMovie{}).
		Where("user_id = ? AND movie_id = ?", idgen.Arg(user.ID), idgen.Arg(id)).
		Updates(map[string]interface{}{
			"status":     req.Status,
			"rating":     req.Rating,
			"notes":      req.Notes,
			"updated_at": now,
		})
	if res.Error != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update list movie: "+res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		respondError(w, http.StatusNotFound, "Movie is not on your list")
		return
	}

	m.respondUserMovie(w, r, user.ID, id, http.StatusOK)
}

func (m *Module) removeMovieFromList(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	id, ok := idgen.Parse(chi.URLParam(r, "id"))
	if !ok {
		respondError(w, http.StatusBadRequest, "Invalid movie ID")
		return
	}

	res := m.db.WithContext(r.Context()).Where("user_id = ? AND movie_id = ?", idgen.Arg(user.ID), idgen.Arg(id)).Delete(&UserMovie{})
	if res.Error != nil {
		respondError(w, http.StatusInternalServerError, "Failed to remove movie from list: "+res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		respondError(w, http.StatusNotFound, "Movie is not on your list")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Movie removed from your list"})
}

func (m *Module) bulkRemoveFromList(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		respondError(w, http.StatusBadRequest, "Invalid request body: ids array required")
		return
	}

	res := m.db.WithContext(r.Context()).Where("user_id = ? AND movie_id IN ?", idgen.Arg(user.ID), idgen.Args(req.IDs)).Delete(&UserMovie{})
	if res.Error != nil {
		respondError(w, http.StatusInternalServerError, "Failed to bulk remove movies from list: "+res.Error.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "Movies removed from your list",
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
		IDs    []string `json:"ids"`
		Status string   `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 || strings.TrimSpace(req.Status) == "" {
		respondError(w, http.StatusBadRequest, "Invalid request body: ids array and status required")
		return
	}

	now := time.Now()
	res := m.db.WithContext(r.Context()).Model(&UserMovie{}).
		Where("user_id = ? AND movie_id IN ?", idgen.Arg(user.ID), idgen.Args(req.IDs)).
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

func (m *Module) respondUserMovie(w http.ResponseWriter, r *http.Request, userID string, movieID string, status int) {
	var item MovieListItem
	err := m.userMovieQuery(r, userID).Where("user_movies.movie_id = ?", idgen.Arg(movieID)).Scan(&item).Error
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database query error: "+err.Error())
		return
	}
	if item.ID == "" {
		respondError(w, http.StatusNotFound, "Movie is not on your list")
		return
	}
	respondJSON(w, status, item)
}

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
