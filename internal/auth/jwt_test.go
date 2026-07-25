package auth

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}

	createTableSQL := `
	CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username VARCHAR(100) NOT NULL UNIQUE,
		email VARCHAR(255) NOT NULL UNIQUE,
		password_hash VARCHAR(255) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := db.Exec(createTableSQL); err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}

	return db
}

func TestJWTAuthProviderRegistrationAndAuth(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	provider := NewJWTAuthProvider(db, "test-secret")
	ctx := context.Background()

	// Register User
	user, err := provider.RegisterUser(ctx, "testuser", "test@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}
	if user.Username != "testuser" || user.Email != "test@example.com" {
		t.Errorf("unexpected user data: %+v", user)
	}

	// Duplicate registration error
	_, err = provider.RegisterUser(ctx, "testuser", "test2@example.com", "password123")
	if err == nil {
		t.Errorf("expected error registering duplicate username, got nil")
	}

	// Authenticate User Success
	token, authUser, err := provider.AuthenticateUser(ctx, "testuser", "password123")
	if err != nil {
		t.Fatalf("failed to authenticate user: %v", err)
	}
	if token == "" {
		t.Errorf("expected non-empty JWT token")
	}
	if authUser.ID != user.ID {
		t.Errorf("expected user ID %d, got %d", user.ID, authUser.ID)
	}

	// Authenticate Invalid Password
	_, _, err = provider.AuthenticateUser(ctx, "testuser", "wrongpassword")
	if err != ErrInvalidCreds {
		t.Errorf("expected ErrInvalidCreds, got %v", err)
	}

	// Verify JWT Token
	verifiedUser, err := provider.VerifyToken(ctx, token)
	if err != nil {
		t.Fatalf("failed to verify valid token: %v", err)
	}
	if verifiedUser.ID != user.ID || verifiedUser.Username != "testuser" {
		t.Errorf("unexpected verified user: %+v", verifiedUser)
	}

	// Verify Invalid Token
	_, err = provider.VerifyToken(ctx, "invalid.token.string")
	if err == nil {
		t.Errorf("expected error verifying invalid token, got nil")
	}
}
