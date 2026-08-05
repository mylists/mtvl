package tvshows

import "time"

// TVShow represents a TV show tracking entry.
type TVShow struct {
	ID             int64     `json:"id" gorm:"primaryKey;autoIncrement;column:id"`
	UserID         int64     `json:"user_id" gorm:"index;column:user_id;not null"`
	Title          string    `json:"title" gorm:"column:title;not null"`
	CurrentSeason  int       `json:"current_season" gorm:"column:current_season"`
	CurrentEpisode int       `json:"current_episode" gorm:"column:current_episode"`
	TotalEpisodes  int       `json:"total_episodes" gorm:"column:total_episodes"`
	Status         string    `json:"status" gorm:"column:status;not null;default:'plan_to_watch'"`
	Rating         int       `json:"rating" gorm:"column:rating;default:0"`
	Notes          string    `json:"notes" gorm:"column:notes"`
	CreatedAt      time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (TVShow) TableName() string {
	return "tv_shows"
}

