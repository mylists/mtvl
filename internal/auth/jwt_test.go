package auth

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"mtvl/internal/idgen"
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
	if user.Username != "testuser" || user.Email != "test@example.com" {
		t.Errorf("unexpected user data: %+v", user)
	}
	if _, ok := idgen.Parse(user.ID); !ok {
		t.Errorf("expected unique UUID id, got %q", user.ID)
	}

	other, err := provider.RegisterUser(ctx, "otheruser", "other@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to register second user: %v", err)
	}
	if _, ok := idgen.Parse(other.ID); !ok {
		t.Errorf("expected unique UUID id, got %q", other.ID)
	}
	if other.ID == user.ID {
		t.Errorf("expected distinct user ids, got %q twice", user.ID)
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
	if verified.Username != "testuser" || verified.ID != user.ID {
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

func TestVerifyTokenResolvesLegacyNumericUserID(t *testing.T) {
	db := setupTestGormDB(t)
	provider := NewJWTAuthProvider(db, "test-secret")
	ctx := context.Background()

	user, err := provider.RegisterUser(ctx, "testuser", "test@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  1,
		"username": "testuser",
		"email":    "test@example.com",
		"exp":      time.Now().Add(time.Hour).Unix(),
	})
	tokenStr, err := tok.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("failed to sign legacy token: %v", err)
	}

	verified, err := provider.VerifyToken(ctx, tokenStr)
	if err != nil {
		t.Fatalf("expected legacy numeric user_id token to resolve: %v", err)
	}
	if verified.ID != user.ID {
		t.Fatalf("expected resolved uuid %q, got %q", user.ID, verified.ID)
	}
	if _, ok := idgen.Parse(verified.ID); !ok {
		t.Fatalf("resolved id must be a uuid, got %q", verified.ID)
	}
}

