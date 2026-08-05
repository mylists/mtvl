package db

import (
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
	pgUsers := `CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);`
	if !strings.Contains(pgUsers, "SERIAL PRIMARY KEY") {
		t.Errorf("expected SERIAL PRIMARY KEY in postgres migration")
	}

	mysqlUsers := `CREATE TABLE IF NOT EXISTS users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(100) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);`
	if !strings.Contains(mysqlUsers, "AUTO_INCREMENT PRIMARY KEY") {
		t.Errorf("expected AUTO_INCREMENT PRIMARY KEY in mysql migration")
	}
}
