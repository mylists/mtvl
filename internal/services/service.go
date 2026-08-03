package services

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"mtvl/internal/auth"
	"mtvl/internal/modules/books"
	"mtvl/internal/modules/movies"
	"mtvl/internal/modules/tvshows"
)

type ServiceHandler struct {
	db *sql.DB
}

func NewServiceHandler(db *sql.DB) *ServiceHandler {
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

// GetStats returns aggregated dashboard statistics across all tracking categories.
func (s *ServiceHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	ctx := r.Context()

	// Movies stats
	var movieTotal int
	var movieAvgRating float64
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(AVG(rating), 0) FROM movies WHERE user_id = ?`, user.ID).Scan(&movieTotal, &movieAvgRating)

	movieStatusBreakdown := make(map[string]int)
	mRows, err := s.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM movies WHERE user_id = ? GROUP BY status`, user.ID)
	if err == nil {
		for mRows.Next() {
			var st string
			var cnt int
			if err := mRows.Scan(&st, &cnt); err == nil {
				movieStatusBreakdown[st] = cnt
			}
		}
		mRows.Close()
	}

	// TV Shows stats
	var tvTotal int
	var tvAvgRating float64
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(AVG(rating), 0) FROM tv_shows WHERE user_id = ?`, user.ID).Scan(&tvTotal, &tvAvgRating)

	tvStatusBreakdown := make(map[string]int)
	tRows, err := s.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM tv_shows WHERE user_id = ? GROUP BY status`, user.ID)
	if err == nil {
		for tRows.Next() {
			var st string
			var cnt int
			if err := tRows.Scan(&st, &cnt); err == nil {
				tvStatusBreakdown[st] = cnt
			}
		}
		tRows.Close()
	}

	// Books stats
	var bookTotal int
	var bookAvgRating float64
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(AVG(rating), 0) FROM books WHERE user_id = ?`, user.ID).Scan(&bookTotal, &bookAvgRating)

	bookStatusBreakdown := make(map[string]int)
	bRows, err := s.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM books WHERE user_id = ? GROUP BY status`, user.ID)
	if err == nil {
		for bRows.Next() {
			var st string
			var cnt int
			if err := bRows.Scan(&st, &cnt); err == nil {
				bookStatusBreakdown[st] = cnt
			}
		}
		bRows.Close()
	}

	totalItems := movieTotal + tvTotal + bookTotal

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total_items": totalItems,
		"categories": map[string]interface{}{
			"movies": map[string]interface{}{
				"total":            movieTotal,
				"average_rating":   movieAvgRating,
				"status_breakdown": movieStatusBreakdown,
			},
			"tv_shows": map[string]interface{}{
				"total":            tvTotal,
				"average_rating":   tvAvgRating,
				"status_breakdown": tvStatusBreakdown,
			},
			"books": map[string]interface{}{
				"total":            bookTotal,
				"average_rating":   bookAvgRating,
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
	mRows, err := s.db.QueryContext(ctx, `SELECT id, user_id, title, release_year, director, status, rating, notes, created_at, updated_at FROM movies WHERE user_id = ? AND (LOWER(title) LIKE ? OR LOWER(director) LIKE ? OR LOWER(notes) LIKE ?)`, user.ID, pattern, pattern, pattern)
	if err == nil {
		for mRows.Next() {
			var mov movies.Movie
			if err := mRows.Scan(&mov.ID, &mov.UserID, &mov.Title, &mov.ReleaseYear, &mov.Director, &mov.Status, &mov.Rating, &mov.Notes, &mov.CreatedAt, &mov.UpdatedAt); err == nil {
				movieResults = append(movieResults, mov)
			}
		}
		mRows.Close()
	}

	// Search tv shows
	tvResults := make([]tvshows.TVShow, 0)
	tRows, err := s.db.QueryContext(ctx, `SELECT id, user_id, title, current_season, current_episode, total_episodes, status, rating, notes, created_at, updated_at FROM tv_shows WHERE user_id = ? AND (LOWER(title) LIKE ? OR LOWER(notes) LIKE ?)`, user.ID, pattern, pattern)
	if err == nil {
		for tRows.Next() {
			var show tvshows.TVShow
			if err := tRows.Scan(&show.ID, &show.UserID, &show.Title, &show.CurrentSeason, &show.CurrentEpisode, &show.TotalEpisodes, &show.Status, &show.Rating, &show.Notes, &show.CreatedAt, &show.UpdatedAt); err == nil {
				tvResults = append(tvResults, show)
			}
		}
		tRows.Close()
	}

	// Search books
	bookResults := make([]books.Book, 0)
	bRows, err := s.db.QueryContext(ctx, `SELECT id, user_id, title, status, rating, notes, created_at, updated_at FROM books WHERE user_id = ? AND (LOWER(title) LIKE ? OR LOWER(notes) LIKE ?)`, user.ID, pattern, pattern)
	if err == nil {
		for bRows.Next() {
			var book books.Book
			if err := bRows.Scan(&book.ID, &book.UserID, &book.Title, &book.Status, &book.Rating, &book.Notes, &book.CreatedAt, &book.UpdatedAt); err == nil {
				bookResults = append(bookResults, book)
			}
		}
		bRows.Close()
	}

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
	mRows, err := s.db.QueryContext(ctx, `SELECT id, user_id, title, release_year, director, status, rating, notes, created_at, updated_at FROM movies WHERE user_id = ? ORDER BY id ASC`, user.ID)
	if err == nil {
		for mRows.Next() {
			var mov movies.Movie
			if err := mRows.Scan(&mov.ID, &mov.UserID, &mov.Title, &mov.ReleaseYear, &mov.Director, &mov.Status, &mov.Rating, &mov.Notes, &mov.CreatedAt, &mov.UpdatedAt); err == nil {
				movieList = append(movieList, mov)
			}
		}
		mRows.Close()
	}

	// TV Shows
	tvList := make([]tvshows.TVShow, 0)
	tRows, err := s.db.QueryContext(ctx, `SELECT id, user_id, title, current_season, current_episode, total_episodes, status, rating, notes, created_at, updated_at FROM tv_shows WHERE user_id = ? ORDER BY id ASC`, user.ID)
	if err == nil {
		for tRows.Next() {
			var show tvshows.TVShow
			if err := tRows.Scan(&show.ID, &show.UserID, &show.Title, &show.CurrentSeason, &show.CurrentEpisode, &show.TotalEpisodes, &show.Status, &show.Rating, &show.Notes, &show.CreatedAt, &show.UpdatedAt); err == nil {
				tvList = append(tvList, show)
			}
		}
		tRows.Close()
	}

	// Books
	bookList := make([]books.Book, 0)
	bRows, err := s.db.QueryContext(ctx, `SELECT id, user_id, title, status, rating, notes, created_at, updated_at FROM books WHERE user_id = ? ORDER BY id ASC`, user.ID)
	if err == nil {
		for bRows.Next() {
			var book books.Book
			if err := bRows.Scan(&book.ID, &book.UserID, &book.Title, &book.Status, &book.Rating, &book.Notes, &book.CreatedAt, &book.UpdatedAt); err == nil {
				bookList = append(bookList, book)
			}
		}
		bRows.Close()
	}

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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to begin transaction: "+err.Error())
		return
	}
	defer tx.Rollback()

	if req.Overwrite {
		_, _ = tx.ExecContext(ctx, `DELETE FROM movies WHERE user_id = ?`, user.ID)
		_, _ = tx.ExecContext(ctx, `DELETE FROM tv_shows WHERE user_id = ?`, user.ID)
		_, _ = tx.ExecContext(ctx, `DELETE FROM books WHERE user_id = ?`, user.ID)
	}

	now := time.Now()
	var importedMovies, importedTVShows, importedBooks int

	for _, mov := range req.Data.Movies {
		if strings.TrimSpace(mov.Title) == "" {
			continue
		}
		status := mov.Status
		if status == "" {
			status = "plan_to_watch"
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO movies (user_id, title, release_year, director, status, rating, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, user.ID, mov.Title, mov.ReleaseYear, mov.Director, status, mov.Rating, mov.Notes, now, now)
		if err == nil {
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
		_, err := tx.ExecContext(ctx, `INSERT INTO tv_shows (user_id, title, current_season, current_episode, total_episodes, status, rating, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, user.ID, show.Title, season, show.CurrentEpisode, show.TotalEpisodes, status, show.Rating, show.Notes, now, now)
		if err == nil {
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
		_, err := tx.ExecContext(ctx, `INSERT INTO books (user_id, title, status, rating, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, user.ID, book.Title, status, book.Rating, book.Notes, now, now)
		if err == nil {
			importedBooks++
		}
	}

	if err := tx.Commit(); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to commit import transaction: "+err.Error())
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
