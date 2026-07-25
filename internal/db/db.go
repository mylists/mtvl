package db

import (
	"database/sql"
	"embed"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

// OpenDB opens a database connection given driver and dsn.
func OpenDB(driver, dsn string) (*sql.DB, error) {
	sqlDriver := driver
	if driver == "sqlite3" || driver == "sqlite" {
		sqlDriver = "sqlite"
	}

	database, err := sql.Open(sqlDriver, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database (%s): %w", driver, err)
	}

	if err := database.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database (%s): %w", driver, err)
	}

	return database, nil
}

// RunMigrations runs goose migrations from embed.FS or directory.
func RunMigrations(database *sql.DB, driver string, migrationFS embed.FS, dir string) error {
	dialect := driver
	if driver == "sqlite3" || driver == "sqlite" {
		dialect = "sqlite3"
	} else if driver == "postgres" {
		dialect = "postgres"
	} else if driver == "mysql" {
		dialect = "mysql"
	}

	if err := goose.SetDialect(dialect); err != nil {
		return fmt.Errorf("failed to set goose dialect (%s): %w", dialect, err)
	}

	goose.SetBaseFS(migrationFS)

	log.Printf("[DB] Running migrations with dialect: %s", dialect)
	if err := goose.Up(database, dir); err != nil {
		return fmt.Errorf("failed to run goose up migrations: %w", err)
	}

	return nil
}
