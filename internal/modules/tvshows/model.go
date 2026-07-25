package tvshows

import "time"

// TVShow represents a TV show tracking entry.
type TVShow struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	Title          string    `json:"title"`
	CurrentSeason  int       `json:"current_season"`
	CurrentEpisode int       `json:"current_episode"`
	TotalEpisodes  int       `json:"total_episodes"`
	Status         string    `json:"status"` // plan_to_watch, watching, completed, dropped
	Rating         int       `json:"rating"` // 1 - 10
	Notes          string    `json:"notes"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
