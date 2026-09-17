-- +goose Up
-- +goose StatementBegin
ALTER TABLE movies ADD COLUMN new_id UUID;
UPDATE movies SET new_id = gen_random_uuid() WHERE new_id IS NULL;
ALTER TABLE movies ALTER COLUMN new_id SET NOT NULL;
ALTER TABLE movies DROP CONSTRAINT movies_pkey;
ALTER TABLE movies DROP COLUMN id;
ALTER TABLE movies RENAME COLUMN new_id TO id;
ALTER TABLE movies ADD PRIMARY KEY (id);

ALTER TABLE tv_shows ADD COLUMN new_id UUID;
UPDATE tv_shows SET new_id = gen_random_uuid() WHERE new_id IS NULL;
ALTER TABLE tv_shows ALTER COLUMN new_id SET NOT NULL;
ALTER TABLE tv_shows DROP CONSTRAINT tv_shows_pkey;
ALTER TABLE tv_shows DROP COLUMN id;
ALTER TABLE tv_shows RENAME COLUMN new_id TO id;
ALTER TABLE tv_shows ADD PRIMARY KEY (id);

ALTER TABLE books ADD COLUMN new_id UUID;
UPDATE books SET new_id = gen_random_uuid() WHERE new_id IS NULL;
ALTER TABLE books ALTER COLUMN new_id SET NOT NULL;
ALTER TABLE books DROP CONSTRAINT books_pkey;
ALTER TABLE books DROP COLUMN id;
ALTER TABLE books RENAME COLUMN new_id TO id;
ALTER TABLE books ADD PRIMARY KEY (id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE movies ADD COLUMN new_id SERIAL;
ALTER TABLE movies DROP CONSTRAINT movies_pkey;
ALTER TABLE movies DROP COLUMN id;
ALTER TABLE movies RENAME COLUMN new_id TO id;
ALTER TABLE movies ADD PRIMARY KEY (id);

ALTER TABLE tv_shows ADD COLUMN new_id SERIAL;
ALTER TABLE tv_shows DROP CONSTRAINT tv_shows_pkey;
ALTER TABLE tv_shows DROP COLUMN id;
ALTER TABLE tv_shows RENAME COLUMN new_id TO id;
ALTER TABLE tv_shows ADD PRIMARY KEY (id);

ALTER TABLE books ADD COLUMN new_id SERIAL;
ALTER TABLE books DROP CONSTRAINT books_pkey;
ALTER TABLE books DROP COLUMN id;
ALTER TABLE books RENAME COLUMN new_id TO id;
ALTER TABLE books ADD PRIMARY KEY (id);
-- +goose StatementEnd
