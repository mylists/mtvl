package services

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"mtvl/internal/auth"
	"mtvl/internal/modules/books"
	"mtvl/internal/modules/movies"
	"mtvl/internal/modules/tvshows"
)

type ServiceHandler struct {
	db *gorm.DB
}

func NewServiceHandler(db *gorm.DB) *ServiceHandler {
	return &ServiceHandler{db: db}
}

func (s *ServiceHandler) RegisterRoutes(r chi.Router, authMw func(http.Handler) http.Handler) {
	r.Group(func(sub chi.Router) {
		sub.Use(authMw)

		sub.Get("/api/v1/stats", s.GetStats)
		sub.Get("/api/v1/search", s.GlobalSearch)
		sub.Get("/api/v1/export", s.ExportUserData)
		sub.Post("/api/v1/import", s.ImportUserData)
	})
}

type StatResult struct {
	Count int64   `gorm:"column:count"`
	Avg   float64 `gorm:"column:avg"`
}

type StatusCount struct {
	Status string `gorm:"column:status"`
	Count  int    `gorm:"column:count"`
}

// GetStats returns aggregated dashboard statistics across all tracking categories.
func (s *ServiceHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	ctx := r.Context()

	// Movies stats
	var movieStat StatResult
	_ = s.db.WithContext(ctx).Model(&movies.Movie{}).Where("user_id = ?", user.ID).Select("COUNT(*) as count, COALESCE(AVG(rating), 0) as avg").Scan(&movieStat)

	movieStatusBreakdown := make(map[string]int)
	var movieCounts []StatusCount
	if err := s.db.WithContext(ctx).Model(&movies.Movie{}).Where("user_id = ?", user.ID).Select("status, COUNT(*) as count").Group("status").Find(&movieCounts).Error; err == nil {
		for _, c := range movieCounts {
			movieStatusBreakdown[c.Status] = c.Count
		}
	}

	// TV Shows stats
	var tvStat StatResult
	_ = s.db.WithContext(ctx).Model(&tvshows.TVShow{}).Where("user_id = ?", user.ID).Select("COUNT(*) as count, COALESCE(AVG(rating), 0) as avg").Scan(&tvStat)

	tvStatusBreakdown := make(map[string]int)
	var tvCounts []StatusCount
	if err := s.db.WithContext(ctx).Model(&tvshows.TVShow{}).Where("user_id = ?", user.ID).Select("status, COUNT(*) as count").Group("status").Find(&tvCounts).Error; err == nil {
		for _, c := range tvCounts {
			tvStatusBreakdown[c.Status] = c.Count
		}
	}

	// Books stats
	var bookStat StatResult
	_ = s.db.WithContext(ctx).Model(&books.Book{}).Where("user_id = ?", user.ID).Select("COUNT(*) as count, COALESCE(AVG(rating), 0) as avg").Scan(&bookStat)

	bookStatusBreakdown := make(map[string]int)
	var bookCounts []StatusCount
	if err := s.db.WithContext(ctx).Model(&books.Book{}).Where("user_id = ?", user.ID).Select("status, COUNT(*) as count").Group("status").Find(&bookCounts).Error; err == nil {
		for _, c := range bookCounts {
			bookStatusBreakdown[c.Status] = c.Count
		}
	}

	totalItems := movieStat.Count + tvStat.Count + bookStat.Count

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total_items": totalItems,
		"categories": map[string]interface{}{
			"movies": map[string]interface{}{
				"total":            movieStat.Count,
				"average_rating":   movieStat.Avg,
				"status_breakdown": movieStatusBreakdown,
			},
			"tv_shows": map[string]interface{}{
				"total":            tvStat.Count,
				"average_rating":   tvStat.Avg,
				"status_breakdown": tvStatusBreakdown,
			},
			"books": map[string]interface{}{
				"total":            bookStat.Count,
				"average_rating":   bookStat.Avg,
				"status_breakdown": bookStatusBreakdown,
			},
		},
	})
}

// GlobalSearch searches across movies, tv shows, and books for a keyword.
func (s *ServiceHandler) GlobalSearch(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	queryTerm := strings.TrimSpace(r.URL.Query().Get("q"))
	if queryTerm == "" {
		respondError(w, http.StatusBadRequest, "Query parameter 'q' is required")
		return
	}

	pattern := "%" + strings.ToLower(queryTerm) + "%"
	ctx := r.Context()

	// Search movies
	movieResults := make([]movies.Movie, 0)
	_ = s.db.WithContext(ctx).Where("user_id = ? AND (LOWER(title) LIKE ? OR LOWER(director) LIKE ? OR LOWER(notes) LIKE ?)", user.ID, pattern, pattern, pattern).Find(&movieResults).Error

	// Search tv shows
	tvResults := make([]tvshows.TVShow, 0)
	_ = s.db.WithContext(ctx).Where("user_id = ? AND (LOWER(title) LIKE ? OR LOWER(notes) LIKE ?)", user.ID, pattern, pattern).Find(&tvResults).Error

	// Search books
	bookResults := make([]books.Book, 0)
	_ = s.db.WithContext(ctx).Where("user_id = ? AND (LOWER(title) LIKE ? OR LOWER(notes) LIKE ?)", user.ID, pattern, pattern).Find(&bookResults).Error

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"query": queryTerm,
		"results": map[string]interface{}{
			"movies":   movieResults,
			"tv_shows": tvResults,
			"books":    bookResults,
		},
		"total_matches": len(movieResults) + len(tvResults) + len(bookResults),
	})
}

// ExportUserData exports all tracking data for the authenticated user as a structured JSON backup.
func (s *ServiceHandler) ExportUserData(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	ctx := r.Context()

	// Movies
	movieList := make([]movies.Movie, 0)
	_ = s.db.WithContext(ctx).Where("user_id = ?", user.ID).Order("id ASC").Find(&movieList).Error

	// TV Shows
	tvList := make([]tvshows.TVShow, 0)
	_ = s.db.WithContext(ctx).Where("user_id = ?", user.ID).Order("id ASC").Find(&tvList).Error

	// Books
	bookList := make([]books.Book, 0)
	_ = s.db.WithContext(ctx).Where("user_id = ?", user.ID).Order("id ASC").Find(&bookList).Error

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"version":     "1.0",
		"exported_at": time.Now(),
		"user":        user,
		"data": map[string]interface{}{
			"movies":   movieList,
			"tv_shows": tvList,
			"books":    bookList,
		},
	})
}

// ImportUserData imports tracking data into the user's account.
func (s *ServiceHandler) ImportUserData(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		Overwrite bool `json:"overwrite"`
		Data      struct {
			Movies []struct {
				Title       string `json:"title"`
				ReleaseYear int    `json:"release_year"`
				Director    string `json:"director"`
				Status      string `json:"status"`
				Rating      int    `json:"rating"`
				Notes       string `json:"notes"`
			} `json:"movies"`
			TVShows []struct {
				Title          string `json:"title"`
				CurrentSeason  int    `json:"current_season"`
				CurrentEpisode int    `json:"current_episode"`
				TotalEpisodes  int    `json:"total_episodes"`
				Status         string `json:"status"`
				Rating         int    `json:"rating"`
				Notes          string `json:"notes"`
			} `json:"tv_shows"`
			Books []struct {
				Title  string `json:"title"`
				Status string `json:"status"`
				Rating int    `json:"rating"`
				Notes  string `json:"notes"`
			} `json:"books"`
		} `json:"data"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload for import")
		return
	}

	ctx := r.Context()
	var importedMovies, importedTVShows, importedBooks int

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if req.Overwrite {
			tx.Where("user_id = ?", user.ID).Delete(&movies.Movie{})
			tx.Where("user_id = ?", user.ID).Delete(&tvshows.TVShow{})
			tx.Where("user_id = ?", user.ID).Delete(&books.Book{})
		}

		now := time.Now()

		for _, mov := range req.Data.Movies {
			if strings.TrimSpace(mov.Title) == "" {
				continue
			}
			status := mov.Status
			if status == "" {
				status = "plan_to_watch"
			}
			m := movies.Movie{
				UserID:      user.ID,
				Title:       mov.Title,
				ReleaseYear: mov.ReleaseYear,
				Director:    mov.Director,
				Status:      status,
				Rating:      mov.Rating,
				Notes:       mov.Notes,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			if err := tx.Create(&m).Error; err == nil {
				importedMovies++
			}
		}

		for _, show := range req.Data.TVShows {
			if strings.TrimSpace(show.Title) == "" {
				continue
			}
			status := show.Status
			if status == "" {
				status = "watching"
			}
			season := show.CurrentSeason
			if season <= 0 {
				season = 1
			}
			t := tvshows.TVShow{
				UserID:         user.ID,
				Title:          show.Title,
				CurrentSeason:  season,
				CurrentEpisode: show.CurrentEpisode,
				TotalEpisodes:  show.TotalEpisodes,
				Status:         status,
				Rating:         show.Rating,
				Notes:          show.Notes,
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			if err := tx.Create(&t).Error; err == nil {
				importedTVShows++
			}
		}

		for _, book := range req.Data.Books {
			if strings.TrimSpace(book.Title) == "" {
				continue
			}
			status := book.Status
			if status == "" {
				status = "plan_to_read"
			}
			b := books.Book{
				UserID:    user.ID,
				Title:     book.Title,
				Status:    status,
				Rating:    book.Rating,
				Notes:     book.Notes,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := tx.Create(&b).Error; err == nil {
				importedBooks++
			}
		}

		return nil
	})

	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to import data: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Data imported successfully",
		"imported": map[string]int{
			"movies":   importedMovies,
			"tv_shows": importedTVShows,
			"books":    importedBooks,
		},
	})
}

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

