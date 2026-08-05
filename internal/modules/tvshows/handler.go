package tvshows

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

// Module implements core.CategoryModule for TV Shows.
type Module struct {
	db *gorm.DB
}

// NewModule initializes a new TV Shows CategoryModule.
func NewModule(db *gorm.DB) *Module {
	return &Module{db: db}
}

func (m *Module) Info() core.CategoryInfo {
	return core.CategoryInfo{
		Category:    "tv_shows",
		DisplayName: "TV Shows",
		Description: "Track TV series progress, seasons, and episodes",
		Endpoint:    "/api/v1/tvshows",
	}
}

func (m *Module) RegisterRoutes(r chi.Router, authMw func(http.Handler) http.Handler) {
	r.Route("/api/v1/tvshows", func(sub chi.Router) {
		sub.Use(authMw)

		sub.Get("/", m.listTVShows)
		sub.Post("/", m.createTVShow)
		sub.Post("/bulk-delete", m.bulkDeleteTVShows)
		sub.Post("/bulk-status", m.bulkStatusTVShows)
		sub.Get("/{id}", m.getTVShow)
		sub.Put("/{id}", m.updateTVShow)
		sub.Delete("/{id}", m.deleteTVShow)
	})
}

func (m *Module) listTVShows(w http.ResponseWriter, r *http.Request) {
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

	query := m.db.WithContext(r.Context()).Model(&TVShow{}).Where("user_id = ?", user.ID)

	if statusFilter != "" {
		query = query.Where("status = ?", statusFilter)
	}

	if qParam != "" {
		pattern := "%" + strings.ToLower(qParam) + "%"
		query = query.Where("(LOWER(title) LIKE ? OR LOWER(notes) LIKE ?)", pattern, pattern)
	}

	validSortColumns := map[string]string{
		"id":              "id",
		"title":           "title",
		"current_season":  "current_season",
		"current_episode": "current_episode",
		"total_episodes":  "total_episodes",
		"status":          "status",
		"rating":          "rating",
		"created_at":      "created_at",
		"updated_at":      "updated_at",
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
			respondError(w, http.StatusInternalServerError, "Failed to count TV shows: "+err.Error())
			return
		}
	}

	query = query.Order(sortCol + " " + strings.ToUpper(orderParam))

	if isPaginated {
		offset := (page - 1) * limit
		query = query.Offset(offset).Limit(limit)
	}

	shows := make([]TVShow, 0)
	if err := query.Find(&shows).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to fetch TV shows: "+err.Error())
		return
	}

	if isPaginated {
		totalPages := (int(total) + limit - 1) / limit
		if totalPages < 0 {
			totalPages = 0
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"data": shows,
			"pagination": map[string]interface{}{
				"total":       total,
				"page":        page,
				"limit":       limit,
				"total_pages": totalPages,
			},
		})
		return
	}

	respondJSON(w, http.StatusOK, shows)
}

func (m *Module) bulkDeleteTVShows(w http.ResponseWriter, r *http.Request) {
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

	res := m.db.WithContext(r.Context()).Where("user_id = ? AND id IN ?", user.ID, req.IDs).Delete(&TVShow{})
	if res.Error != nil {
		respondError(w, http.StatusInternalServerError, "Failed to bulk delete TV shows: "+res.Error.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "TV shows deleted successfully",
		"deleted_count": res.RowsAffected,
	})
}

func (m *Module) bulkStatusTVShows(w http.ResponseWriter, r *http.Request) {
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
	res := m.db.WithContext(r.Context()).Model(&TVShow{}).
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
		"message":       "TV shows status updated successfully",
		"updated_count": res.RowsAffected,
	})
}

func (m *Module) createTVShow(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		Title          string `json:"title"`
		CurrentSeason  int    `json:"current_season"`
		CurrentEpisode int    `json:"current_episode"`
		TotalEpisodes  int    `json:"total_episodes"`
		Status         string `json:"status"`
		Rating         int    `json:"rating"`
		Notes          string `json:"notes"`
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

	if req.CurrentSeason <= 0 {
		req.CurrentSeason = 1
	}
	if req.Status == "" {
		req.Status = "watching"
	}

	now := time.Now()
	show := TVShow{
		UserID:         user.ID,
		Title:          req.Title,
		CurrentSeason:  req.CurrentSeason,
		CurrentEpisode: req.CurrentEpisode,
		TotalEpisodes:  req.TotalEpisodes,
		Status:         req.Status,
		Rating:         req.Rating,
		Notes:          req.Notes,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := m.db.WithContext(r.Context()).Create(&show).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to insert TV show: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, show)
}

func (m *Module) getTVShow(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid TV show ID")
		return
	}

	var show TVShow
	err = m.db.WithContext(r.Context()).Where("id = ? AND user_id = ?", id, user.ID).First(&show).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respondError(w, http.StatusNotFound, "TV show not found")
		return
	} else if err != nil {
		respondError(w, http.StatusInternalServerError, "Database query error: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, show)
}

func (m *Module) updateTVShow(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid TV show ID")
		return
	}

	var req struct {
		Title          string `json:"title"`
		CurrentSeason  int    `json:"current_season"`
		CurrentEpisode int    `json:"current_episode"`
		TotalEpisodes  int    `json:"total_episodes"`
		Status         string `json:"status"`
		Rating         int    `json:"rating"`
		Notes          string `json:"notes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	now := time.Now()
	res := m.db.WithContext(r.Context()).Model(&TVShow{}).
		Where("id = ? AND user_id = ?", id, user.ID).
		Updates(map[string]interface{}{
			"title":           req.Title,
			"current_season":  req.CurrentSeason,
			"current_episode": req.CurrentEpisode,
			"total_episodes":  req.TotalEpisodes,
			"status":          req.Status,
			"rating":          req.Rating,
			"notes":           req.Notes,
			"updated_at":      now,
		})

	if res.Error != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update TV show: "+res.Error.Error())
		return
	}

	if res.RowsAffected == 0 {
		respondError(w, http.StatusNotFound, "TV show not found")
		return
	}

	m.getTVShow(w, r)
}

func (m *Module) deleteTVShow(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid TV show ID")
		return
	}

	res := m.db.WithContext(r.Context()).Where("id = ? AND user_id = ?", id, user.ID).Delete(&TVShow{})
	if res.Error != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete TV show: "+res.Error.Error())
		return
	}

	if res.RowsAffected == 0 {
		respondError(w, http.StatusNotFound, "TV show not found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "TV show deleted successfully"})
}

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

