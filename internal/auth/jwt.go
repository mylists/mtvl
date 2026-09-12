package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"mtvl/internal/idgen"
)

// UserModel is the internal database model for users.
type UserModel struct {
	ID           string    `gorm:"primaryKey;type:uuid;size:36;column:id"`
	Username     string    `gorm:"uniqueIndex;not null;column:username"`
	Email        string    `gorm:"uniqueIndex;not null;column:email"`
	PasswordHash string    `gorm:"not null;column:password_hash"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

func (UserModel) TableName() string {
	return "users"
}

func (u *UserModel) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = idgen.New()
	}
	return nil
}

// APITokenModel is the internal database model for API tokens.
type APITokenModel struct {
	ID         string     `gorm:"primaryKey;type:uuid;size:36;column:id"`
	UserID     string     `gorm:"type:uuid;size:36;not null;column:user_id;index"`
	Token      string     `gorm:"type:varchar(128);size:128;not null;uniqueIndex;column:token"`
	Name       string     `gorm:"type:varchar(100);column:name"`
	CreatedAt  time.Time  `gorm:"column:created_at"`
	LastUsedAt *time.Time `gorm:"column:last_used_at"`
}

func (APITokenModel) TableName() string {
	return "api_tokens"
}

func (t *APITokenModel) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = idgen.New()
	}
	return nil
}

// GenerateAPITokenString generates a cryptographically secure token.
func GenerateAPITokenString() (string, error) {
	bytes := make([]byte, 64)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes for api token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// JWTAuthProvider implements AuthProvider using JWT and SQLite/Postgres.
type JWTAuthProvider struct {
	db        *gorm.DB
	jwtSecret []byte
	tokenTTL  time.Duration
}

// TokenUserID accepts UUID strings and legacy numeric user ids in JWTs.
type TokenUserID string

func (id *TokenUserID) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		*id = ""
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*id = TokenUserID(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	*id = TokenUserID(n.String())
	return nil
}

func (id TokenUserID) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(id))
}

// Claims defines standard JWT claims with User info.
type Claims struct {
	UserID   TokenUserID `json:"user_id"`
	Username string      `json:"username"`
	Email    string      `json:"email"`
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

// VerifyToken validates a JWT token or an API token and extracts User details.
func (p *JWTAuthProvider) VerifyToken(ctx context.Context, tokenString string) (*User, error) {
	tokenString = strings.TrimSpace(tokenString)
	if tokenString == "" {
		return nil, ErrInvalidToken
	}

	// 1. Check if token matches API token in database
	if p.db != nil {
		var tokenRecord APITokenModel
		if err := p.db.WithContext(ctx).Where("token = ?", tokenString).First(&tokenRecord).Error; err == nil {
			var u UserModel
			if err := p.db.WithContext(ctx).Where("id = ?", idgen.Arg(tokenRecord.UserID)).First(&u).Error; err == nil {
				now := time.Now()
				_ = p.db.WithContext(ctx).Model(&tokenRecord).Update("last_used_at", now)
				return &User{
					ID:        u.ID,
					Username:  u.Username,
					Email:     u.Email,
					CreatedAt: u.CreatedAt,
				}, nil
			}
		}
	}

	// 2. Otherwise validate JWT token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return p.jwtSecret, nil
	})

	if err != nil || !token.Valid {
		// Fallback check API token in case of token format
		if p.db != nil {
			var tokenRecord APITokenModel
			if err := p.db.WithContext(ctx).Where("token = ?", tokenString).First(&tokenRecord).Error; err == nil {
				var u UserModel
				if err := p.db.WithContext(ctx).Where("id = ?", idgen.Arg(tokenRecord.UserID)).First(&u).Error; err == nil {
					now := time.Now()
					_ = p.db.WithContext(ctx).Model(&tokenRecord).Update("last_used_at", now)
					return &User{
						ID:        u.ID,
						Username:  u.Username,
						Email:     u.Email,
						CreatedAt: u.CreatedAt,
					}, nil
				}
			}
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, ErrInvalidToken
	}

	userID := string(claims.UserID)
	if p.db == nil {
		return &User{
			ID:       userID,
			Username: claims.Username,
			Email:    claims.Email,
		}, nil
	}

	var u UserModel
	q := p.db.WithContext(ctx)
	if id, ok := idgen.Parse(userID); ok {
		err = q.Where("id = ?", idgen.Arg(id)).First(&u).Error
	} else if claims.Username != "" {
		// Legacy tokens still carry serial integer user ids.
		err = q.Where("username = ?", claims.Username).First(&u).Error
	} else {
		return nil, ErrInvalidToken
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrInvalidToken
	} else if err != nil {
		return nil, fmt.Errorf("failed to load user: %w", err)
	}

	return &User{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
	}, nil
}

// CreateAPIToken generates and stores a new API token for the specified user.
func (p *JWTAuthProvider) CreateAPIToken(ctx context.Context, userID, name string) (*APIToken, error) {
	if p.db == nil {
		return nil, errors.New("database not available")
	}

	var user UserModel
	if err := p.db.WithContext(ctx).Where("id = ?", idgen.Arg(userID)).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to load user: %w", err)
	}

	tokenStr, err := GenerateAPITokenString()
	if err != nil {
		return nil, err
	}

	tokenRecord := APITokenModel{
		UserID:    user.ID,
		Token:     tokenStr,
		Name:      strings.TrimSpace(name),
		CreatedAt: time.Now(),
	}

	if err := p.db.WithContext(ctx).Create(&tokenRecord).Error; err != nil {
		return nil, fmt.Errorf("failed to save api token: %w", err)
	}

	return &APIToken{
		ID:         tokenRecord.ID,
		UserID:     tokenRecord.UserID,
		Token:      tokenRecord.Token,
		Name:       tokenRecord.Name,
		CreatedAt:  tokenRecord.CreatedAt,
		LastUsedAt: tokenRecord.LastUsedAt,
	}, nil
}

// ListAPITokens returns all active API tokens for the specified user.
func (p *JWTAuthProvider) ListAPITokens(ctx context.Context, userID string) ([]APIToken, error) {
	if p.db == nil {
		return nil, errors.New("database not available")
	}

	var records []APITokenModel
	if err := p.db.WithContext(ctx).Where("user_id = ?", idgen.Arg(userID)).Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("failed to query api tokens: %w", err)
	}

	tokens := make([]APIToken, len(records))
	for i, r := range records {
		tokens[i] = APIToken{
			ID:         r.ID,
			UserID:     r.UserID,
			Token:      r.Token,
			Name:       r.Name,
			CreatedAt:  r.CreatedAt,
			LastUsedAt: r.LastUsedAt,
		}
	}
	return tokens, nil
}

// RevokeAPIToken removes an API token belonging to the specified user.
func (p *JWTAuthProvider) RevokeAPIToken(ctx context.Context, userID, tokenID string) error {
	if p.db == nil {
		return errors.New("database not available")
	}

	res := p.db.WithContext(ctx).Where("user_id = ? AND (id = ? OR token = ?)", idgen.Arg(userID), idgen.Arg(tokenID), tokenID).Delete(&APITokenModel{})
	if res.Error != nil {
		return fmt.Errorf("failed to delete api token: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrTokenNotFound
	}
	return nil
}

// UpdateUser updates user's profile details.
func (p *JWTAuthProvider) UpdateUser(ctx context.Context, userID string, username, email string) (*User, error) {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(strings.ToLower(email))

	if username == "" || email == "" {
		return nil, fmt.Errorf("username and email cannot be empty")
	}

	res := p.db.WithContext(ctx).Model(&UserModel{}).Where("id = ?", idgen.Arg(userID)).Updates(map[string]interface{}{
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
	if err := p.db.WithContext(ctx).Where("id = ?", idgen.Arg(userID)).First(&u).Error; err != nil {
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
func (p *JWTAuthProvider) ChangePassword(ctx context.Context, userID string, oldPassword, newPassword string) error {
	if newPassword == "" {
		return fmt.Errorf("new password cannot be empty")
	}
	if oldPassword == newPassword {
		return ErrSamePassword
	}

	var u UserModel
	err := p.db.WithContext(ctx).Select("id", "password_hash").Where("id = ?", idgen.Arg(userID)).First(&u).Error
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

	if err := p.db.WithContext(ctx).Model(&UserModel{}).Where("id = ?", idgen.Arg(userID)).Update("password_hash", string(newHash)).Error; err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// DeleteUser deletes the user account. Shared category items are left in place; list links cascade away.
func (p *JWTAuthProvider) DeleteUser(ctx context.Context, userID string) error {
	err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("id = ?", idgen.Arg(userID)).Delete(&UserModel{})
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
		UserID:   TokenUserID(user.ID),
		Username: user.Username,
		Email:    user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(p.tokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   user.ID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(p.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenStr, nil
}