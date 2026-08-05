package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddleware(t *testing.T) {
	provider := NewJWTAuthProvider(nil, "secret-key")
	token, err := provider.generateToken(&User{ID: 1, Username: "alice", Email: "alice@example.com"})
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

	// Test 3: Valid Token -> 200 OK
	req = httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestExternalAuthProviderAdapter(t *testing.T) {
	extProvider := NewExternalAuthProvider("https://auth.example.com", "my-app")
	user, err := extProvider.VerifyToken(t.Context(), "valid-external-token")
	if err != nil {
		t.Fatalf("expected successful verification of external token: %v", err)
	}
	if user.Username != "external_user" {
		t.Errorf("expected external_user, got %s", user.Username)
	}
}
