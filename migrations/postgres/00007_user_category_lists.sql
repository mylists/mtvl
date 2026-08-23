-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS user_movies (
    user_id INTEGER NOT NULL,
    movie_id UUID NOT NULL,
    status VARCHAR(50) DEFAULT 'plan_to_watch',
    rating INTEGER DEFAULT 0,
    notes TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, movie_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (movie_id) REFERENCES movies(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS user_tv_shows (
    user_id INTEGER NOT NULL,
    tv_show_id UUID NOT NULL,
    current_season INTEGER DEFAULT 1,
    current_episode INTEGER DEFAULT 0,
    status VARCHAR(50) DEFAULT 'plan_to_watch',
    rating INTEGER DEFAULT 0,
    notes TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, tv_show_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (tv_show_id) REFERENCES tv_shows(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS user_books (
    user_id INTEGER NOT NULL,
    book_id UUID NOT NULL,
    status VARCHAR(50) DEFAULT 'plan_to_read',
    rating INTEGER DEFAULT 0,
    notes TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, book_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
);

INSERT INTO user_movies (user_id, movie_id, status, rating, notes, created_at, updated_at)
SELECT m.user_id, m.id, m.status, m.rating, m.notes, m.created_at, m.updated_at
FROM movies m
INNER JOIN users u ON u.id = m.user_id;

INSERT INTO user_tv_shows (user_id, tv_show_id, current_season, current_episode, status, rating, notes, created_at, updated_at)
SELECT t.user_id, t.id, t.current_season, t.current_episode, t.status, t.rating, t.notes, t.created_at, t.updated_at
FROM tv_shows t
INNER JOIN users u ON u.id = t.user_id;

INSERT INTO user_books (user_id, book_id, status, rating, notes, created_at, updated_at)
SELECT b.user_id, b.id, b.status, b.rating, b.notes, b.created_at, b.updated_at
FROM books b
INNER JOIN users u ON u.id = b.user_id;

ALTER TABLE movies
    DROP COLUMN user_id,
    DROP COLUMN status,
    DROP COLUMN rating,
    DROP COLUMN notes;

ALTER TABLE tv_shows
    DROP COLUMN user_id,
    DROP COLUMN current_season,
    DROP COLUMN current_episode,
    DROP COLUMN status,
    DROP COLUMN rating,
    DROP COLUMN notes;

ALTER TABLE books
    DROP COLUMN user_id,
    DROP COLUMN status,
    DROP COLUMN rating,
    DROP COLUMN notes;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE movies
    ADD COLUMN user_id INTEGER,
    ADD COLUMN status VARCHAR(50) DEFAULT 'plan_to_watch',
    ADD COLUMN rating INTEGER DEFAULT 0,
    ADD COLUMN notes TEXT DEFAULT '';

ALTER TABLE tv_shows
    ADD COLUMN user_id INTEGER,
    ADD COLUMN current_season INTEGER DEFAULT 1,
    ADD COLUMN current_episode INTEGER DEFAULT 0,
    ADD COLUMN status VARCHAR(50) DEFAULT 'plan_to_watch',
    ADD COLUMN rating INTEGER DEFAULT 0,
    ADD COLUMN notes TEXT DEFAULT '';

ALTER TABLE books
    ADD COLUMN user_id INTEGER,
    ADD COLUMN status VARCHAR(50) DEFAULT 'plan_to_read',
    ADD COLUMN rating INTEGER DEFAULT 0,
    ADD COLUMN notes TEXT DEFAULT '';

UPDATE movies m
SET user_id = u.user_id,
    status = u.status,
    rating = u.rating,
    notes = u.notes
FROM user_movies u
WHERE u.movie_id = m.id
  AND u.user_id = (SELECT MIN(user_id) FROM user_movies WHERE movie_id = m.id);

UPDATE tv_shows t
SET user_id = u.user_id,
    current_season = u.current_season,
    current_episode = u.current_episode,
    status = u.status,
    rating = u.rating,
    notes = u.notes
FROM user_tv_shows u
WHERE u.tv_show_id = t.id
  AND u.user_id = (SELECT MIN(user_id) FROM user_tv_shows WHERE tv_show_id = t.id);

UPDATE books b
SET user_id = u.user_id,
    status = u.status,
    rating = u.rating,
    notes = u.notes
FROM user_books u
WHERE u.book_id = b.id
  AND u.user_id = (SELECT MIN(user_id) FROM user_books WHERE book_id = b.id);

DROP TABLE IF EXISTS user_movies;
DROP TABLE IF EXISTS user_tv_shows;
DROP TABLE IF EXISTS user_books;
-- +goose StatementEnd
