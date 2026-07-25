package config

import (
	"os"
)

// Config holds all configuration options for the application.
type Config struct {
	ServerPort       string
	DBDriver         string // "sqlite3", "postgres", "mysql"
	DBDSN            string
	JWTSecret        string
	AuthProviderType string // "jwt", "external"
}

// LoadConfig loads configuration from environment variables with sensible defaults.
func LoadConfig() *Config {
	port := getEnv("SERVER_PORT", "8080")
	driver := getEnv("DB_DRIVER", "postgres")
	dsn := getEnv("DB_DSN", "postgres://postgres:postgres@localhost:5432/mtvl?sslmode=disable")
	secret := getEnv("JWT_SECRET", "super-secret-key-change-in-production")
	authType := getEnv("AUTH_PROVIDER", "jwt")

	return &Config{
		ServerPort:       port,
		DBDriver:         driver,
		DBDSN:            dsn,
		JWTSecret:        secret,
		AuthProviderType: authType,
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
