package auth

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/crypto/bcrypt"
)

func TestJWTAuthProviderRegisterUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	provider := NewJWTAuthProvider(db, "test-secret")
	ctx := context.Background()

	mock.ExpectExec("INSERT INTO users").
		WithArgs("testuser", "test@example.com", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	user, err := provider.RegisterUser(ctx, "testuser", "test@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}
	if user.ID != 1 || user.Username != "testuser" || user.Email != "test@example.com" {
		t.Errorf("unexpected user data: %+v", user)
	}
}

func TestJWTAuthProviderAuthenticateUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	provider := NewJWTAuthProvider(db, "test-secret")
	ctx := context.Background()

	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "username", "email", "password_hash", "created_at"}).
		AddRow(1, "testuser", "test@example.com", string(hash), now)

	mock.ExpectQuery("SELECT id, username, email, password_hash, created_at FROM users").
		WithArgs("testuser", "testuser").
		WillReturnRows(rows)

	token, user, err := provider.AuthenticateUser(ctx, "testuser", "password123")
	if err != nil {
		t.Fatalf("failed to authenticate: %v", err)
	}
	if token == "" || user.ID != 1 {
		t.Errorf("unexpected authentication response: token=%s, user=%+v", token, user)
	}

	// Verify Token
	verified, err := provider.VerifyToken(ctx, token)
	if err != nil {
		t.Fatalf("failed to verify token: %v", err)
	}
	if verified.ID != 1 || verified.Username != "testuser" {
		t.Errorf("unexpected verified user: %+v", verified)
	}
}

func TestJWTAuthProviderUpdateAndChangePassword(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	provider := NewJWTAuthProvider(db, "test-secret")
	ctx := context.Background()

	// UpdateUser
	mock.ExpectExec("UPDATE users SET username = \\?, email = \\? WHERE id = \\?").
		WithArgs("newuser", "new@example.com", int64(1)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	rows := sqlmock.NewRows([]string{"id", "username", "email", "created_at"}).
		AddRow(1, "newuser", "new@example.com", time.Now())
	mock.ExpectQuery("SELECT id, username, email, created_at FROM users WHERE id = \\?").
		WithArgs(int64(1)).
		WillReturnRows(rows)

	updated, err := provider.UpdateUser(ctx, 1, "newuser", "new@example.com")
	if err != nil {
		t.Fatalf("failed to update user: %v", err)
	}
	if updated.Username != "newuser" {
		t.Errorf("expected newuser, got %s", updated.Username)
	}

	// ChangePassword
	hash, _ := bcrypt.GenerateFromPassword([]byte("oldpass"), bcrypt.DefaultCost)
	hashRows := sqlmock.NewRows([]string{"password_hash"}).AddRow(string(hash))

	mock.ExpectQuery("SELECT password_hash FROM users WHERE id = \\?").
		WithArgs(int64(1)).
		WillReturnRows(hashRows)

	mock.ExpectExec("UPDATE users SET password_hash = \\? WHERE id = \\?").
		WithArgs(sqlmock.AnyArg(), int64(1)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = provider.ChangePassword(ctx, 1, "oldpass", "newpass")
	if err != nil {
		t.Fatalf("failed to change password: %v", err)
	}
}
