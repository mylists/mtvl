-- +goose Up
-- +goose StatementBegin
SET FOREIGN_KEY_CHECKS = 0;

ALTER TABLE user_movies DROP FOREIGN KEY user_movies_ibfk_1;
ALTER TABLE user_tv_shows DROP FOREIGN KEY user_tv_shows_ibfk_1;
ALTER TABLE user_books DROP FOREIGN KEY user_books_ibfk_1;

ALTER TABLE user_movies DROP PRIMARY KEY;
ALTER TABLE user_tv_shows DROP PRIMARY KEY;
ALTER TABLE user_books DROP PRIMARY KEY;

ALTER TABLE users ADD COLUMN new_id CHAR(36) NULL;
UPDATE users SET new_id = UUID() WHERE new_id IS NULL;
ALTER TABLE users MODIFY new_id CHAR(36) NOT NULL;

ALTER TABLE user_movies ADD COLUMN new_user_id CHAR(36) NULL;
UPDATE user_movies um JOIN users u ON um.user_id = u.id SET um.new_user_id = u.new_id;
DELETE FROM user_movies WHERE new_user_id IS NULL;

ALTER TABLE user_tv_shows ADD COLUMN new_user_id CHAR(36) NULL;
UPDATE user_tv_shows ut JOIN users u ON ut.user_id = u.id SET ut.new_user_id = u.new_id;
DELETE FROM user_tv_shows WHERE new_user_id IS NULL;

ALTER TABLE user_books ADD COLUMN new_user_id CHAR(36) NULL;
UPDATE user_books ub JOIN users u ON ub.user_id = u.id SET ub.new_user_id = u.new_id;
DELETE FROM user_books WHERE new_user_id IS NULL;

ALTER TABLE users DROP PRIMARY KEY, DROP COLUMN id, CHANGE new_id id CHAR(36) NOT NULL, ADD PRIMARY KEY (id);

ALTER TABLE user_movies DROP COLUMN user_id, CHANGE new_user_id user_id CHAR(36) NOT NULL, ADD PRIMARY KEY (user_id, movie_id);
ALTER TABLE user_movies ADD CONSTRAINT user_movies_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE user_tv_shows DROP COLUMN user_id, CHANGE new_user_id user_id CHAR(36) NOT NULL, ADD PRIMARY KEY (user_id, tv_show_id);
ALTER TABLE user_tv_shows ADD CONSTRAINT user_tv_shows_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE user_books DROP COLUMN user_id, CHANGE new_user_id user_id CHAR(36) NOT NULL, ADD PRIMARY KEY (user_id, book_id);
ALTER TABLE user_books ADD CONSTRAINT user_books_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

SET FOREIGN_KEY_CHECKS = 1;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET FOREIGN_KEY_CHECKS = 0;

ALTER TABLE user_movies DROP FOREIGN KEY user_movies_user_id_fkey;
ALTER TABLE user_tv_shows DROP FOREIGN KEY user_tv_shows_user_id_fkey;
ALTER TABLE user_books DROP FOREIGN KEY user_books_user_id_fkey;

ALTER TABLE user_movies DROP PRIMARY KEY;
ALTER TABLE user_tv_shows DROP PRIMARY KEY;
ALTER TABLE user_books DROP PRIMARY KEY;

ALTER TABLE users ADD COLUMN new_id INT NULL;
SET @row := 0;
UPDATE users SET new_id = (@row := @row + 1);
ALTER TABLE users MODIFY new_id INT NOT NULL AUTO_INCREMENT;

ALTER TABLE user_movies ADD COLUMN new_user_id INT NULL;
UPDATE user_movies um JOIN users u ON um.user_id = u.id SET um.new_user_id = u.new_id;

ALTER TABLE user_tv_shows ADD COLUMN new_user_id INT NULL;
UPDATE user_tv_shows ut JOIN users u ON ut.user_id = u.id SET ut.new_user_id = u.new_id;

ALTER TABLE user_books ADD COLUMN new_user_id INT NULL;
UPDATE user_books ub JOIN users u ON ub.user_id = u.id SET ub.new_user_id = u.new_id;

ALTER TABLE users DROP PRIMARY KEY, DROP COLUMN id, CHANGE new_id id INT NOT NULL AUTO_INCREMENT, ADD PRIMARY KEY (id);

ALTER TABLE user_movies DROP COLUMN user_id, CHANGE new_user_id user_id INT NOT NULL, ADD PRIMARY KEY (user_id, movie_id);
ALTER TABLE user_movies ADD CONSTRAINT user_movies_ibfk_1 FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE user_tv_shows DROP COLUMN user_id, CHANGE new_user_id user_id INT NOT NULL, ADD PRIMARY KEY (user_id, tv_show_id);
ALTER TABLE user_tv_shows ADD CONSTRAINT user_tv_shows_ibfk_1 FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE user_books DROP COLUMN user_id, CHANGE new_user_id user_id INT NOT NULL, ADD PRIMARY KEY (user_id, book_id);
ALTER TABLE user_books ADD CONSTRAINT user_books_ibfk_1 FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

SET FOREIGN_KEY_CHECKS = 1;
-- +goose StatementEnd
