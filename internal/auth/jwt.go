package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// UserModel represents the users table structure for GORM.
type UserModel struct {
	ID           int64     `gorm:"primaryKey;autoIncrement;column:id"`
	Username     string    `gorm:"uniqueIndex;not null;column:username"`
	Email        string    `gorm:"uniqueIndex;not null;column:email"`
	PasswordHash string    `gorm:"column:password_hash;not null"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

func (UserModel) TableName() string {
	return "users"
}

// JWTAuthProvider implements AuthProvider using GORM database and JWT tokens.
type JWTAuthProvider struct {
	db        *gorm.DB
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
func NewJWTAuthProvider(db *gorm.DB, jwtSecret string) *JWTAuthProvider {
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

	userRecord := UserModel{
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
		CreatedAt:    time.Now(),
	}

	if err := p.db.WithContext(ctx).Create(&userRecord).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrUserExists
		}
		return nil, fmt.Errorf("failed to insert user: %w", err)
	}

	return &User{
		ID:        userRecord.ID,
		Username:  userRecord.Username,
		Email:     userRecord.Email,
		CreatedAt: userRecord.CreatedAt,
	}, nil
}

// AuthenticateUser checks user credentials and returns a JWT token string.
func (p *JWTAuthProvider) AuthenticateUser(ctx context.Context, usernameOrEmail, password string) (string, *User, error) {
	usernameOrEmail = strings.TrimSpace(usernameOrEmail)

	var u UserModel
	err := p.db.WithContext(ctx).Where("username = ? OR LOWER(email) = LOWER(?)", usernameOrEmail, usernameOrEmail).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil, ErrInvalidCreds
	} else if err != nil {
		return "", nil, fmt.Errorf("database query error: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return "", nil, ErrInvalidCreds
	}

	user := &User{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
	}

	tokenStr, err := p.generateToken(user)
	if err != nil {
		return "", nil, err
	}

	return tokenStr, user, nil
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

	res := p.db.WithContext(ctx).Model(&UserModel{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"username": username,
		"email":    email,
	})
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) || strings.Contains(res.Error.Error(), "UNIQUE") || strings.Contains(res.Error.Error(), "unique") || strings.Contains(res.Error.Error(), "duplicate") {
			return nil, ErrUserExists
		}
		return nil, fmt.Errorf("failed to update user: %w", res.Error)
	}

	var u UserModel
	if err := p.db.WithContext(ctx).First(&u, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to query updated user: %w", err)
	}

	return &User{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
	}, nil
}

// ChangePassword changes the user password after validating the old password.
func (p *JWTAuthProvider) ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error {
	if newPassword == "" {
		return fmt.Errorf("new password cannot be empty")
	}
	if oldPassword == newPassword {
		return ErrSamePassword
	}

	var u UserModel
	err := p.db.WithContext(ctx).Select("id", "password_hash").Where("id = ?", userID).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrUserNotFound
	} else if err != nil {
		return fmt.Errorf("database query error: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(oldPassword)); err != nil {
		return ErrInvalidCreds
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	if err := p.db.WithContext(ctx).Model(&UserModel{}).Where("id = ?", userID).Update("password_hash", string(newHash)).Error; err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// DeleteUser deletes the user and all associated data.
func (p *JWTAuthProvider) DeleteUser(ctx context.Context, userID int64) error {
	err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tx.Exec("DELETE FROM movies WHERE user_id = ?", userID)
		tx.Exec("DELETE FROM tv_shows WHERE user_id = ?", userID)
		tx.Exec("DELETE FROM books WHERE user_id = ?", userID)

		res := tx.Where("id = ?", userID).Delete(&UserModel{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrUserNotFound
		}
		return nil
	})

	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to delete user: %w", err)
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

