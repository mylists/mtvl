package auth

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// JWTAuthProvider implements AuthProvider using SQL database and JWT tokens.
type JWTAuthProvider struct {
	db        *sql.DB
	jwtSecret []byte
	tokenTTL  time.Duration
}

// Claims defines standard JWT claims with User info.
type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	jwt.RegisteredClaims
}

// NewJWTAuthProvider initializes a new JWTAuthProvider.
func NewJWTAuthProvider(db *sql.DB, jwtSecret string) *JWTAuthProvider {
	return &JWTAuthProvider{
		db:        db,
		jwtSecret: []byte(jwtSecret),
		tokenTTL:  24 * time.Hour,
	}
}

// RegisterUser registers a new user in the database.
func (p *JWTAuthProvider) RegisterUser(ctx context.Context, username, email, password string) (*User, error) {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(strings.ToLower(email))

	if username == "" || email == "" || password == "" {
		return nil, fmt.Errorf("username, email, and password are required")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	query := `INSERT INTO users (username, email, password_hash) VALUES (?, ?, ?)`
	res, err := p.db.ExecContext(ctx, query, username, email, string(hash))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrUserExists
		}
		return nil, fmt.Errorf("failed to insert user: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get user id: %w", err)
	}

	return &User{
		ID:        id,
		Username:  username,
		Email:     email,
		CreatedAt: time.Now(),
	}, nil
}

// AuthenticateUser checks user credentials and returns a JWT token string.
func (p *JWTAuthProvider) AuthenticateUser(ctx context.Context, usernameOrEmail, password string) (string, *User, error) {
	usernameOrEmail = strings.TrimSpace(usernameOrEmail)

	var u User
	var passwordHash string

	query := `SELECT id, username, email, password_hash, created_at FROM users WHERE username = ? OR LOWER(email) = LOWER(?)`
	err := p.db.QueryRowContext(ctx, query, usernameOrEmail, usernameOrEmail).Scan(&u.ID, &u.Username, &u.Email, &passwordHash, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return "", nil, ErrInvalidCreds
	} else if err != nil {
		return "", nil, fmt.Errorf("database query error: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return "", nil, ErrInvalidCreds
	}

	tokenStr, err := p.generateToken(&u)
	if err != nil {
		return "", nil, err
	}

	return tokenStr, &u, nil
}

// VerifyToken validates a JWT token and extracts User details.
func (p *JWTAuthProvider) VerifyToken(ctx context.Context, tokenString string) (*User, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return p.jwtSecret, nil
	})

	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, ErrInvalidToken
	}

	return &User{
		ID:       claims.UserID,
		Username: claims.Username,
		Email:    claims.Email,
	}, nil
}

// UpdateUser updates user's profile details.
func (p *JWTAuthProvider) UpdateUser(ctx context.Context, userID int64, username, email string) (*User, error) {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(strings.ToLower(email))

	if username == "" || email == "" {
		return nil, fmt.Errorf("username and email cannot be empty")
	}

	query := `UPDATE users SET username = ?, email = ? WHERE id = ?`
	_, err := p.db.ExecContext(ctx, query, username, email, userID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrUserExists
		}
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	var u User
	err = p.db.QueryRowContext(ctx, `SELECT id, username, email, created_at FROM users WHERE id = ?`, userID).Scan(&u.ID, &u.Username, &u.Email, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	} else if err != nil {
		return nil, fmt.Errorf("failed to query updated user: %w", err)
	}

	return &u, nil
}

// ChangePassword changes the user password after validating the old password.
func (p *JWTAuthProvider) ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error {
	if newPassword == "" {
		return fmt.Errorf("new password cannot be empty")
	}
	if oldPassword == newPassword {
		return ErrSamePassword
	}

	var passwordHash string
	err := p.db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = ?`, userID).Scan(&passwordHash)
	if err == sql.ErrNoRows {
		return ErrUserNotFound
	} else if err != nil {
		return fmt.Errorf("database query error: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(oldPassword)); err != nil {
		return ErrInvalidCreds
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	_, err = p.db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, string(newHash), userID)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// DeleteUser deletes the user and all associated data.
func (p *JWTAuthProvider) DeleteUser(ctx context.Context, userID int64) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Clean up user items across all tables
	_, _ = tx.ExecContext(ctx, `DELETE FROM movies WHERE user_id = ?`, userID)
	_, _ = tx.ExecContext(ctx, `DELETE FROM tv_shows WHERE user_id = ?`, userID)
	_, _ = tx.ExecContext(ctx, `DELETE FROM books WHERE user_id = ?`, userID)

	res, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrUserNotFound
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit user deletion: %w", err)
	}

	return nil
}

func (p *JWTAuthProvider) generateToken(user *User) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   user.ID,
		Username: user.Username,
		Email:    user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(p.tokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   fmt.Sprintf("%d", user.ID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(p.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenStr, nil
}
