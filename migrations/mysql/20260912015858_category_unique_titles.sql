-- +goose Up
-- +goose StatementBegin
CREATE UNIQUE INDEX idx_movies_title ON movies (title);
CREATE UNIQUE INDEX idx_tv_shows_title ON tv_shows (title);
CREATE UNIQUE INDEX idx_books_title ON books (title);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX idx_movies_title ON movies;
DROP INDEX idx_tv_shows_title ON tv_shows;
DROP INDEX idx_books_title ON books;
-- +goose StatementEnd
