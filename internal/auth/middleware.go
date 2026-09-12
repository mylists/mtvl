package auth

import (
	"net/http"
	"strings"
)

// Middleware returns an HTTP middleware enforcing authorization.
func Middleware(provider AuthProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			tokenString := ""

			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && (parts[0] == "Bearer" || parts[0] == "token") {
					tokenString = parts[1]
				} else if len(parts) == 1 && parts[0] != "" {
					tokenString = parts[0]
				} else {
					http.Error(w, `{"error":"Invalid Authorization header format"}`, http.StatusUnauthorized)
					return
				}
			} else if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
				tokenString = apiKey
			} else if apiToken := r.Header.Get("X-API-Token"); apiToken != "" {
				tokenString = apiToken
			}

			if tokenString == "" {
				http.Error(w, `{"error":"Missing Authorization header"}`, http.StatusUnauthorized)
				return
			}

			user, err := provider.VerifyToken(r.Context(), tokenString)
			if err != nil {
				http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusUnauthorized)
				return
			}

			ctx := WithUserContext(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}