package tvshows

import (
	"time"

	"gorm.io/gorm"
	"mtvl/internal/idgen"
)

// TVShow is a shared catalog item.
type TVShow struct {
	ID            string    `json:"id" gorm:"primaryKey;size:36;column:id"`
	Title         string    `json:"title" gorm:"column:title;not null"`
	TotalEpisodes int       `json:"total_episodes" gorm:"column:total_episodes"`
	CreatedAt     time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (TVShow) TableName() string {
	return "tv_shows"
}

func (m *TVShow) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = idgen.New()
	}
	return nil
}

// UserTVShow links a user to a catalog TV show on their list.
type UserTVShow struct {
	UserID         int64     `json:"user_id" gorm:"primaryKey;column:user_id"`
	TVShowID       string    `json:"tv_show_id" gorm:"primaryKey;size:36;column:tv_show_id"`
	CurrentSeason  int       `json:"current_season" gorm:"column:current_season"`
	CurrentEpisode int       `json:"current_episode" gorm:"column:current_episode"`
	Status         string    `json:"status" gorm:"column:status;not null;default:'plan_to_watch'"`
	Rating         int       `json:"rating" gorm:"column:rating;default:0"`
	Notes          string    `json:"notes" gorm:"column:notes"`
	CreatedAt      time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (UserTVShow) TableName() string {
	return "user_tv_shows"
}

// TVShowListItem is a catalog show joined with the current user's list fields.
type TVShowListItem struct {
	ID             string    `json:"id" gorm:"column:id"`
	Title          string    `json:"title" gorm:"column:title"`
	TotalEpisodes  int       `json:"total_episodes" gorm:"column:total_episodes"`
	CurrentSeason  int       `json:"current_season" gorm:"column:current_season"`
	CurrentEpisode int       `json:"current_episode" gorm:"column:current_episode"`
	Status         string    `json:"status" gorm:"column:status"`
	Rating         int       `json:"rating" gorm:"column:rating"`
	Notes          string    `json:"notes" gorm:"column:notes"`
	CreatedAt      time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"column:updated_at"`
}
