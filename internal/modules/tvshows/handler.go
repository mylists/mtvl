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

	statusFilter := r.URL.Query().Get("status")

	query := `SELECT id, user_id, title, current_season, current_episode, total_episodes, status, rating, notes, created_at, updated_at FROM tv_shows WHERE user_id = ?`
	args := []interface{}{user.ID}

	if statusFilter != "" {
		query += ` AND status = ?`
		args = append(args, statusFilter)
	}

	query += ` ORDER BY updated_at DESC`

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

	respondJSON(w, http.StatusOK, shows)
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
