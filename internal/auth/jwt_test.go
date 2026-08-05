package auth

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestGormDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}

	if err := db.AutoMigrate(&UserModel{}); err != nil {
		t.Fatalf("failed to migrate tables: %v", err)
	}

	return db
}

func TestJWTAuthProviderRegisterUser(t *testing.T) {
	db := setupTestGormDB(t)

	provider := NewJWTAuthProvider(db, "test-secret")
	ctx := context.Background()

	user, err := provider.RegisterUser(ctx, "testuser", "test@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}
	if user.ID == 0 || user.Username != "testuser" || user.Email != "test@example.com" {
		t.Errorf("unexpected user data: %+v", user)
	}
}

func TestJWTAuthProviderAuthenticateUser(t *testing.T) {
	db := setupTestGormDB(t)

	provider := NewJWTAuthProvider(db, "test-secret")
	ctx := context.Background()

	_, err := provider.RegisterUser(ctx, "testuser", "test@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	token, user, err := provider.AuthenticateUser(ctx, "testuser", "password123")
	if err != nil {
		t.Fatalf("failed to authenticate: %v", err)
	}
	if token == "" || user.Username != "testuser" {
		t.Errorf("unexpected authentication response: token=%s, user=%+v", token, user)
	}

	// Verify Token
	verified, err := provider.VerifyToken(ctx, token)
	if err != nil {
		t.Fatalf("failed to verify token: %v", err)
	}
	if verified.Username != "testuser" {
		t.Errorf("unexpected verified user: %+v", verified)
	}
}

func TestJWTAuthProviderUpdateAndChangePassword(t *testing.T) {
	db := setupTestGormDB(t)

	provider := NewJWTAuthProvider(db, "test-secret")
	ctx := context.Background()

	u, err := provider.RegisterUser(ctx, "testuser", "test@example.com", "oldpass")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	updated, err := provider.UpdateUser(ctx, u.ID, "newuser", "new@example.com")
	if err != nil {
		t.Fatalf("failed to update user: %v", err)
	}
	if updated.Username != "newuser" {
		t.Errorf("expected newuser, got %s", updated.Username)
	}

	err = provider.ChangePassword(ctx, u.ID, "oldpass", "newpass")
	if err != nil {
		t.Fatalf("failed to change password: %v", err)
	}
}

