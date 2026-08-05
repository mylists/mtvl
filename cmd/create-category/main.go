package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func main() {
	nameFlag := flag.String("name", "", "Category identifier in snake_case (e.g. books, anime, video_games)")
	displayFlag := flag.String("display", "", "Human-readable display name (e.g. 'Books', 'Video Games')")
	descFlag := flag.String("description", "", "Category description")
	endpointFlag := flag.String("endpoint", "", "API endpoint path (e.g. /api/v1/books)")

	flag.Parse()

	name := strings.TrimSpace(strings.ToLower(*nameFlag))
	if name == "" && len(flag.Args()) > 0 {
		name = strings.TrimSpace(strings.ToLower(flag.Arg(0)))
	}

	if name == "" {
		fmt.Println("Error: Category name is required.")
		fmt.Println("Usage: go run cmd/create-category/main.go -name books -display 'Books' -description 'Track books read'")
		os.Exit(1)
	}

	displayName := *displayFlag
	if displayName == "" {
		parts := strings.Split(name, "_")
		for i, p := range parts {
			if len(p) > 0 {
				parts[i] = strings.ToUpper(p[:1]) + p[1:]
			}
		}
		displayName = strings.Join(parts, " ")
	}

	description := *descFlag
	if description == "" {
		description = fmt.Sprintf("Track and manage %s list", displayName)
	}

	endpoint := *endpointFlag
	if endpoint == "" {
		cleanName := strings.ReplaceAll(name, "_", "")
		endpoint = "/api/v1/" + cleanName
	}

	projectRoot := findProjectRoot()
	migrationsDir := filepath.Join(projectRoot, "migrations")
	modulesDir := filepath.Join(projectRoot, "internal", "modules")

	// 1. Generate Goose Migration
	nextSeq := getNextMigrationSeq(migrationsDir)
	migrationFilename := fmt.Sprintf("%05d_%s.sql", nextSeq, name)

	dialects := map[string]string{
		"postgres": generateMigrationSQL(name, "postgres"),
		"mysql":    generateMigrationSQL(name, "mysql"),
	}

	for dialect, content := range dialects {
		dDir := filepath.Join(migrationsDir, dialect)
		_ = os.MkdirAll(dDir, 0755)
		if err := writeFile(filepath.Join(dDir, migrationFilename), content); err != nil {
			fmt.Printf("Error creating %s migration file: %v\n", dialect, err)
			os.Exit(1)
		}
	}
	_ = writeFile(filepath.Join(migrationsDir, migrationFilename), dialects["postgres"])
	fmt.Printf(" Created Migrations: migrations/%s (postgres, mysql)\n", migrationFilename)

	// 2. Create Module Directory
	targetModuleDir := filepath.Join(modulesDir, name)
	if err := os.MkdirAll(targetModuleDir, 0755); err != nil {
		fmt.Printf("Error creating module dir: %v\n", err)
		os.Exit(1)
	}

	// 3. Generate model.go
	modelPath := filepath.Join(targetModuleDir, "model.go")
	modelContent := generateModelGo(name)
	if err := writeFile(modelPath, modelContent); err != nil {
		fmt.Printf("Error creating model.go: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf(" Created Model: internal/modules/%s/model.go\n", name)

	// 4. Generate handler.go
	handlerPath := filepath.Join(targetModuleDir, "handler.go")
	handlerContent := generateHandlerGo(name, displayName, description, endpoint)
	if err := writeFile(handlerPath, handlerContent); err != nil {
		fmt.Printf("Error creating handler.go: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf(" Created Handler: internal/modules/%s/handler.go\n", name)

	// 5. Generate handler_test.go
	testPath := filepath.Join(targetModuleDir, "handler_test.go")
	testContent := generateHandlerTestGo(name, endpoint)
	if err := writeFile(testPath, testContent); err != nil {
		fmt.Printf("Error creating handler_test.go: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf(" Created Tests: internal/modules/%s/handler_test.go\n", name)

	fmt.Println("\nSuccessfully bootstrapped category module!")
	fmt.Println("Final Step: Register your new module in main.go:")
	fmt.Printf("    registry.Register(%s.NewModule(database))\n", name)
}

func findProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "."
}

func getNextMigrationSeq(migrationsDir string) int {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return 1
	}

	re := regexp.MustCompile(`^(\d+)_`)
	maxSeq := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := re.FindStringSubmatch(entry.Name())
		if len(matches) == 2 {
			seq, _ := strconv.Atoi(matches[1])
			if seq > maxSeq {
				maxSeq = seq
			}
		}
	}

	return maxSeq + 1
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

func toCamelCase(s string) string {
	parts := strings.Split(s, "_")
	var result []string
	for _, p := range parts {
		if len(p) > 0 {
			result = append(result, strings.ToUpper(p[:1])+p[1:])
		}
	}
	return strings.Join(result, "")
}

func getStructName(name string) string {
	structName := toCamelCase(name)
	if strings.HasSuffix(structName, "s") {
		structName = structName[:len(structName)-1]
	}
	return structName
}

func generateMigrationSQL(name, dialect string) string {
	var primaryKey string
	var notesType string

	switch dialect {
	case "mysql":
		primaryKey = "INT AUTO_INCREMENT PRIMARY KEY"
		notesType = "TEXT"
	default: // postgres
		primaryKey = "SERIAL PRIMARY KEY"
		notesType = "TEXT DEFAULT ''"
	}

	tmpl := `-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS {{NAME}} (
    id {{PRIMARY_KEY}},
    user_id INTEGER NOT NULL,
    title VARCHAR(255) NOT NULL,
    status VARCHAR(50) DEFAULT 'plan_to_watch',
    rating INTEGER DEFAULT 0,
    notes {{NOTES_TYPE}},
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS {{NAME}};
-- +goose StatementEnd
`
	r := strings.NewReplacer(
		"{{NAME}}", name,
		"{{PRIMARY_KEY}}", primaryKey,
		"{{NOTES_TYPE}}", notesType,
	)
	return r.Replace(tmpl)
}

func generateModelGo(name string) string {
	structName := getStructName(name)
	tmpl := `package {{NAME}}

import "time"

// {{STRUCT}} represents a tracking entry for {{NAME}}.
type {{STRUCT}} struct {
	ID        int64     ` + "`json:\"id\"`" + `
	UserID    int64     ` + "`json:\"user_id\"`" + `
	Title     string    ` + "`json:\"title\"`" + `
	Status    string    ` + "`json:\"status\"`" + `
	Rating    int       ` + "`json:\"rating\"`" + `
	Notes     string    ` + "`json:\"notes\"`" + `
	CreatedAt time.Time ` + "`json:\"created_at\"`" + `
	UpdatedAt time.Time ` + "`json:\"updated_at\"`" + `
}
`
	r := strings.NewReplacer("{{NAME}}", name, "{{STRUCT}}", structName)
	return r.Replace(tmpl)
}

func generateHandlerGo(name, displayName, description, endpoint string) string {
	structName := getStructName(name)
	tmpl := `package {{NAME}}

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"mtvl/internal/auth"
	"mtvl/internal/core"
)

type Module struct {
	db *sql.DB
}

func NewModule(db *sql.DB) *Module {
	return &Module{db: db}
}

func (m *Module) Info() core.CategoryInfo {
	return core.CategoryInfo{
		Category:    "{{NAME}}",
		DisplayName: "{{DISPLAY}}",
		Description: "{{DESC}}",
		Endpoint:    "{{ENDPOINT}}",
	}
}

func (m *Module) RegisterRoutes(r chi.Router, authMw func(http.Handler) http.Handler) {
	r.Route("{{ENDPOINT}}", func(sub chi.Router) {
		sub.Use(authMw)

		sub.Get("/", m.listItems)
		sub.Post("/", m.createItem)
		sub.Post("/bulk-delete", m.bulkDeleteItems)
		sub.Post("/bulk-status", m.bulkStatusItems)
		sub.Get("/{id}", m.getItem)
		sub.Put("/{id}", m.updateItem)
		sub.Delete("/{id}", m.deleteItem)
	})
}

func (m *Module) listItems(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	qParam := strings.TrimSpace(r.URL.Query().Get("q"))
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	sortByParam := strings.TrimSpace(r.URL.Query().Get("sort_by"))
	orderParam := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("order")))
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	whereClauses := []string{"user_id = ?"}
	args := []interface{}{user.ID}

	if statusFilter != "" {
		whereClauses = append(whereClauses, "status = ?")
		args = append(args, statusFilter)
	}

	if qParam != "" {
		whereClauses = append(whereClauses, "(LOWER(title) LIKE ? OR LOWER(notes) LIKE ?)")
		pattern := "%" + strings.ToLower(qParam) + "%"
		args = append(args, pattern, pattern)
	}

	whereStmt := strings.Join(whereClauses, " AND ")

	validSortColumns := map[string]string{
		"id":         "id",
		"title":      "title",
		"status":     "status",
		"rating":     "rating",
		"created_at": "created_at",
		"updated_at": "updated_at",
	}

	sortCol, valid := validSortColumns[sortByParam]
	if !valid {
		sortCol = "updated_at"
	}

	if orderParam != "asc" && orderParam != "desc" {
		orderParam = "desc"
	}

	isPaginated := pageStr != "" || limitStr != ""
	page := 1
	limit := 50

	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
			if limit > 100 {
				limit = 100
			}
		}
	}

	var total int
	if isPaginated {
		countQuery := "SELECT COUNT(*) FROM {{NAME}} WHERE " + whereStmt
		_ = m.db.QueryRowContext(r.Context(), countQuery, args...).Scan(&total)
	}

	query := "SELECT id, user_id, title, status, rating, notes, created_at, updated_at FROM {{NAME}} WHERE " + whereStmt + " ORDER BY " + sortCol + " " + strings.ToUpper(orderParam)

	if isPaginated {
		offset := (page - 1) * limit
		query += " LIMIT " + strconv.Itoa(limit) + " OFFSET " + strconv.Itoa(offset)
	}

	rows, err := m.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	items := make([]{{STRUCT}}, 0)
	for rows.Next() {
		var item {{STRUCT}}
		if err := rows.Scan(&item.ID, &item.UserID, &item.Title, &item.Status, &item.Rating, &item.Notes, &item.CreatedAt, &item.UpdatedAt); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		items = append(items, item)
	}

	if isPaginated {
		totalPages := (total + limit - 1) / limit
		if totalPages < 0 {
			totalPages = 0
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"data": items,
			"pagination": map[string]interface{}{
				"total":       total,
				"page":        page,
				"limit":       limit,
				"total_pages": totalPages,
			},
		})
		return
	}

	respondJSON(w, http.StatusOK, items)
}

func (m *Module) bulkDeleteItems(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		IDs []int64 ` + "`json:\"ids\"`" + `
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		respondError(w, http.StatusBadRequest, "Invalid request body: ids array required")
		return
	}

	placeholders := make([]string, len(req.IDs))
	args := make([]interface{}, 0, len(req.IDs)+1)
	args = append(args, user.ID)
	for i, id := range req.IDs {
		placeholders[i] = "?"
		args = append(args, id)
	}

	query := "DELETE FROM {{NAME}} WHERE user_id = ? AND id IN (" + strings.Join(placeholders, ",") + ")"
	res, err := m.db.ExecContext(r.Context(), query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	rowsAffected, _ := res.RowsAffected()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "Items deleted successfully",
		"deleted_count": rowsAffected,
	})
}

func (m *Module) bulkStatusItems(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		IDs    []int64 ` + "`json:\"ids\"`" + `
		Status string  ` + "`json:\"status\"`" + `
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 || strings.TrimSpace(req.Status) == "" {
		respondError(w, http.StatusBadRequest, "Invalid request body: ids array and status required")
		return
	}

	placeholders := make([]string, len(req.IDs))
	now := time.Now()
	args := make([]interface{}, 0, len(req.IDs)+3)
	args = append(args, req.Status, now, user.ID)
	for i, id := range req.IDs {
		placeholders[i] = "?"
		args = append(args, id)
	}

	query := "UPDATE {{NAME}} SET status = ?, updated_at = ? WHERE user_id = ? AND id IN (" + strings.Join(placeholders, ",") + ")"
	res, err := m.db.ExecContext(r.Context(), query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	rowsAffected, _ := res.RowsAffected()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "Items status updated successfully",
		"updated_count": rowsAffected,
	})
}

func (m *Module) createItem(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		Title  string ` + "`json:\"title\"`" + `
		Status string ` + "`json:\"status\"`" + `
		Rating int    ` + "`json:\"rating\"`" + `
		Notes  string ` + "`json:\"notes\"`" + `
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		respondError(w, http.StatusBadRequest, "Title is required")
		return
	}

	if req.Status == "" {
		req.Status = "plan_to_watch"
	}

	now := time.Now()
	query := ` + "`INSERT INTO {{NAME}} (user_id, title, status, rating, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`" + `
	res, err := m.db.ExecContext(r.Context(), query, user.ID, req.Title, req.Status, req.Rating, req.Notes, now, now)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	id, _ := res.LastInsertId()
	item := {{STRUCT}}{
		ID:        id,
		UserID:    user.ID,
		Title:     req.Title,
		Status:    req.Status,
		Rating:    req.Rating,
		Notes:     req.Notes,
		CreatedAt: now,
		UpdatedAt: now,
	}

	respondJSON(w, http.StatusCreated, item)
}

func (m *Module) getItem(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	var item {{STRUCT}}
	query := ` + "`SELECT id, user_id, title, status, rating, notes, created_at, updated_at FROM {{NAME}} WHERE id = ? AND user_id = ?`" + `
	err = m.db.QueryRowContext(r.Context(), query, id, user.ID).Scan(&item.ID, &item.UserID, &item.Title, &item.Status, &item.Rating, &item.Notes, &item.CreatedAt, &item.UpdatedAt)
	if err == sql.ErrNoRows {
		respondError(w, http.StatusNotFound, "Item not found")
		return
	} else if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, item)
}

func (m *Module) updateItem(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	var req struct {
		Title  string ` + "`json:\"title\"`" + `
		Status string ` + "`json:\"status\"`" + `
		Rating int    ` + "`json:\"rating\"`" + `
		Notes  string ` + "`json:\"notes\"`" + `
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	now := time.Now()
	query := ` + "`UPDATE {{NAME}} SET title = ?, status = ?, rating = ?, notes = ?, updated_at = ? WHERE id = ? AND user_id = ?`" + `
	res, err := m.db.ExecContext(r.Context(), query, req.Title, req.Status, req.Rating, req.Notes, now, id, user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		respondError(w, http.StatusNotFound, "Item not found")
		return
	}

	m.getItem(w, r)
}

func (m *Module) deleteItem(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	query := ` + "`DELETE FROM {{NAME}} WHERE id = ? AND user_id = ?`" + `
	res, err := m.db.ExecContext(r.Context(), query, id, user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		respondError(w, http.StatusNotFound, "Item not found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Item deleted successfully"})
}

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
`
	r := strings.NewReplacer(
		"{{NAME}}", name,
		"{{STRUCT}}", structName,
		"{{DISPLAY}}", displayName,
		"{{DESC}}", description,
		"{{ENDPOINT}}", endpoint,
	)
	return r.Replace(tmpl)
}

func generateHandlerTestGo(name, endpoint string) string {
	structName := getStructName(name)
	tmpl := `package {{NAME}}

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	_ "modernc.org/sqlite"
	"mtvl/internal/auth"
)

func setupTestDB(t *testing.T) (*sql.DB, *auth.User) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}

	schemas := []string{
		` + "`CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, username VARCHAR(100) NOT NULL, email VARCHAR(255) NOT NULL, password_hash VARCHAR(255) NOT NULL, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`" + `,
		` + "`CREATE TABLE {{NAME}} (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, title VARCHAR(255) NOT NULL, status VARCHAR(50) DEFAULT 'plan_to_watch', rating INTEGER DEFAULT 0, notes TEXT DEFAULT '', created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`" + `,
	}

	for _, s := range schemas {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("failed to create table: %v", err)
		}
	}

	user := &auth.User{ID: 1, Username: "testuser", Email: "test@example.com"}
	return db, user
}

func TestModuleCRUD(t *testing.T) {
	db, user := setupTestDB(t)
	defer db.Close()

	mod := NewModule(db)
	router := chi.NewRouter()

	authMw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithUserContext(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	mod.RegisterRoutes(router, authMw)

	// 1. Create
	body := []byte(` + "`{\"title\":\"Sample Item\",\"status\":\"completed\",\"rating\":9}`" + `)
	req := httptest.NewRequest("POST", "{{ENDPOINT}}", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var created {{STRUCT}}
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created item: %v", err)
	}
	if created.ID == 0 || created.Title != "Sample Item" {
		t.Errorf("unexpected item: %+v", created)
	}

	// 2. List
	req = httptest.NewRequest("GET", "{{ENDPOINT}}", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}
}
`
	r := strings.NewReplacer(
		"{{NAME}}", name,
		"{{STRUCT}}", structName,
		"{{ENDPOINT}}", endpoint,
	)
	return r.Replace(tmpl)
}
