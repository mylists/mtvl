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
	"mtvl/internal/idgen"
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
		sub.Get("/", m.listTVShows)
		sub.Get("/{id}", m.getTVShow)

		sub.Group(func(protected chi.Router) {
			protected.Use(authMw)
			protected.Post("/", m.createTVShow)
			protected.Post("/bulk-delete", m.bulkDeleteTVShows)
			protected.Get("/list", m.listUserTVShows)
			protected.Post("/list", m.addTVShowToList)
			protected.Post("/list/bulk-delete", m.bulkRemoveFromList)
			protected.Post("/list/bulk-status", m.bulkStatusTVShows)
			protected.Post("/bulk-status", m.bulkStatusTVShows)
			protected.Get("/list/{id}", m.getUserTVShow)
			protected.Put("/list/{id}", m.updateUserTVShow)
			protected.Delete("/list/{id}", m.removeTVShowFromList)
			protected.Put("/{id}", m.updateTVShow)
			protected.Delete("/{id}", m.deleteTVShow)
		})
	})
}

func (m *Module) listTVShows(w http.ResponseWriter, r *http.Request) {
	qParam := strings.TrimSpace(r.URL.Query().Get("q"))
	sortByParam := strings.TrimSpace(r.URL.Query().Get("sort_by"))
	orderParam := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("order")))
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	query := m.db.WithContext(r.Context()).Model(&TVShow{})

	if qParam != "" {
		pattern := "%" + strings.ToLower(qParam) + "%"
		query = query.Where("LOWER(title) LIKE ?", pattern)
	}

	validSortColumns := map[string]string{
		"id":             "id",
		"title":          "title",
		"total_episodes": "total_episodes",
		"created_at":     "created_at",
		"updated_at":     "updated_at",
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
		if err := tx.Where("tv_show_id IN ?", idgen.Args(req.IDs)).Delete(&UserTVShow{}).Error; err != nil {
			return err
		}
		res := tx.Where("id IN ?", idgen.Args(req.IDs)).Delete(&TVShow{})
		deletedCount = res.RowsAffected
		return res.Error
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to bulk delete TV shows: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "TV shows deleted successfully",
		"deleted_count": deletedCount,
	})
}

func (m *Module) createTVShow(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.GetUserFromContext(r.Context()); !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		Title         string `json:"title"`
		TotalEpisodes int    `json:"total_episodes"`
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

	var existing TVShow
	if err := m.db.WithContext(r.Context()).Where("LOWER(title) = ?", strings.ToLower(req.Title)).First(&existing).Error; err == nil {
		respondError(w, http.StatusConflict, "A TV show with this title already exists")
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		respondError(w, http.StatusInternalServerError, "Failed to check existing TV show: "+err.Error())
		return
	}

	now := time.Now()
	show := TVShow{
		Title:         req.Title,
		TotalEpisodes: req.TotalEpisodes,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := m.db.WithContext(r.Context()).Create(&show).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			respondError(w, http.StatusConflict, "A TV show with this title already exists")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to insert TV show: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, show)
}

func (m *Module) getTVShow(w http.ResponseWriter, r *http.Request) {
	id, ok := idgen.Parse(chi.URLParam(r, "id"))
	if !ok {
		respondError(w, http.StatusBadRequest, "Invalid TV show ID")
		return
	}

	var show TVShow
	err := m.db.WithContext(r.Context()).Where("id = ?", idgen.Arg(id)).First(&show).Error
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
	if _, ok := auth.GetUserFromContext(r.Context()); !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	id, ok := idgen.Parse(chi.URLParam(r, "id"))
	if !ok {
		respondError(w, http.StatusBadRequest, "Invalid TV show ID")
		return
	}

	var req struct {
		Title         string `json:"title"`
		TotalEpisodes int    `json:"total_episodes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	now := time.Now()
	res := m.db.WithContext(r.Context()).Model(&TVShow{}).
		Where("id = ?", idgen.Arg(id)).
		Updates(map[string]interface{}{
			"title":          req.Title,
			"total_episodes": req.TotalEpisodes,
			"updated_at":     now,
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
	if _, ok := auth.GetUserFromContext(r.Context()); !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	id, ok := idgen.Parse(chi.URLParam(r, "id"))
	if !ok {
		respondError(w, http.StatusBadRequest, "Invalid TV show ID")
		return
	}

	var deleted int64
	err := m.db.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tv_show_id = ?", idgen.Arg(id)).Delete(&UserTVShow{}).Error; err != nil {
			return err
		}
		res := tx.Where("id = ?", idgen.Arg(id)).Delete(&TVShow{})
		deleted = res.RowsAffected
		return res.Error
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete TV show: "+err.Error())
		return
	}

	if deleted == 0 {
		respondError(w, http.StatusNotFound, "TV show not found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "TV show deleted successfully"})
}

func (m *Module) userTVShowQuery(r *http.Request, userID string) *gorm.DB {
	return m.db.WithContext(r.Context()).
		Table("user_tv_shows").
		Select("tv_shows.id AS id, tv_shows.title AS title, tv_shows.total_episodes AS total_episodes, user_tv_shows.current_season AS current_season, user_tv_shows.current_episode AS current_episode, user_tv_shows.status AS status, user_tv_shows.rating AS rating, user_tv_shows.notes AS notes, user_tv_shows.created_at AS created_at, user_tv_shows.updated_at AS updated_at").
		Joins("JOIN tv_shows ON tv_shows.id = user_tv_shows.tv_show_id").
		Where("user_tv_shows.user_id = ?", idgen.Arg(userID))
}

func (m *Module) listUserTVShows(w http.ResponseWriter, r *http.Request) {
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

	query := m.userTVShowQuery(r, user.ID)

	if statusFilter != "" {
		query = query.Where("user_tv_shows.status = ?", statusFilter)
	}

	if qParam != "" {
		pattern := "%" + strings.ToLower(qParam) + "%"
		query = query.Where("(LOWER(tv_shows.title) LIKE ? OR LOWER(user_tv_shows.notes) LIKE ?)", pattern, pattern)
	}

	validSortColumns := map[string]string{
		"id":              "tv_shows.id",
		"title":           "tv_shows.title",
		"total_episodes":  "tv_shows.total_episodes",
		"current_season":  "user_tv_shows.current_season",
		"current_episode": "user_tv_shows.current_episode",
		"status":          "user_tv_shows.status",
		"rating":          "user_tv_shows.rating",
		"created_at":      "user_tv_shows.created_at",
		"updated_at":      "user_tv_shows.updated_at",
	}

	sortCol, valid := validSortColumns[sortByParam]
	if !valid {
		sortCol = "user_tv_shows.updated_at"
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
			respondError(w, http.StatusInternalServerError, "Failed to count list TV shows: "+err.Error())
			return
		}
	}

	query = query.Order(sortCol + " " + strings.ToUpper(orderParam))

	if isPaginated {
		offset := (page - 1) * limit
		query = query.Offset(offset).Limit(limit)
	}

	items := make([]TVShowListItem, 0)
	if err := query.Scan(&items).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to fetch list TV shows: "+err.Error())
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

func (m *Module) addTVShowToList(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		ID             string `json:"id"`
		CurrentSeason  int    `json:"current_season"`
		CurrentEpisode int    `json:"current_episode"`
		Status         string `json:"status"`
		Rating         int    `json:"rating"`
		Notes          string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	id, ok := idgen.Parse(req.ID)
	if !ok {
		respondError(w, http.StatusBadRequest, "Invalid TV show ID")
		return
	}

	var show TVShow
	err := m.db.WithContext(r.Context()).Where("id = ?", idgen.Arg(id)).First(&show).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respondError(w, http.StatusNotFound, "TV show not found")
		return
	} else if err != nil {
		respondError(w, http.StatusInternalServerError, "Database query error: "+err.Error())
		return
	}

	if req.CurrentSeason <= 0 {
		req.CurrentSeason = 1
	}
	if req.Status == "" {
		req.Status = "watching"
	}

	now := time.Now()
	link := UserTVShow{
		UserID:         user.ID,
		TVShowID:       id,
		CurrentSeason:  req.CurrentSeason,
		CurrentEpisode: req.CurrentEpisode,
		Status:         req.Status,
		Rating:         req.Rating,
		Notes:          req.Notes,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	err = m.db.WithContext(r.Context()).Where("user_id = ? AND tv_show_id = ?", idgen.Arg(user.ID), idgen.Arg(id)).First(&UserTVShow{}).Error
	if err == nil {
		respondError(w, http.StatusConflict, "TV show is already on your list")
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		respondError(w, http.StatusInternalServerError, "Database query error: "+err.Error())
		return
	}

	if err := m.db.WithContext(r.Context()).Create(&link).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to add TV show to list: "+err.Error())
		return
	}

	m.respondUserTVShow(w, r, user.ID, id, http.StatusCreated)
}

func (m *Module) getUserTVShow(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	id, ok := idgen.Parse(chi.URLParam(r, "id"))
	if !ok {
		respondError(w, http.StatusBadRequest, "Invalid TV show ID")
		return
	}

	m.respondUserTVShow(w, r, user.ID, id, http.StatusOK)
}

func (m *Module) updateUserTVShow(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	id, ok := idgen.Parse(chi.URLParam(r, "id"))
	if !ok {
		respondError(w, http.StatusBadRequest, "Invalid TV show ID")
		return
	}

	var req struct {
		CurrentSeason  int    `json:"current_season"`
		CurrentEpisode int    `json:"current_episode"`
		Status         string `json:"status"`
		Rating         int    `json:"rating"`
		Notes          string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	now := time.Now()
	res := m.db.WithContext(r.Context()).Model(&UserTVShow{}).
		Where("user_id = ? AND tv_show_id = ?", idgen.Arg(user.ID), idgen.Arg(id)).
		Updates(map[string]interface{}{
			"current_season":  req.CurrentSeason,
			"current_episode": req.CurrentEpisode,
			"status":          req.Status,
			"rating":          req.Rating,
			"notes":           req.Notes,
			"updated_at":      now,
		})
	if res.Error != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update list TV show: "+res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		respondError(w, http.StatusNotFound, "TV show is not on your list")
		return
	}

	m.respondUserTVShow(w, r, user.ID, id, http.StatusOK)
}

func (m *Module) removeTVShowFromList(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	id, ok := idgen.Parse(chi.URLParam(r, "id"))
	if !ok {
		respondError(w, http.StatusBadRequest, "Invalid TV show ID")
		return
	}

	res := m.db.WithContext(r.Context()).Where("user_id = ? AND tv_show_id = ?", idgen.Arg(user.ID), idgen.Arg(id)).Delete(&UserTVShow{})
	if res.Error != nil {
		respondError(w, http.StatusInternalServerError, "Failed to remove TV show from list: "+res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		respondError(w, http.StatusNotFound, "TV show is not on your list")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "TV show removed from your list"})
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

	res := m.db.WithContext(r.Context()).Where("user_id = ? AND tv_show_id IN ?", idgen.Arg(user.ID), idgen.Args(req.IDs)).Delete(&UserTVShow{})
	if res.Error != nil {
		respondError(w, http.StatusInternalServerError, "Failed to bulk remove TV shows from list: "+res.Error.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "TV shows removed from your list",
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
		IDs    []string `json:"ids"`
		Status string   `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 || strings.TrimSpace(req.Status) == "" {
		respondError(w, http.StatusBadRequest, "Invalid request body: ids array and status required")
		return
	}

	now := time.Now()
	res := m.db.WithContext(r.Context()).Model(&UserTVShow{}).
		Where("user_id = ? AND tv_show_id IN ?", idgen.Arg(user.ID), idgen.Args(req.IDs)).
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

func (m *Module) respondUserTVShow(w http.ResponseWriter, r *http.Request, userID string, showID string, status int) {
	var item TVShowListItem
	err := m.userTVShowQuery(r, userID).Where("user_tv_shows.tv_show_id = ?", idgen.Arg(showID)).Scan(&item).Error
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database query error: "+err.Error())
		return
	}
	if item.ID == "" {
		respondError(w, http.StatusNotFound, "TV show is not on your list")
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
