-- +goose Up
-- +goose StatementBegin
CREATE TABLE test_table (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(100) NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS test_table;
-- +goose StatementEnd
