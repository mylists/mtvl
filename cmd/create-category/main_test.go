package main

import (
	"os"
	"path/filepath"
	"strings"
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
	if strings.Contains(handlerCode, `Model(&VideoGame{}).Where("user_id`) {
		t.Errorf("generated catalog queries should not scope items to a single user")
	}
	catalogStart := strings.Index(migSQL, "CREATE TABLE IF NOT EXISTS video_games")
	listStart := strings.Index(migSQL, "CREATE TABLE IF NOT EXISTS user_video_games")
	if catalogStart >= 0 && listStart > catalogStart && strings.Contains(migSQL[catalogStart:listStart], "user_id") {
		t.Errorf("category table should only store shared items")
	}
	if listStart < 0 {
		t.Errorf("expected user list join table")
	}
	if !strings.Contains(migSQL, "FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE") {
		t.Errorf("list table should unlink when a user is deleted")
	}
	if strings.Contains(migSQL, "SERIAL") || strings.Contains(migSQL, "AUTO_INCREMENT") {
		t.Errorf("generated category ids should be unique UUIDs, not incremental")
	}
	if !strings.Contains(migSQL, "UUID PRIMARY KEY") {
		t.Errorf("expected UUID PRIMARY KEY in generated postgres/sqlite migration")
	}
	if strings.Contains(migSQL, "user_id INTEGER") || strings.Contains(migSQL, "user_id INT ") {
		t.Errorf("generated user ids should be unique UUIDs, not incremental")
	}
	if !strings.Contains(migSQL, "user_id UUID NOT NULL") {
		t.Errorf("expected user_id UUID NOT NULL in generated postgres/sqlite migration")
	}
	if strings.Contains(modelCode, "UserID    int64") || strings.Contains(handlerCode, "userID int64") {
		t.Errorf("generated user ids should be unique strings, not serial ints")
	}
	if !strings.Contains(handlerCode, "idgen.Arg(user.ID)") || !strings.Contains(handlerCode, "idgen.Arg(userID)") {
		t.Errorf("generated queries should bind user ids as UUIDs, not raw integers")
	}
	if !strings.Contains(migSQL, "title VARCHAR(255) NOT NULL UNIQUE") {
		t.Errorf("expected title to be unique in generated migration")
	}
	if !strings.Contains(modelCode, "uniqueIndex") {
		t.Errorf("expected uniqueIndex tag on title in generated model")
	}

	mysqlSQL := generateMigrationSQL(name, "mysql")
	if strings.Contains(mysqlSQL, "AUTO_INCREMENT") || strings.Contains(mysqlSQL, "user_id INT") {
		t.Errorf("generated mysql user ids should be unique CHAR(36) values, not incremental")
	}
	if !strings.Contains(mysqlSQL, "user_id CHAR(36) NOT NULL") {
		t.Errorf("expected user_id CHAR(36) NOT NULL in generated mysql migration")
	}
	if !strings.Contains(mysqlSQL, "title VARCHAR(255) NOT NULL UNIQUE") {
		t.Errorf("expected title to be unique in generated mysql migration")
	}

	testCode := generateHandlerTestGo(name, "/api/v1/games")
	if len(testCode) == 0 {
		t.Errorf("expected non-empty test code")
	}
}