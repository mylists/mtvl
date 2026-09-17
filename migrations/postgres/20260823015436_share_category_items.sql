-- +goose Up
-- +goose StatementBegin
ALTER TABLE movies DROP CONSTRAINT IF EXISTS movies_user_id_fkey;
ALTER TABLE tv_shows DROP CONSTRAINT IF EXISTS tv_shows_user_id_fkey;
ALTER TABLE books DROP CONSTRAINT IF EXISTS books_user_id_fkey;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE movies ADD CONSTRAINT movies_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE tv_shows ADD CONSTRAINT tv_shows_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE books ADD CONSTRAINT books_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
-- +goose StatementEnd
