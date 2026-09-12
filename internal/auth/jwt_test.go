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

	if err := db.AutoMigrate(&UserModel{}, &APITokenModel{}); err != nil {
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

func TestAPITokenLifecycle(t *testing.T) {
	db := setupTestGormDB(t)
	provider := NewJWTAuthProvider(db, "test-secret")
	ctx := context.Background()

	user, err := provider.RegisterUser(ctx, "apitokenuser", "api@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	// 1. Create API Token
	apiToken, err := provider.CreateAPIToken(ctx, user.ID, "CI Runner")
	if err != nil {
		t.Fatalf("failed to create api token: %v", err)
	}
	if apiToken.UserID != user.ID {
		t.Errorf("expected user_id %s, got %s", user.ID, apiToken.UserID)
	}
	if len(apiToken.Token) != 128 {
		t.Fatalf("unexpected API token: %s (length %d)", apiToken.Token, len(apiToken.Token))
	}
	if apiToken.Name != "CI Runner" {
		t.Errorf("expected name 'CI Runner', got %s", apiToken.Name)
	}

	// 2. Verify with API Token
	verified, err := provider.VerifyToken(ctx, apiToken.Token)
	if err != nil {
		t.Fatalf("failed to verify API token: %v", err)
	}
	if verified.ID != user.ID || verified.Username != "apitokenuser" {
		t.Errorf("unexpected user from API token: %+v", verified)
	}

	// 3. List API Tokens
	tokens, err := provider.ListAPITokens(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to list tokens: %v", err)
	}
	if len(tokens) != 1 || tokens[0].ID != apiToken.ID {
		t.Fatalf("expected 1 token in list, got %+v", tokens)
	}

	// 4. Cross-user isolation
	other, _ := provider.RegisterUser(ctx, "otherapi", "otherapi@example.com", "pass")
	otherTokens, err := provider.ListAPITokens(ctx, other.ID)
	if err != nil {
		t.Fatalf("failed to list other tokens: %v", err)
	}
	if len(otherTokens) != 0 {
		t.Fatalf("expected 0 tokens for other user, got %+v", otherTokens)
	}

	// 5. Revoke API Token
	if err := provider.RevokeAPIToken(ctx, user.ID, apiToken.ID); err != nil {
		t.Fatalf("failed to revoke api token: %v", err)
	}

	// 6. Verify revoked token fails
	_, err = provider.VerifyToken(ctx, apiToken.Token)
	if err == nil {
		t.Fatalf("expected verification to fail after token revocation, but succeeded")
	}

	// 7. Verify list is now empty
	tokens, err = provider.ListAPITokens(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to list tokens after revocation: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("expected 0 tokens after revocation, got %+v", tokens)
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