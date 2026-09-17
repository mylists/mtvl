package db

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestOpenDBUnsupportedDriver(t *testing.T) {
	_, err := OpenDB("sqlite3", ":memory:")
	if err == nil {
		t.Errorf("expected error for unsupported sqlite3 driver, got nil")
	}
}

func TestRebind(t *testing.T) {
	query := "SELECT * FROM users WHERE username = ? AND email = ?"

	pgQuery := Rebind("postgres", query)
	expectedPg := "SELECT * FROM users WHERE username = $1 AND email = $2"
	if pgQuery != expectedPg {
		t.Errorf("expected %q, got %q", expectedPg, pgQuery)
	}

	mysqlQuery := Rebind("mysql", query)
	if mysqlQuery != query {
		t.Errorf("expected %q, got %q", query, mysqlQuery)
	}
}

func TestDialectMigrationSQLSyntax(t *testing.T) {
	pgUsers, err := os.ReadFile(filepath.Join("..", "..", "migrations", "postgres", "20260823023445_user_unique_ids.sql"))
	if err != nil {
		t.Fatalf("failed to read postgres user unique id migration: %v", err)
	}
	if !strings.Contains(string(pgUsers), "new_id UUID") {
		t.Errorf("expected postgres user ids to convert to UUID")
	}
	if strings.Contains(upSection(string(pgUsers)), "SERIAL") {
		t.Errorf("postgres user unique id up migration should not keep serial ids")
	}

	mysqlUsers, err := os.ReadFile(filepath.Join("..", "..", "migrations", "mysql", "20260823023445_user_unique_ids.sql"))
	if err != nil {
		t.Fatalf("failed to read mysql user unique id migration: %v", err)
	}
	if !strings.Contains(string(mysqlUsers), "CHAR(36)") {
		t.Errorf("expected mysql user ids to convert to CHAR(36)")
	}
	if strings.Contains(upSection(string(mysqlUsers)), "AUTO_INCREMENT") {
		t.Errorf("mysql user unique id up migration should not keep auto increment ids")
	}

	pgCategoryTitles, err := os.ReadFile(filepath.Join("..", "..", "migrations", "postgres", "20260912015858_category_unique_titles.sql"))
	if err != nil {
		t.Fatalf("failed to read postgres category unique title migration: %v", err)
	}
	for _, table := range []string{"movies", "tv_shows", "books"} {
		if !strings.Contains(upSection(string(pgCategoryTitles)), "CREATE UNIQUE INDEX") || !strings.Contains(upSection(string(pgCategoryTitles)), table+" (title)") {
			t.Errorf("expected postgres unique index on %s (title)", table)
		}
	}

	mysqlCategoryTitles, err := os.ReadFile(filepath.Join("..", "..", "migrations", "mysql", "20260912015858_category_unique_titles.sql"))
	if err != nil {
		t.Fatalf("failed to read mysql category unique title migration: %v", err)
	}
	for _, table := range []string{"movies", "tv_shows", "books"} {
		if !strings.Contains(upSection(string(mysqlCategoryTitles)), "CREATE UNIQUE INDEX") || !strings.Contains(upSection(string(mysqlCategoryTitles)), table+" (title)") {
			t.Errorf("expected mysql unique index on %s (title)", table)
		}
	}

	pgTokens, err := os.ReadFile(filepath.Join("..", "..", "migrations", "postgres", "20260912020906_api_tokens.sql"))
	if err != nil {
		t.Fatalf("failed to read postgres api tokens migration: %v", err)
	}
	if !strings.Contains(upSection(string(pgTokens)), "VARCHAR(128)") || !strings.Contains(upSection(string(pgTokens)), "api_tokens") {
		t.Errorf("expected postgres api tokens table")
	}
	if !strings.Contains(upSection(string(pgTokens)), "idx_api_tokens_token") {
		t.Errorf("expected postgres api tokens unique index")
	}

	mysqlTokens, err := os.ReadFile(filepath.Join("..", "..", "migrations", "mysql", "20260912020906_api_tokens.sql"))
	if err != nil {
		t.Fatalf("failed to read mysql api tokens migration: %v", err)
	}
	if !strings.Contains(upSection(string(mysqlTokens)), "VARCHAR(128)") || !strings.Contains(upSection(string(mysqlTokens)), "api_tokens") {
		t.Errorf("expected mysql api tokens table")
	}
	if !strings.Contains(upSection(string(mysqlTokens)), "idx_api_tokens_token") {
		t.Errorf("expected mysql api tokens unique index")
	}
}

func upSection(sql string) string {
	idx := strings.Index(sql, "-- +goose Down")
	if idx < 0 {
		return sql
	}
	return sql[:idx]
}

func TestMigrationFileNamingConvention(t *testing.T) {
	timestampRegex := regexp.MustCompile(`^(\d{14})_([a-z0-9_]+)\.sql$`)

	for _, dialect := range []string{"postgres", "mysql"} {
		dir := filepath.Join("..", "..", "migrations", dialect)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("failed to read %s migrations dir: %v", dialect, err)
		}

		if len(entries) == 0 {
			t.Fatalf("no migrations found in %s", dialect)
		}

		var lastVersion int64
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			matches := timestampRegex.FindStringSubmatch(entry.Name())
			if matches == nil {
				t.Errorf("%s migration %q does not match YYYYMMDDHHMMSS_<name>.sql format", dialect, entry.Name())
				continue
			}

			version, err := goose.NumericComponent(entry.Name())
			if err != nil {
				t.Errorf("goose failed to parse numeric component from %q: %v", entry.Name(), err)
			}
			if version <= lastVersion {
				t.Errorf("migration %q version %d not strictly greater than previous version %d", entry.Name(), version, lastVersion)
			}
			lastVersion = version
		}
	}
}