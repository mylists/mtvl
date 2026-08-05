package books

import "time"

// Book represents a tracking entry for books.
type Book struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement;column:id"`
	UserID    int64     `json:"user_id" gorm:"index;column:user_id;not null"`
	Title     string    `json:"title" gorm:"column:title;not null"`
	Status    string    `json:"status" gorm:"column:status;not null;default:'plan_to_read'"`
	Rating    int       `json:"rating" gorm:"column:rating;default:0"`
	Notes     string    `json:"notes" gorm:"column:notes"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (Book) TableName() string {
	return "books"
}

