package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	pgUsers, err := os.ReadFile(filepath.Join("..", "..", "migrations", "postgres", "00008_user_unique_ids.sql"))
	if err != nil {
		t.Fatalf("failed to read postgres user unique id migration: %v", err)
	}
	if !strings.Contains(string(pgUsers), "new_id UUID") {
		t.Errorf("expected postgres user ids to convert to UUID")
	}
	if strings.Contains(upSection(string(pgUsers)), "SERIAL") {
		t.Errorf("postgres user unique id up migration should not keep serial ids")
	}

	mysqlUsers, err := os.ReadFile(filepath.Join("..", "..", "migrations", "mysql", "00008_user_unique_ids.sql"))
	if err != nil {
		t.Fatalf("failed to read mysql user unique id migration: %v", err)
	}
	if !strings.Contains(string(mysqlUsers), "CHAR(36)") {
		t.Errorf("expected mysql user ids to convert to CHAR(36)")
	}
	if strings.Contains(upSection(string(mysqlUsers)), "AUTO_INCREMENT") {
		t.Errorf("mysql user unique id up migration should not keep auto increment ids")
	}
}

func upSection(sql string) string {
	idx := strings.Index(sql, "-- +goose Down")
	if idx < 0 {
		return sql
	}
	return sql[:idx]
}
