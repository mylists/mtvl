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
	pgUsers, err := os.ReadFile(filepath.Join("..", "..", "migrations", "postgres", "20260804191235_users.sql"))
	if err != nil {
		t.Fatalf("failed to read postgres users migration: %v", err)
	}
	if !strings.Contains(string(pgUsers), "id UUID PRIMARY KEY") {
		t.Errorf("expected postgres user ids to be UUID")
	}

	mysqlUsers, err := os.ReadFile(filepath.Join("..", "..", "migrations", "mysql", "20260804191235_users.sql"))
	if err != nil {
		t.Fatalf("failed to read mysql users migration: %v", err)
	}
	if !strings.Contains(string(mysqlUsers), "CHAR(36)") {
		t.Errorf("expected mysql user ids to be CHAR(36)")
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
