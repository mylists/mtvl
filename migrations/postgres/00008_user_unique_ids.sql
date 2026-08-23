-- +goose Up
-- +goose StatementBegin
ALTER TABLE user_movies DROP CONSTRAINT user_movies_user_id_fkey;
ALTER TABLE user_tv_shows DROP CONSTRAINT user_tv_shows_user_id_fkey;
ALTER TABLE user_books DROP CONSTRAINT user_books_user_id_fkey;

ALTER TABLE user_movies DROP CONSTRAINT user_movies_pkey;
ALTER TABLE user_tv_shows DROP CONSTRAINT user_tv_shows_pkey;
ALTER TABLE user_books DROP CONSTRAINT user_books_pkey;

ALTER TABLE users ADD COLUMN new_id UUID;
UPDATE users SET new_id = gen_random_uuid() WHERE new_id IS NULL;
ALTER TABLE users ALTER COLUMN new_id SET NOT NULL;

ALTER TABLE user_movies ADD COLUMN new_user_id UUID;
UPDATE user_movies um SET new_user_id = u.new_id FROM users u WHERE um.user_id = u.id;
DELETE FROM user_movies WHERE new_user_id IS NULL;

ALTER TABLE user_tv_shows ADD COLUMN new_user_id UUID;
UPDATE user_tv_shows ut SET new_user_id = u.new_id FROM users u WHERE ut.user_id = u.id;
DELETE FROM user_tv_shows WHERE new_user_id IS NULL;

ALTER TABLE user_books ADD COLUMN new_user_id UUID;
UPDATE user_books ub SET new_user_id = u.new_id FROM users u WHERE ub.user_id = u.id;
DELETE FROM user_books WHERE new_user_id IS NULL;

ALTER TABLE users DROP CONSTRAINT users_pkey;
ALTER TABLE users DROP COLUMN id;
ALTER TABLE users RENAME COLUMN new_id TO id;
ALTER TABLE users ADD PRIMARY KEY (id);

ALTER TABLE user_movies DROP COLUMN user_id;
ALTER TABLE user_movies RENAME COLUMN new_user_id TO user_id;
ALTER TABLE user_movies ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE user_movies ADD PRIMARY KEY (user_id, movie_id);
ALTER TABLE user_movies ADD CONSTRAINT user_movies_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE user_tv_shows DROP COLUMN user_id;
ALTER TABLE user_tv_shows RENAME COLUMN new_user_id TO user_id;
ALTER TABLE user_tv_shows ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE user_tv_shows ADD PRIMARY KEY (user_id, tv_show_id);
ALTER TABLE user_tv_shows ADD CONSTRAINT user_tv_shows_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE user_books DROP COLUMN user_id;
ALTER TABLE user_books RENAME COLUMN new_user_id TO user_id;
ALTER TABLE user_books ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE user_books ADD PRIMARY KEY (user_id, book_id);
ALTER TABLE user_books ADD CONSTRAINT user_books_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE user_movies DROP CONSTRAINT user_movies_user_id_fkey;
ALTER TABLE user_tv_shows DROP CONSTRAINT user_tv_shows_user_id_fkey;
ALTER TABLE user_books DROP CONSTRAINT user_books_user_id_fkey;

ALTER TABLE user_movies DROP CONSTRAINT user_movies_pkey;
ALTER TABLE user_tv_shows DROP CONSTRAINT user_tv_shows_pkey;
ALTER TABLE user_books DROP CONSTRAINT user_books_pkey;

ALTER TABLE users ADD COLUMN new_id SERIAL;
ALTER TABLE user_movies ADD COLUMN new_user_id INTEGER;
UPDATE user_movies um SET new_user_id = u.new_id FROM users u WHERE um.user_id = u.id;

ALTER TABLE user_tv_shows ADD COLUMN new_user_id INTEGER;
UPDATE user_tv_shows ut SET new_user_id = u.new_id FROM users u WHERE ut.user_id = u.id;

ALTER TABLE user_books ADD COLUMN new_user_id INTEGER;
UPDATE user_books ub SET new_user_id = u.new_id FROM users u WHERE ub.user_id = u.id;

ALTER TABLE users DROP CONSTRAINT users_pkey;
ALTER TABLE users DROP COLUMN id;
ALTER TABLE users RENAME COLUMN new_id TO id;
ALTER TABLE users ADD PRIMARY KEY (id);

ALTER TABLE user_movies DROP COLUMN user_id;
ALTER TABLE user_movies RENAME COLUMN new_user_id TO user_id;
ALTER TABLE user_movies ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE user_movies ADD PRIMARY KEY (user_id, movie_id);
ALTER TABLE user_movies ADD CONSTRAINT user_movies_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE user_tv_shows DROP COLUMN user_id;
ALTER TABLE user_tv_shows RENAME COLUMN new_user_id TO user_id;
ALTER TABLE user_tv_shows ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE user_tv_shows ADD PRIMARY KEY (user_id, tv_show_id);
ALTER TABLE user_tv_shows ADD CONSTRAINT user_tv_shows_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE user_books DROP COLUMN user_id;
ALTER TABLE user_books RENAME COLUMN new_user_id TO user_id;
ALTER TABLE user_books ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE user_books ADD PRIMARY KEY (user_id, book_id);
ALTER TABLE user_books ADD CONSTRAINT user_books_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
-- +goose StatementEnd
