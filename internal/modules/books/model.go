package books

import "time"

// Book represents a tracking entry for books.
type Book struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	Rating    int       `json:"rating"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
