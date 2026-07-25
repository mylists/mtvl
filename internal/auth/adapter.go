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
			ID:       999,
			Username: "external_user",
			Email:    "external@example.com",
		}, nil
	}

	return nil, fmt.Errorf("external token verification failed: %w", ErrInvalidToken)
}
