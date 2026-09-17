-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS user_movies (
    user_id INT NOT NULL,
    movie_id CHAR(36) NOT NULL,
    status VARCHAR(50) DEFAULT 'plan_to_watch',
    rating INT DEFAULT 0,
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, movie_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (movie_id) REFERENCES movies(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS user_tv_shows (
    user_id INT NOT NULL,
    tv_show_id CHAR(36) NOT NULL,
    current_season INT DEFAULT 1,
    current_episode INT DEFAULT 0,
    status VARCHAR(50) DEFAULT 'plan_to_watch',
    rating INT DEFAULT 0,
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, tv_show_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (tv_show_id) REFERENCES tv_shows(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS user_books (
    user_id INT NOT NULL,
    book_id CHAR(36) NOT NULL,
    status VARCHAR(50) DEFAULT 'plan_to_read',
    rating INT DEFAULT 0,
    notes TEXT,
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
    ADD COLUMN user_id INT,
    ADD COLUMN status VARCHAR(50) DEFAULT 'plan_to_watch',
    ADD COLUMN rating INT DEFAULT 0,
    ADD COLUMN notes TEXT;

ALTER TABLE tv_shows
    ADD COLUMN user_id INT,
    ADD COLUMN current_season INT DEFAULT 1,
    ADD COLUMN current_episode INT DEFAULT 0,
    ADD COLUMN status VARCHAR(50) DEFAULT 'plan_to_watch',
    ADD COLUMN rating INT DEFAULT 0,
    ADD COLUMN notes TEXT;

ALTER TABLE books
    ADD COLUMN user_id INT,
    ADD COLUMN status VARCHAR(50) DEFAULT 'plan_to_read',
    ADD COLUMN rating INT DEFAULT 0,
    ADD COLUMN notes TEXT;

UPDATE movies m
JOIN (
    SELECT um.*
    FROM user_movies um
    INNER JOIN (
        SELECT movie_id, MIN(user_id) AS user_id
        FROM user_movies
        GROUP BY movie_id
    ) first_link ON first_link.movie_id = um.movie_id AND first_link.user_id = um.user_id
) u ON u.movie_id = m.id
SET m.user_id = u.user_id,
    m.status = u.status,
    m.rating = u.rating,
    m.notes = u.notes;

UPDATE tv_shows t
JOIN (
    SELECT ut.*
    FROM user_tv_shows ut
    INNER JOIN (
        SELECT tv_show_id, MIN(user_id) AS user_id
        FROM user_tv_shows
        GROUP BY tv_show_id
    ) first_link ON first_link.tv_show_id = ut.tv_show_id AND first_link.user_id = ut.user_id
) u ON u.tv_show_id = t.id
SET t.user_id = u.user_id,
    t.current_season = u.current_season,
    t.current_episode = u.current_episode,
    t.status = u.status,
    t.rating = u.rating,
    t.notes = u.notes;

UPDATE books b
JOIN (
    SELECT ub.*
    FROM user_books ub
    INNER JOIN (
        SELECT book_id, MIN(user_id) AS user_id
        FROM user_books
        GROUP BY book_id
    ) first_link ON first_link.book_id = ub.book_id AND first_link.user_id = ub.user_id
) u ON u.book_id = b.id
SET b.user_id = u.user_id,
    b.status = u.status,
    b.rating = u.rating,
    b.notes = u.notes;

DROP TABLE IF EXISTS user_movies;
DROP TABLE IF EXISTS user_tv_shows;
DROP TABLE IF EXISTS user_books;
-- +goose StatementEnd
