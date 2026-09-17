package docs

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type DocsHandler struct{}

func NewDocsHandler() *DocsHandler {
	return &DocsHandler{}
}

func (d *DocsHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/v1/openapi.json", d.GetOpenAPISpec)
	r.Get("/api/v1/docs", d.GetDocsUI)
}

func (d *DocsHandler) GetOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":       "API",
			"description": "Comprehensive RESTful API for backend",
			"version":     "4.0.0",
		},
		"servers": []map[string]interface{}{
			{
				"url":         "/",
				"description": "Current Server",
			},
		},
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"bearerAuth": map[string]interface{}{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
				},
			},
		},
		"paths": map[string]interface{}{
			"/health":         healthProbePath("Combined health check including Postgres"),
			"/healthz":        healthProbePath("Combined health check including Postgres (Kubernetes healthz)"),
			"/livez":          liveProbePath("Kubernetes liveness probe. Process only; does not check Postgres."),
			"/readyz":         healthProbePath("Kubernetes readiness probe. Requires Postgres to be reachable."),
			"/startupz":       healthProbePath("Kubernetes startup probe. Requires Postgres to be reachable."),
			"/health/live":    liveProbePath("Kubernetes liveness probe alias"),
			"/health/ready":   healthProbePath("Kubernetes readiness probe alias"),
			"/health/startup": healthProbePath("Kubernetes startup probe alias"),
			"/api/v1/categories": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "List registered category modules",
					"description": "Returns metadata of all active tracking modules",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "List of active tracking categories",
						},
					},
				},
			},
			"/api/v1/auth/register": map[string]interface{}{
				"post": map[string]interface{}{
					"summary": "Register a new user",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"username": map[string]interface{}{"type": "string"},
										"email":    map[string]interface{}{"type": "string"},
										"password": map[string]interface{}{"type": "string"},
									},
									"required": []string{"username", "email", "password"},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"201": map[string]interface{}{"description": "User registered"},
						"400": map[string]interface{}{"description": "Validation or registration error"},
					},
				},
			},
			"/api/v1/auth/login": map[string]interface{}{
				"post": map[string]interface{}{
					"summary": "Authenticate user and issue JWT token",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"username_or_email": map[string]interface{}{"type": "string"},
										"password":          map[string]interface{}{"type": "string"},
									},
									"required": []string{"username_or_email", "password"},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Successfully authenticated"},
						"401": map[string]interface{}{"description": "Invalid credentials"},
					},
				},
			},
			"/api/v1/auth/me": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":  "Get current user profile",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "User profile"},
						"401": map[string]interface{}{"description": "Unauthorized"},
					},
				},
				"put": map[string]interface{}{
					"summary":  "Update user profile (username / email)",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"requestBody": map[string]interface{}{
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"username": map[string]interface{}{"type": "string"},
										"email":    map[string]interface{}{"type": "string"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Profile updated"},
						"400": map[string]interface{}{"description": "Bad request"},
						"401": map[string]interface{}{"description": "Unauthorized"},
					},
				},
				"delete": map[string]interface{}{
					"summary":  "Delete user account. Shared category items are kept.",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Account deleted"},
						"401": map[string]interface{}{"description": "Unauthorized"},
					},
				},
			},
			"/api/v1/auth/password": map[string]interface{}{
				"put": map[string]interface{}{
					"summary":  "Change password",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"requestBody": map[string]interface{}{
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"old_password": map[string]interface{}{"type": "string"},
										"new_password": map[string]interface{}{"type": "string"},
									},
									"required": []string{"old_password", "new_password"},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Password updated"},
						"400": map[string]interface{}{"description": "Invalid old password or bad request"},
						"401": map[string]interface{}{"description": "Unauthorized"},
					},
				},
			},
			"/api/v1/auth/tokens": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":  "List user API tokens",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "List of active API tokens"},
						"401": map[string]interface{}{"description": "Unauthorized"},
					},
				},
				"post": map[string]interface{}{
					"summary":  "Create API token tied to current user",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"requestBody": map[string]interface{}{
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"name": map[string]interface{}{"type": "string", "description": "Token label / name"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"201": map[string]interface{}{"description": "API token created"},
						"401": map[string]interface{}{"description": "Unauthorized"},
					},
				},
			},
			"/api/v1/auth/tokens/{id}": map[string]interface{}{
				"delete": map[string]interface{}{
					"summary":  "Revoke an API token",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "schema": map[string]string{"type": "string"}, "description": "Token ID or token string"},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "API token revoked"},
						"404": map[string]interface{}{"description": "Token not found"},
						"401": map[string]interface{}{"description": "Unauthorized"},
					},
				},
			},
			"/api/v1/movies": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":  "List public movie catalog items with search, sorting, and pagination",
					"parameters": []map[string]interface{}{
						{"name": "q", "in": "query", "schema": map[string]string{"type": "string"}, "description": "Search term"},
						{"name": "sort_by", "in": "query", "schema": map[string]string{"type": "string"}, "description": "Column to sort by"},
						{"name": "order", "in": "query", "schema": map[string]string{"type": "string"}, "description": "asc or desc"},
						{"name": "page", "in": "query", "schema": map[string]string{"type": "integer"}, "description": "Page number"},
						{"name": "limit", "in": "query", "schema": map[string]string{"type": "integer"}, "description": "Items per page"},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "List or paginated list of movies"},
					},
				},
				"post": map[string]interface{}{
					"summary":  "Create movie catalog item",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"201": map[string]interface{}{"description": "Movie created"},
					},
				},
			},
			"/api/v1/movies/list": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":  "List movies on the current user's list",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "User movie list"},
					},
				},
				"post": map[string]interface{}{
					"summary":  "Add a catalog movie to the current user's list",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"201": map[string]interface{}{"description": "Movie added to list"},
					},
				},
			},
			"/api/v1/movies/bulk-delete": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":  "Bulk delete movie catalog items",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Movies deleted"},
					},
				},
			},
			"/api/v1/movies/bulk-status": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":  "Bulk update movies status on the current user's list",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Movies status updated"},
					},
				},
			},
			"/api/v1/tvshows": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":  "List public TV show catalog items with search, sorting, and pagination",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "List or paginated list of TV shows"},
					},
				},
				"post": map[string]interface{}{
					"summary":  "Create TV show catalog item",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"201": map[string]interface{}{"description": "TV show created"},
					},
				},
			},
			"/api/v1/books": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":  "List public book catalog items with search, sorting, and pagination",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "List or paginated list of books"},
					},
				},
				"post": map[string]interface{}{
					"summary":  "Create book catalog item",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"201": map[string]interface{}{"description": "Book created"},
					},
				},
			},
			"/api/v1/stats": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":  "Get dashboard statistics for the current user's lists",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Dashboard statistics"},
					},
				},
			},
			"/api/v1/search": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":  "Public global cross-category catalog search",
					"parameters": []map[string]interface{}{
						{"name": "q", "in": "query", "required": true, "schema": map[string]string{"type": "string"}, "description": "Search keyword"},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Search results across categories"},
					},
				},
			},
			"/api/v1/export": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":  "Export shared catalog items and the current user's lists",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Shared category data backup"},
					},
				},
			},
			"/api/v1/import": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":  "Import catalog items and add them to the current user's lists",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Data imported successfully"},
					},
				},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(spec)
}

func healthProbePath(description string) map[string]interface{} {
	return map[string]interface{}{
		"get": map[string]interface{}{
			"summary":     "Health check",
			"description": description,
			"responses": map[string]interface{}{
				"200": map[string]interface{}{"description": "Server and Postgres are healthy"},
				"503": map[string]interface{}{"description": "Postgres is unreachable"},
			},
		},
	}
}

func liveProbePath(description string) map[string]interface{} {
	return map[string]interface{}{
		"get": map[string]interface{}{
			"summary":     "Liveness probe",
			"description": description,
			"responses": map[string]interface{}{
				"200": map[string]interface{}{"description": "Process is alive"},
			},
		},
	}
}

func (d *DocsHandler) GetDocsUI(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>API Documentation</title>
  <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <style>
    html { box-sizing: border-box; overflow: -moz-scrollbars-vertical; overflow-y: scroll; }
    *, *:before, *:after { box-sizing: inherit; }
    body { margin: 0; background: #fafafa; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js" charset="UTF-8"> </script>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js" charset="UTF-8"> </script>
  <script>
    window.onload = function() {
      const ui = SwaggerUIBundle({
        url: "/api/v1/openapi.json",
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset
        ],
        plugins: [
          SwaggerUIBundle.plugins.DownloadUrl
        ],
        layout: "StandaloneLayout"
      });
      window.ui = ui;
    };
  </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
}
