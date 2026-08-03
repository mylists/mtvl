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
			"title":       "mtvl API",
			"description": "Comprehensive RESTful API for Media Tracking Vector List backend",
			"version":     "1.0.0",
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
			"/health": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "Health check endpoint",
					"description": "Returns server operational status",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Server is healthy",
						},
					},
				},
			},
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
					"summary":  "Delete user account and purge all data",
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
			"/api/v1/movies": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":  "List movies with search, status filtering, sorting, and pagination",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"parameters": []map[string]interface{}{
						{"name": "q", "in": "query", "schema": map[string]string{"type": "string"}, "description": "Search term"},
						{"name": "status", "in": "query", "schema": map[string]string{"type": "string"}, "description": "Filter status"},
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
					"summary":  "Create movie",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"201": map[string]interface{}{"description": "Movie created"},
					},
				},
			},
			"/api/v1/movies/bulk-delete": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":  "Bulk delete movies",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Movies deleted"},
					},
				},
			},
			"/api/v1/movies/bulk-status": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":  "Bulk update movies status",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Movies status updated"},
					},
				},
			},
			"/api/v1/tvshows": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":  "List TV shows with search, filtering, sorting, and pagination",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "List or paginated list of TV shows"},
					},
				},
				"post": map[string]interface{}{
					"summary":  "Create TV show",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"201": map[string]interface{}{"description": "TV show created"},
					},
				},
			},
			"/api/v1/books": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":  "List books with search, filtering, sorting, and pagination",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "List or paginated list of books"},
					},
				},
				"post": map[string]interface{}{
					"summary":  "Create book",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"201": map[string]interface{}{"description": "Book created"},
					},
				},
			},
			"/api/v1/stats": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":  "Get cross-category dashboard statistics",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Dashboard statistics"},
					},
				},
			},
			"/api/v1/search": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":  "Global cross-category search",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
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
					"summary":  "Export full user tracking data as JSON",
					"security": []map[string]interface{}{{"bearerAuth": []string{}}},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "User tracking data backup"},
					},
				},
			},
			"/api/v1/import": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":  "Import tracking data into user account",
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

func (d *DocsHandler) GetDocsUI(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>mtvl API Documentation</title>
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
