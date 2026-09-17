-- +goose Up
-- +goose StatementBegin
ALTER TABLE movies DROP FOREIGN KEY movies_ibfk_1;
ALTER TABLE tv_shows DROP FOREIGN KEY tv_shows_ibfk_1;
ALTER TABLE books DROP FOREIGN KEY books_ibfk_1;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE movies ADD CONSTRAINT movies_ibfk_1 FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE tv_shows ADD CONSTRAINT tv_shows_ibfk_1 FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE books ADD CONSTRAINT books_ibfk_1 FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
-- +goose StatementEnd
