package movies

import (
	"time"

	"gorm.io/gorm"
	"mtvl/internal/idgen"
)

// Movie is a shared catalog item.
type Movie struct {
	ID          string    `json:"id" gorm:"primaryKey;type:uuid;size:36;column:id"`
	Title       string    `json:"title" gorm:"column:title;not null;uniqueIndex"`
	ReleaseYear int       `json:"release_year" gorm:"column:release_year"`
	Director    string    `json:"director" gorm:"column:director"`
	CreatedAt   time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (Movie) TableName() string {
	return "movies"
}

func (m *Movie) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = idgen.New()
	}
	return nil
}

// UserMovie links a user to a catalog movie on their list.
type UserMovie struct {
	UserID    string    `json:"user_id" gorm:"primaryKey;type:uuid;size:36;column:user_id"`
	MovieID   string    `json:"movie_id" gorm:"primaryKey;type:uuid;size:36;column:movie_id"`
	Status    string    `json:"status" gorm:"column:status;not null;default:'plan_to_watch'"`
	Rating    int       `json:"rating" gorm:"column:rating;default:0"`
	Notes     string    `json:"notes" gorm:"column:notes"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (UserMovie) TableName() string {
	return "user_movies"
}

// MovieListItem is a catalog movie joined with the current user's list fields.
type MovieListItem struct {
	ID          string    `json:"id" gorm:"column:id"`
	Title       string    `json:"title" gorm:"column:title"`
	ReleaseYear int       `json:"release_year" gorm:"column:release_year"`
	Director    string    `json:"director" gorm:"column:director"`
	Status      string    `json:"status" gorm:"column:status"`
	Rating      int       `json:"rating" gorm:"column:rating"`
	Notes       string    `json:"notes" gorm:"column:notes"`
	CreatedAt   time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"column:updated_at"`
}
