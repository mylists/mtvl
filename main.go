package main

import (
	"context"
	"embed"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"mtvl/config"
	"mtvl/internal/auth"
	"mtvl/internal/core"
	"mtvl/internal/db"
	"mtvl/internal/modules/movies"
	"mtvl/internal/modules/tvshows"
)

// Embed goose SQL migrations
//
//go:embed migrations/*.sql
var migrationFS embed.FS

func main() {
	cfg := config.LoadConfig()
	log.Printf("[Init] Starting mtvl backend server on port %s", cfg.ServerPort)
	log.Printf("[Init] Database driver: %s, DSN: %s", cfg.DBDriver, cfg.DBDSN)

	database, err := db.OpenDB(cfg.DBDriver, cfg.DBDSN)
	if err != nil {
		log.Fatalf("Fatal: failed to connect to database: %v", err)
	}
	defer database.Close()

	if err := db.RunMigrations(database, cfg.DBDriver, migrationFS, "migrations"); err != nil {
		log.Fatalf("Fatal: database migration failed: %v", err)
	}

	var authProvider auth.AuthProvider
	if cfg.AuthProviderType == "external" {
		authProvider = auth.NewExternalAuthProvider("https://auth.example.com", "mtvl")
		log.Printf("[Auth] Initialized External Auth Provider")
	} else {
		authProvider = auth.NewJWTAuthProvider(database, cfg.JWTSecret)
		log.Printf("[Auth] Initialized Built-in JWT Auth Provider")
	}

	registry := core.NewRegistry()
	registry.Register(movies.NewModule(database))
	registry.Register(tvshows.NewModule(database))

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	// Healthcheck endpoint
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "up"})
	})

	// Auth Endpoints
	router.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/register", func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Username string `json:"username"`
				Email    string `json:"email"`
				Password string `json:"password"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
				return
			}
			user, err := authProvider.RegisterUser(r.Context(), req.Username, req.Email, req.Password)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(user)
		})

		r.Post("/login", func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				UsernameOrEmail string `json:"username_or_email"`
				Password        string `json:"password"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
				return
			}
			token, user, err := authProvider.AuthenticateUser(r.Context(), req.UsernameOrEmail, req.Password)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"token": token,
				"user":  user,
			})
		})

		r.With(auth.Middleware(authProvider)).Get("/me", func(w http.ResponseWriter, r *http.Request) {
			user, _ := auth.GetUserFromContext(r.Context())
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(user)
		})
	})

	// Register Category Extensions API Routes
	registry.RegisterAllRoutes(router, auth.Middleware(authProvider))

	server := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[Server] Listening on http://localhost:%s", cfg.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Fatal: server error: %v", err)
		}
	}()

	<-stop
	log.Println("[Server] Shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("[Server] Graceful shutdown failed: %v", err)
	}
	log.Println("[Server] Server stopped.")
}
