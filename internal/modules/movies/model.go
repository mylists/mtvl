package movies

import "time"

// Movie represents a movie tracking entry.
type Movie struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	Title       string    `json:"title"`
	ReleaseYear int       `json:"release_year"`
	Director    string    `json:"director"`
	Status      string    `json:"status"` // plan_to_watch, watching, completed, dropped
	Rating      int       `json:"rating"` // 1 - 10
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
