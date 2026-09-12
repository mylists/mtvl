-- +goose Up
-- +goose StatementBegin
CREATE UNIQUE INDEX IF NOT EXISTS idx_movies_title ON movies (title);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tv_shows_title ON tv_shows (title);
CREATE UNIQUE INDEX IF NOT EXISTS idx_books_title ON books (title);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_movies_title;
DROP INDEX IF EXISTS idx_tv_shows_title;
DROP INDEX IF EXISTS idx_books_title;
-- +goose StatementEnd
