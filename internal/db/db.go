package db

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

// OpenDB opens a database connection given driver and dsn.
func OpenDB(driver, dsn string) (*sql.DB, error) {
	sqlDriver := driver
	if driver == "postgres" || driver == "postgresql" {
		sqlDriver = "postgres"
	} else if driver == "mysql" {
		sqlDriver = "mysql"
	} else {
		return nil, fmt.Errorf("unsupported database driver (%s): supported drivers are 'postgres', 'mysql'", driver)
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

// RunMigrations runs goose migrations from embed.FS or directory for the specified dialect.
func RunMigrations(database *sql.DB, driver string, migrationFS embed.FS, dir string) error {
	dialect := driver
	if driver == "postgres" || driver == "postgresql" {
		dialect = "postgres"
	} else if driver == "mysql" {
		dialect = "mysql"
	} else {
		return fmt.Errorf("unsupported migration dialect (%s): supported dialects are 'postgres', 'mysql'", driver)
	}

	if err := goose.SetDialect(dialect); err != nil {
		return fmt.Errorf("failed to set goose dialect (%s): %w", dialect, err)
	}

	goose.SetBaseFS(migrationFS)

	targetDir := dir
	candidates := []string{
		dir + "/" + dialect,
	}

	for _, candidate := range candidates {
		if entries, err := migrationFS.ReadDir(candidate); err == nil && len(entries) > 0 {
			targetDir = candidate
			break
		}
	}

	log.Printf("[DB] Running migrations with dialect: %s (dir: %s)", dialect, targetDir)
	if err := goose.Up(database, targetDir); err != nil {
		return fmt.Errorf("failed to run goose up migrations: %w", err)
	}

	return nil
}

// Rebind replaces '?' positional placeholders with '$1', '$2', ... for PostgreSQL.
func Rebind(driver, query string) string {
	if driver != "postgres" && driver != "postgresql" {
		return query
	}
	var b strings.Builder
	paramIdx := 1
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			fmt.Fprintf(&b, "$%d", paramIdx)
			paramIdx++
		} else {
			b.WriteByte(query[i])
		}
	}
	return b.String()
}

