package auth

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidToken  = errors.New("invalid or expired authorization token")
	ErrUserNotFound  = errors.New("user not found")
	ErrInvalidCreds  = errors.New("invalid username or password")
	ErrUserExists    = errors.New("username or email already registered")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrSamePassword  = errors.New("new password cannot be the same as old password")
	ErrTokenNotFound = errors.New("api token not found")
)

type userCtxKey string

const UserContextKey userCtxKey = "authenticated_user"

// User represents an authenticated user in the system.
type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// APIToken represents a 128-character API token tied to a user.
type APIToken struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Token      string     `json:"token"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// AuthProvider defines the interface for pluggable authentication systems.
// This allows swapping between built-in JWT, Auth0, Clerk, Supabase, OIDC, etc.
type AuthProvider interface {
	RegisterUser(ctx context.Context, username, email, password string) (*User, error)
	AuthenticateUser(ctx context.Context, usernameOrEmail, password string) (string, *User, error)
	VerifyToken(ctx context.Context, tokenString string) (*User, error)
	UpdateUser(ctx context.Context, userID string, username, email string) (*User, error)
	ChangePassword(ctx context.Context, userID string, oldPassword, newPassword string) error
	DeleteUser(ctx context.Context, userID string) error
	CreateAPIToken(ctx context.Context, userID string, name string) (*APIToken, error)
	ListAPITokens(ctx context.Context, userID string) ([]APIToken, error)
	RevokeAPIToken(ctx context.Context, userID string, tokenID string) error
}

// WithUserContext injects an authenticated User into request Context.
func WithUserContext(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, UserContextKey, user)
}

// GetUserFromContext extracts the authenticated User from request Context.
func GetUserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(UserContextKey).(*User)
	return user, ok && user != nil
}