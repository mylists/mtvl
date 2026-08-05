package movies

import "time"

// Movie represents a movie tracking entry.
type Movie struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement;column:id"`
	UserID      int64     `json:"user_id" gorm:"index;column:user_id;not null"`
	Title       string    `json:"title" gorm:"column:title;not null"`
	ReleaseYear int       `json:"release_year" gorm:"column:release_year"`
	Director    string    `json:"director" gorm:"column:director"`
	Status      string    `json:"status" gorm:"column:status;not null;default:'plan_to_watch'"`
	Rating      int       `json:"rating" gorm:"column:rating;default:0"`
	Notes       string    `json:"notes" gorm:"column:notes"`
	CreatedAt   time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (Movie) TableName() string {
	return "movies"
}

