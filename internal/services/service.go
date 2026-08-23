package services

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"mtvl/internal/auth"
	"mtvl/internal/idgen"
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

// GetStats returns aggregated dashboard statistics for the current user's lists.
func (s *ServiceHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	ctx := r.Context()

	var movieStat StatResult
	_ = s.db.WithContext(ctx).Model(&movies.UserMovie{}).
		Where("user_id = ?", idgen.Arg(user.ID)).
		Select("COUNT(*) as count, COALESCE(AVG(rating), 0) as avg").
		Scan(&movieStat)

	movieStatusBreakdown := make(map[string]int)
	var movieCounts []StatusCount
	if err := s.db.WithContext(ctx).Model(&movies.UserMovie{}).
		Select("status, COUNT(*) as count").
		Where("user_id = ?", idgen.Arg(user.ID)).
		Group("status").
		Find(&movieCounts).Error; err == nil {
		for _, c := range movieCounts {
			movieStatusBreakdown[c.Status] = c.Count
		}
	}

	var tvStat StatResult
	_ = s.db.WithContext(ctx).Model(&tvshows.UserTVShow{}).
		Where("user_id = ?", idgen.Arg(user.ID)).
		Select("COUNT(*) as count, COALESCE(AVG(rating), 0) as avg").
		Scan(&tvStat)

	tvStatusBreakdown := make(map[string]int)
	var tvCounts []StatusCount
	if err := s.db.WithContext(ctx).Model(&tvshows.UserTVShow{}).
		Select("status, COUNT(*) as count").
		Where("user_id = ?", idgen.Arg(user.ID)).
		Group("status").
		Find(&tvCounts).Error; err == nil {
		for _, c := range tvCounts {
			tvStatusBreakdown[c.Status] = c.Count
		}
	}

	var bookStat StatResult
	_ = s.db.WithContext(ctx).Model(&books.UserBook{}).
		Where("user_id = ?", idgen.Arg(user.ID)).
		Select("COUNT(*) as count, COALESCE(AVG(rating), 0) as avg").
		Scan(&bookStat)

	bookStatusBreakdown := make(map[string]int)
	var bookCounts []StatusCount
	if err := s.db.WithContext(ctx).Model(&books.UserBook{}).
		Select("status, COUNT(*) as count").
		Where("user_id = ?", idgen.Arg(user.ID)).
		Group("status").
		Find(&bookCounts).Error; err == nil {
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

// GlobalSearch searches shared catalog titles across movies, tv shows, and books.
func (s *ServiceHandler) GlobalSearch(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.GetUserFromContext(r.Context()); !ok {
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

	movieResults := make([]movies.Movie, 0)
	_ = s.db.WithContext(ctx).Where("LOWER(title) LIKE ? OR LOWER(director) LIKE ?", pattern, pattern).Find(&movieResults).Error

	tvResults := make([]tvshows.TVShow, 0)
	_ = s.db.WithContext(ctx).Where("LOWER(title) LIKE ?", pattern).Find(&tvResults).Error

	bookResults := make([]books.Book, 0)
	_ = s.db.WithContext(ctx).Where("LOWER(title) LIKE ?", pattern).Find(&bookResults).Error

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

// ExportUserData exports the shared catalog plus the authenticated user's list links.
func (s *ServiceHandler) ExportUserData(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	ctx := r.Context()

	movieList := make([]movies.Movie, 0)
	_ = s.db.WithContext(ctx).Order("id ASC").Find(&movieList).Error

	tvList := make([]tvshows.TVShow, 0)
	_ = s.db.WithContext(ctx).Order("id ASC").Find(&tvList).Error

	bookList := make([]books.Book, 0)
	_ = s.db.WithContext(ctx).Order("id ASC").Find(&bookList).Error

	userMovies := make([]movies.UserMovie, 0)
	_ = s.db.WithContext(ctx).Where("user_id = ?", idgen.Arg(user.ID)).Order("movie_id ASC").Find(&userMovies).Error

	userTVShows := make([]tvshows.UserTVShow, 0)
	_ = s.db.WithContext(ctx).Where("user_id = ?", idgen.Arg(user.ID)).Order("tv_show_id ASC").Find(&userTVShows).Error

	userBooks := make([]books.UserBook, 0)
	_ = s.db.WithContext(ctx).Where("user_id = ?", idgen.Arg(user.ID)).Order("book_id ASC").Find(&userBooks).Error

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"version":     "1.0",
		"exported_at": time.Now(),
		"user":        user,
		"data": map[string]interface{}{
			"movies":   movieList,
			"tv_shows": tvList,
			"books":    bookList,
		},
		"lists": map[string]interface{}{
			"movies":   userMovies,
			"tv_shows": userTVShows,
			"books":    userBooks,
		},
	})
}

// ImportUserData imports catalog items and adds them to the current user's lists.
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
			if err := tx.Where("user_id = ?", idgen.Arg(user.ID)).Delete(&movies.UserMovie{}).Error; err != nil {
				return err
			}
			if err := tx.Where("user_id = ?", idgen.Arg(user.ID)).Delete(&tvshows.UserTVShow{}).Error; err != nil {
				return err
			}
			if err := tx.Where("user_id = ?", idgen.Arg(user.ID)).Delete(&books.UserBook{}).Error; err != nil {
				return err
			}
		}

		now := time.Now()

		for _, mov := range req.Data.Movies {
			title := strings.TrimSpace(mov.Title)
			if title == "" {
				continue
			}
			item, err := findOrCreateMovie(tx, title, mov.ReleaseYear, mov.Director, now)
			if err != nil {
				continue
			}
			status := mov.Status
			if status == "" {
				status = "plan_to_watch"
			}
			link := movies.UserMovie{
				UserID:    user.ID,
				MovieID:   item.ID,
				Status:    status,
				Rating:    mov.Rating,
				Notes:     mov.Notes,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := upsertUserMovie(tx, &link); err == nil {
				importedMovies++
			}
		}

		for _, show := range req.Data.TVShows {
			title := strings.TrimSpace(show.Title)
			if title == "" {
				continue
			}
			item, err := findOrCreateTVShow(tx, title, show.TotalEpisodes, now)
			if err != nil {
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
			link := tvshows.UserTVShow{
				UserID:         user.ID,
				TVShowID:       item.ID,
				CurrentSeason:  season,
				CurrentEpisode: show.CurrentEpisode,
				Status:         status,
				Rating:         show.Rating,
				Notes:          show.Notes,
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			if err := upsertUserTVShow(tx, &link); err == nil {
				importedTVShows++
			}
		}

		for _, book := range req.Data.Books {
			title := strings.TrimSpace(book.Title)
			if title == "" {
				continue
			}
			item, err := findOrCreateBook(tx, title, now)
			if err != nil {
				continue
			}
			status := book.Status
			if status == "" {
				status = "plan_to_read"
			}
			link := books.UserBook{
				UserID:    user.ID,
				BookID:    item.ID,
				Status:    status,
				Rating:    book.Rating,
				Notes:     book.Notes,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := upsertUserBook(tx, &link); err == nil {
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

func findOrCreateMovie(tx *gorm.DB, title string, year int, director string, now time.Time) (*movies.Movie, error) {
	var existing movies.Movie
	err := tx.Where("LOWER(title) = ?", strings.ToLower(title)).First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	item := movies.Movie{
		Title:       title,
		ReleaseYear: year,
		Director:    director,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := tx.Create(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func findOrCreateTVShow(tx *gorm.DB, title string, totalEpisodes int, now time.Time) (*tvshows.TVShow, error) {
	var existing tvshows.TVShow
	err := tx.Where("LOWER(title) = ?", strings.ToLower(title)).First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	item := tvshows.TVShow{
		Title:         title,
		TotalEpisodes: totalEpisodes,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := tx.Create(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func findOrCreateBook(tx *gorm.DB, title string, now time.Time) (*books.Book, error) {
	var existing books.Book
	err := tx.Where("LOWER(title) = ?", strings.ToLower(title)).First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	item := books.Book{
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := tx.Create(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func upsertUserMovie(tx *gorm.DB, link *movies.UserMovie) error {
	var existing movies.UserMovie
	err := tx.Where("user_id = ? AND movie_id = ?", idgen.Arg(link.UserID), idgen.Arg(link.MovieID)).First(&existing).Error
	if err == nil {
		return tx.Model(&existing).Updates(map[string]interface{}{
			"status":     link.Status,
			"rating":     link.Rating,
			"notes":      link.Notes,
			"updated_at": link.UpdatedAt,
		}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return tx.Create(link).Error
}

func upsertUserTVShow(tx *gorm.DB, link *tvshows.UserTVShow) error {
	var existing tvshows.UserTVShow
	err := tx.Where("user_id = ? AND tv_show_id = ?", idgen.Arg(link.UserID), idgen.Arg(link.TVShowID)).First(&existing).Error
	if err == nil {
		return tx.Model(&existing).Updates(map[string]interface{}{
			"current_season":  link.CurrentSeason,
			"current_episode": link.CurrentEpisode,
			"status":          link.Status,
			"rating":          link.Rating,
			"notes":           link.Notes,
			"updated_at":      link.UpdatedAt,
		}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return tx.Create(link).Error
}

func upsertUserBook(tx *gorm.DB, link *books.UserBook) error {
	var existing books.UserBook
	err := tx.Where("user_id = ? AND book_id = ?", idgen.Arg(link.UserID), idgen.Arg(link.BookID)).First(&existing).Error
	if err == nil {
		return tx.Model(&existing).Updates(map[string]interface{}{
			"status":     link.Status,
			"rating":     link.Rating,
			"notes":      link.Notes,
			"updated_at": link.UpdatedAt,
		}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return tx.Create(link).Error
}

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
