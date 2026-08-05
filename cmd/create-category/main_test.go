package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCategoryGenerator(t *testing.T) {
	tempDir := t.TempDir()

	migrationsDir := filepath.Join(tempDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatalf("failed to create temp migrations dir: %v", err)
	}

	// Create dummy migration 00001_users.sql
	_ = writeFile(filepath.Join(migrationsDir, "00001_users.sql"), "-- +goose Up\n")

	seq := getNextMigrationSeq(migrationsDir)
	if seq != 2 {
		t.Errorf("expected next migration seq 2, got %d", seq)
	}

	name := "video_games"
	structName := getStructName(name)
	if structName != "VideoGame" {
		t.Errorf("expected VideoGame, got %s", structName)
	}

	migSQL := generateMigrationSQL(name, "sqlite")
	if !filepath.IsAbs(migrationsDir) {
		t.Errorf("expected valid migration SQL output")
	}
	if len(migSQL) == 0 {
		t.Errorf("expected non-empty migration SQL")
	}

	modelCode := generateModelGo(name)
	if len(modelCode) == 0 {
		t.Errorf("expected non-empty model code")
	}

	handlerCode := generateHandlerGo(name, "Video Games", "Track video games", "/api/v1/games")
	if len(handlerCode) == 0 {
		t.Errorf("expected non-empty handler code")
	}

	testCode := generateHandlerTestGo(name, "/api/v1/games")
	if len(testCode) == 0 {
		t.Errorf("expected non-empty test code")
	}
}
