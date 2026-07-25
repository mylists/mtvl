package db

import (
	"embed"
	"testing"
)

// Embed test migrations
//
//go:embed test_migrations/*.sql
var testMigrationFS embed.FS

func TestOpenDBAndMigrations(t *testing.T) {
	database, err := OpenDB("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite db: %v", err)
	}
	defer database.Close()

	err = RunMigrations(database, "sqlite3", testMigrationFS, "test_migrations")
	if err != nil {
		t.Fatalf("failed to run test migrations: %v", err)
	}

	var count int
	err = database.QueryRow("SELECT COUNT(*) FROM test_table").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query test_table: %v", err)
	}
}
