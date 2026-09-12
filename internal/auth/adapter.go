package auth

import (
	"context"
	"errors"
	"fmt"
)

// ExternalAuthProvider is a adapter template showing how third-party auth providers
// (Auth0, Clerk, Supabase, Keycloak, Firebase, etc.) can be hooked into the application.
type ExternalAuthProvider struct {
	IssuerURL string
	Audience  string
}

// NewExternalAuthProvider creates a new adapter for an external auth service.
func NewExternalAuthProvider(issuerURL, audience string) *ExternalAuthProvider {
	return &ExternalAuthProvider{
		IssuerURL: issuerURL,
		Audience:  audience,
	}
}

func (e *ExternalAuthProvider) RegisterUser(ctx context.Context, username, email, password string) (*User, error) {
	return nil, errors.New("registration handled by external auth provider portal")
}

func (e *ExternalAuthProvider) AuthenticateUser(ctx context.Context, usernameOrEmail, password string) (string, *User, error) {
	return "", nil, errors.New("authentication handled by external auth provider portal")
}

// VerifyToken validates a token against the external auth provider's public keys / JWKS.
func (e *ExternalAuthProvider) VerifyToken(ctx context.Context, tokenString string) (*User, error) {
	if tokenString == "" {
		return nil, ErrInvalidToken
	}

	// Example adapter mock logic / validation:
	// In production, this would verify JWKS signatures from IssuerURL.
	if tokenString == "valid-external-token" {
		return &User{
			ID:       "99999999-9999-4999-8999-999999999999",
			Username: "external_user",
			Email:    "external@example.com",
		}, nil
	}

	return nil, fmt.Errorf("external token verification failed: %w", ErrInvalidToken)
}

func (e *ExternalAuthProvider) UpdateUser(ctx context.Context, userID string, username, email string) (*User, error) {
	return nil, errors.New("user profile updates handled by external auth provider portal")
}

func (e *ExternalAuthProvider) ChangePassword(ctx context.Context, userID string, oldPassword, newPassword string) error {
	return errors.New("password updates handled by external auth provider portal")
}

func (e *ExternalAuthProvider) DeleteUser(ctx context.Context, userID string) error {
	return errors.New("account deletion handled by external auth provider portal")
}

func (e *ExternalAuthProvider) CreateAPIToken(ctx context.Context, userID string, name string) (*APIToken, error) {
	return nil, errors.New("api token generation handled by external auth provider portal")
}

func (e *ExternalAuthProvider) ListAPITokens(ctx context.Context, userID string) ([]APIToken, error) {
	return nil, errors.New("api token listing handled by external auth provider portal")
}

func (e *ExternalAuthProvider) RevokeAPIToken(ctx context.Context, userID string, tokenID string) error {
	return errors.New("api token revocation handled by external auth provider portal")
}