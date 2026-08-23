# mtvl

`mtvl` is a modular, high-performance tracking backend built in Go. It allows developers to create custom tracking categories (such as movies, tv shows, books, video games, anime, etc.) through a pluggable module system and Goose SQL database migrations.

---

## Key Features

- **Extensible Category Modules**: Each category (e.g., movies, tv shows, books) is an isolated Go module implementing `core.CategoryModule`.
- **Multi-Database Support**: Driven by Goose migrations, supporting **PostgreSQL** and **MySQL**.
- **Pluggable Auth Subsystem**: Clean `auth.AuthProvider` interface with built-in JWT authentication + standard adapter for external providers (Auth0, Clerk, Supabase, Keycloak, or custom OIDC).
- **Dynamic Category Discovery**: Central registry automatically exposes active modules via GET `/api/v1/categories`.
- **Shared Categories**: Movies, TV shows, books, and any new category are one shared catalog. Every authenticated user sees the same items and the same IDs.

---

## Quick Start

### 1. Run Tests

```bash
make test
# or: go test -v ./...
```

### 2. Start the Server

```bash
# PostgreSQL (default)
go run main.go

# Custom PostgreSQL DSN
DB_DRIVER=postgres DB_DSN="postgres://user:pass@localhost:5432/mtvl?sslmode=disable" go run main.go

# MySQL
DB_DRIVER=mysql DB_DSN="user:pass@tcp(localhost:3306)/mtvl?parseTime=true" go run main.go
```

---

## How to Add a New Category Module (e.g., Books)

Adding a new tracking category requires only **3 steps**:

### Step 1: Add a Goose DB Migration

Create `migrations/00004_books.sql`:

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS books (
    id CHAR(36) PRIMARY KEY,
    user_id INTEGER NOT NULL,
    title VARCHAR(255) NOT NULL,
    author VARCHAR(255) DEFAULT '',
    pages_read INTEGER DEFAULT 0,
    total_pages INTEGER DEFAULT 0,
    status VARCHAR(50) DEFAULT 'plan_to_read',
    rating INTEGER DEFAULT 0,
    notes TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS books;
-- +goose StatementEnd
```

### Step 2: Implement `core.CategoryModule`

Create `internal/modules/books/handler.go`:

```go
package books

import (
    "database/sql"
    "net/http"
    "github.com/go-chi/chi/v5"
    "mtvl/internal/core"
)

type Module struct { db *sql.DB }

func NewModule(db *sql.DB) *Module { return &Module{db: db} }

func (m *Module) Info() core.CategoryInfo {
    return core.CategoryInfo{
        Category:    "books",
        DisplayName: "Books",
        Description: "Track reading progress and books",
        Endpoint:    "/api/v1/books",
    }
}

func (m *Module) RegisterRoutes(r chi.Router, authMw func(http.Handler) http.Handler) {
    r.Route("/api/v1/books", func(sub chi.Router) {
        sub.Use(authMw)
        // Attach CRUD handlers...
    })
}
```

### Step 3: Register in `main.go`

In `main.go`, add:

```go
registry.Register(books.NewModule(database))
```

That's it! The new category will automatically show up in `/api/v1/categories` and its endpoints will be protected by your auth middleware.

---

## API Endpoints Overview

| Method | Endpoint | Description | Auth Required |
| --- | --- | --- | --- |
| `GET` | `/health`, `/healthz` | Combined health check (includes Postgres) | No |
| `GET` | `/livez`, `/health/live` | Kubernetes liveness probe | No |
| `GET` | `/readyz`, `/health/ready` | Kubernetes readiness probe (includes Postgres) | No |
| `GET` | `/startupz`, `/health/startup` | Kubernetes startup probe (includes Postgres) | No |
| `POST` | `/api/v1/auth/register` | Register new user | No |
| `POST` | `/api/v1/auth/login` | Login user & get JWT token | No |
| `GET` | `/api/v1/auth/me` | Get current user info | Yes |
| `GET` | `/api/v1/categories` | Discover registered tracking modules | No |
| `GET` | `/api/v1/movies` | List movies (shared) | Yes |
| `POST` | `/api/v1/movies` | Create movie record | Yes |
| `GET` | `/api/v1/movies/{id}` | Get single movie | Yes |
| `PUT` | `/api/v1/movies/{id}` | Update movie | Yes |
| `DELETE` | `/api/v1/movies/{id}` | Delete movie | Yes |
| `GET` | `/api/v1/tvshows` | List TV shows (shared) | Yes |
| `POST` | `/api/v1/tvshows` | Create TV show record | Yes |
| `GET` | `/api/v1/tvshows/{id}` | Get single TV show | Yes |
| `PUT` | `/api/v1/tvshows/{id}` | Update TV show | Yes |
| `DELETE` | `/api/v1/tvshows/{id}` | Delete TV show | Yes |
