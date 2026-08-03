package tvshows

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

// Module implements core.CategoryModule for TV Shows.
type Module struct {
	db *sql.DB
}

// NewModule initializes a new TV Shows CategoryModule.
func NewModule(db *sql.DB) *Module {
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

	whereClauses := []string{"user_id = ?"}
	args := []interface{}{user.ID}

	if statusFilter != "" {
		whereClauses = append(whereClauses, "status = ?")
		args = append(args, statusFilter)
	}

	if qParam != "" {
		whereClauses = append(whereClauses, "(LOWER(title) LIKE ? OR LOWER(notes) LIKE ?)")
		pattern := "%" + strings.ToLower(qParam) + "%"
		args = append(args, pattern, pattern)
	}

	whereStmt := strings.Join(whereClauses, " AND ")

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

	var total int
	if isPaginated {
		countQuery := "SELECT COUNT(*) FROM tv_shows WHERE " + whereStmt
		_ = m.db.QueryRowContext(r.Context(), countQuery, args...).Scan(&total)
	}

	query := "SELECT id, user_id, title, current_season, current_episode, total_episodes, status, rating, notes, created_at, updated_at FROM tv_shows WHERE " + whereStmt + " ORDER BY " + sortCol + " " + strings.ToUpper(orderParam)

	if isPaginated {
		offset := (page - 1) * limit
		query += " LIMIT " + strconv.Itoa(limit) + " OFFSET " + strconv.Itoa(offset)
	}

	rows, err := m.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to fetch TV shows: "+err.Error())
		return
	}
	defer rows.Close()

	shows := make([]TVShow, 0)
	for rows.Next() {
		var show TVShow
		if err := rows.Scan(&show.ID, &show.UserID, &show.Title, &show.CurrentSeason, &show.CurrentEpisode, &show.TotalEpisodes, &show.Status, &show.Rating, &show.Notes, &show.CreatedAt, &show.UpdatedAt); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to scan TV show: "+err.Error())
			return
		}
		shows = append(shows, show)
	}

	if isPaginated {
		totalPages := (total + limit - 1) / limit
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

	placeholders := make([]string, len(req.IDs))
	args := make([]interface{}, 0, len(req.IDs)+1)
	args = append(args, user.ID)
	for i, id := range req.IDs {
		placeholders[i] = "?"
		args = append(args, id)
	}

	query := "DELETE FROM tv_shows WHERE user_id = ? AND id IN (" + strings.Join(placeholders, ",") + ")"
	res, err := m.db.ExecContext(r.Context(), query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to bulk delete TV shows: "+err.Error())
		return
	}

	rowsAffected, _ := res.RowsAffected()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "TV shows deleted successfully",
		"deleted_count": rowsAffected,
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

	placeholders := make([]string, len(req.IDs))
	now := time.Now()
	args := make([]interface{}, 0, len(req.IDs)+3)
	args = append(args, req.Status, now, user.ID)
	for i, id := range req.IDs {
		placeholders[i] = "?"
		args = append(args, id)
	}

	query := "UPDATE tv_shows SET status = ?, updated_at = ? WHERE user_id = ? AND id IN (" + strings.Join(placeholders, ",") + ")"
	res, err := m.db.ExecContext(r.Context(), query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to bulk update status: "+err.Error())
		return
	}

	rowsAffected, _ := res.RowsAffected()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "TV shows status updated successfully",
		"updated_count": rowsAffected,
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
	query := `INSERT INTO tv_shows (user_id, title, current_season, current_episode, total_episodes, status, rating, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := m.db.ExecContext(r.Context(), query, user.ID, req.Title, req.CurrentSeason, req.CurrentEpisode, req.TotalEpisodes, req.Status, req.Rating, req.Notes, now, now)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to insert TV show: "+err.Error())
		return
	}

	id, _ := res.LastInsertId()
	show := TVShow{
		ID:             id,
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
	query := `SELECT id, user_id, title, current_season, current_episode, total_episodes, status, rating, notes, created_at, updated_at FROM tv_shows WHERE id = ? AND user_id = ?`
	err = m.db.QueryRowContext(r.Context(), query, id, user.ID).Scan(&show.ID, &show.UserID, &show.Title, &show.CurrentSeason, &show.CurrentEpisode, &show.TotalEpisodes, &show.Status, &show.Rating, &show.Notes, &show.CreatedAt, &show.UpdatedAt)
	if err == sql.ErrNoRows {
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
	query := `UPDATE tv_shows SET title = ?, current_season = ?, current_episode = ?, total_episodes = ?, status = ?, rating = ?, notes = ?, updated_at = ? WHERE id = ? AND user_id = ?`
	res, err := m.db.ExecContext(r.Context(), query, req.Title, req.CurrentSeason, req.CurrentEpisode, req.TotalEpisodes, req.Status, req.Rating, req.Notes, now, id, user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update TV show: "+err.Error())
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
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

	query := `DELETE FROM tv_shows WHERE id = ? AND user_id = ?`
	res, err := m.db.ExecContext(r.Context(), query, id, user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete TV show: "+err.Error())
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
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
