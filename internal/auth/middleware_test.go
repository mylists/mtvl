package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddleware(t *testing.T) {
	provider := NewJWTAuthProvider(nil, "secret-key")
	token, err := provider.generateToken(&User{ID: "11111111-1111-4111-8111-111111111111", Username: "alice", Email: "alice@example.com"})
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// Protected handler checking context user
	protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxUser, ok := GetUserFromContext(r.Context())
		if !ok || ctxUser.Username != "alice" {
			t.Errorf("expected context user alice, got %+v", ctxUser)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	middleware := Middleware(provider)(protectedHandler)

	// Test 1: Missing Header -> 401
	req := httptest.NewRequest("GET", "/protected", nil)
	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}

	// Test 2: Invalid Header Format -> 401
	req = httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Basic "+token)
	rr = httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}

	// Test 3: Valid Bearer Token -> 200 OK
	req = httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	// Test 4: Valid 'token' prefix -> 200 OK
	req = httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "token "+token)
	rr = httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestAuthMiddlewareWithAPIToken(t *testing.T) {
	db := setupTestGormDB(t)
	provider := NewJWTAuthProvider(db, "secret-key")
	ctx := context.Background()

	user, err := provider.RegisterUser(ctx, "bob", "bob@example.com", "password")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	apiToken, err := provider.CreateAPIToken(ctx, user.ID, "CLI")
	if err != nil {
		t.Fatalf("failed to create api token: %v", err)
	}

	protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxUser, ok := GetUserFromContext(r.Context())
		if !ok || ctxUser.Username != "bob" {
			t.Errorf("expected context user bob, got %+v", ctxUser)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	middleware := Middleware(provider)(protectedHandler)

	// 1. Authorization: Bearer <128-char token>
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+apiToken.Token)
	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK via Bearer API token, got %d: %s", rr.Code, rr.Body.String())
	}

	// 2. X-API-Key: <128-char token>
	req = httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("X-API-Key", apiToken.Token)
	rr = httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK via X-API-Key header, got %d: %s", rr.Code, rr.Body.String())
	}

	// 3. X-API-Token: <128-char token>
	req = httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("X-API-Token", apiToken.Token)
	rr = httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK via X-API-Token header, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestExternalAuthProviderAdapter(t *testing.T) {
	extProvider := NewExternalAuthProvider("https://auth.example.com", "my-app")
	user, err := extProvider.VerifyToken(context.Background(), "valid-external-token")
	if err != nil {
		t.Fatalf("expected successful verification of external token: %v", err)
	}
	if user.Username != "external_user" {
		t.Errorf("expected external_user, got %s", user.Username)
	}
}