-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS tv_shows (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT NOT NULL,
    title VARCHAR(255) NOT NULL,
    current_season INT DEFAULT 1,
    current_episode INT DEFAULT 0,
    total_episodes INT DEFAULT 0,
    status VARCHAR(50) DEFAULT 'plan_to_watch',
    rating INT DEFAULT 0,
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tv_shows;
-- +goose StatementEnd
