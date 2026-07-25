package config

import (
	"os"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	os.Unsetenv("SERVER_PORT")
	os.Unsetenv("DB_DRIVER")
	os.Unsetenv("DB_DSN")
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("AUTH_PROVIDER")

	cfg := LoadConfig()

	if cfg.ServerPort != "8080" {
		t.Errorf("expected default ServerPort 8080, got %s", cfg.ServerPort)
	}
	if cfg.DBDriver != "postgres" {
		t.Errorf("expected default DBDriver postgres, got %s", cfg.DBDriver)
	}
	if cfg.DBDSN != "postgres://postgres:postgres@localhost:5432/mtvl?sslmode=disable" {
		t.Errorf("expected default DBDSN postgres://postgres:postgres@localhost:5432/mtvl?sslmode=disable, got %s", cfg.DBDSN)
	}
	if cfg.JWTSecret != "super-secret-key-change-in-production" {
		t.Errorf("expected default JWTSecret, got %s", cfg.JWTSecret)
	}
	if cfg.AuthProviderType != "jwt" {
		t.Errorf("expected default AuthProviderType jwt, got %s", cfg.AuthProviderType)
	}
}

func TestLoadConfigEnvOverrides(t *testing.T) {
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("DB_DSN", "postgres://user:pass@localhost:5432/mtvldb?sslmode=disable")
	t.Setenv("JWT_SECRET", "customsecret")
	t.Setenv("AUTH_PROVIDER", "external")

	cfg := LoadConfig()

	if cfg.ServerPort != "9090" {
		t.Errorf("expected ServerPort 9090, got %s", cfg.ServerPort)
	}
	if cfg.DBDriver != "postgres" {
		t.Errorf("expected DBDriver postgres, got %s", cfg.DBDriver)
	}
	if cfg.DBDSN != "postgres://user:pass@localhost:5432/mtvldb?sslmode=disable" {
		t.Errorf("expected postgres DSN, got %s", cfg.DBDSN)
	}
	if cfg.JWTSecret != "customsecret" {
		t.Errorf("expected customsecret, got %s", cfg.JWTSecret)
	}
	if cfg.AuthProviderType != "external" {
		t.Errorf("expected external, got %s", cfg.AuthProviderType)
	}
}
