package books

import (
	"time"

	"gorm.io/gorm"
	"mtvl/internal/idgen"
)

// Book is a shared catalog item.
type Book struct {
	ID        string    `json:"id" gorm:"primaryKey;type:uuid;size:36;column:id"`
	Title     string    `json:"title" gorm:"column:title;not null;uniqueIndex"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (Book) TableName() string {
	return "books"
}

func (m *Book) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = idgen.New()
	}
	return nil
}

// UserBook links a user to a catalog book on their list.
type UserBook struct {
	UserID    string    `json:"user_id" gorm:"primaryKey;type:uuid;size:36;column:user_id"`
	BookID    string    `json:"book_id" gorm:"primaryKey;type:uuid;size:36;column:book_id"`
	Status    string    `json:"status" gorm:"column:status;not null;default:'plan_to_read'"`
	Rating    int       `json:"rating" gorm:"column:rating;default:0"`
	Notes     string    `json:"notes" gorm:"column:notes"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (UserBook) TableName() string {
	return "user_books"
}

// BookListItem is a catalog book joined with the current user's list fields.
type BookListItem struct {
	ID        string    `json:"id" gorm:"column:id"`
	Title     string    `json:"title" gorm:"column:title"`
	Status    string    `json:"status" gorm:"column:status"`
	Rating    int       `json:"rating" gorm:"column:rating"`
	Notes     string    `json:"notes" gorm:"column:notes"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"`
}
