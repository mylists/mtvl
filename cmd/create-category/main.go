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
	re := regexp.MustCompile(`^(\d+)_`)
	maxSeq := 0

	scan := func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
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
	}

	scan(migrationsDir)
	scan(filepath.Join(migrationsDir, "postgres"))
	scan(filepath.Join(migrationsDir, "mysql"))

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

func getSingularName(name string) string {
	if strings.HasSuffix(name, "ies") && len(name) > 3 {
		return name[:len(name)-3] + "y"
	}
	if strings.HasSuffix(name, "s") && len(name) > 1 {
		return name[:len(name)-1]
	}
	return name
}

func generateMigrationSQL(name, dialect string) string {
	var itemPK string
	var itemFK string
	var userIDType string
	var notesType string

	switch dialect {
	case "mysql":
		itemPK = "CHAR(36) PRIMARY KEY"
		itemFK = "CHAR(36) NOT NULL"
		userIDType = "INT NOT NULL"
		notesType = "TEXT"
	default: // postgres
		itemPK = "UUID PRIMARY KEY"
		itemFK = "UUID NOT NULL"
		userIDType = "INTEGER NOT NULL"
		notesType = "TEXT DEFAULT ''"
	}

	tmpl := `-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS {{NAME}} (
    id {{ITEM_PK}},
    title VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS user_{{NAME}} (
    user_id {{USER_ID_TYPE}},
    {{ITEM_COL}} {{ITEM_FK}},
    status VARCHAR(50) DEFAULT 'plan_to_watch',
    rating INTEGER DEFAULT 0,
    notes {{NOTES_TYPE}},
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, {{ITEM_COL}}),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY ({{ITEM_COL}}) REFERENCES {{NAME}}(id) ON DELETE CASCADE
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_{{NAME}};
DROP TABLE IF EXISTS {{NAME}};
-- +goose StatementEnd
`
	r := strings.NewReplacer(
		"{{NAME}}", name,
		"{{ITEM_PK}}", itemPK,
		"{{ITEM_FK}}", itemFK,
		"{{ITEM_COL}}", getSingularName(name)+"_id",
		"{{USER_ID_TYPE}}", userIDType,
		"{{NOTES_TYPE}}", notesType,
	)
	return r.Replace(tmpl)
}

func generateModelGo(name string) string {
	structName := getStructName(name)
	itemCol := getSingularName(name) + "_id"
	tmpl := `package {{NAME}}

import (
	"time"

	"gorm.io/gorm"
	"mtvl/internal/idgen"
)

// {{STRUCT}} is a shared catalog item.
type {{STRUCT}} struct {
	ID        string    ` + "`json:\"id\" gorm:\"primaryKey;size:36;column:id\"`" + `
	Title     string    ` + "`json:\"title\" gorm:\"column:title;not null\"`" + `
	CreatedAt time.Time ` + "`json:\"created_at\" gorm:\"column:created_at\"`" + `
	UpdatedAt time.Time ` + "`json:\"updated_at\" gorm:\"column:updated_at\"`" + `
}

func ({{STRUCT}}) TableName() string {
	return "{{NAME}}"
}

func (m *{{STRUCT}}) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = idgen.New()
	}
	return nil
}

// User{{STRUCT}} links a user to a catalog item on their list.
type User{{STRUCT}} struct {
	UserID    int64     ` + "`json:\"user_id\" gorm:\"primaryKey;column:user_id\"`" + `
	ItemID    string    ` + "`json:\"{{ITEM_COL}}\" gorm:\"primaryKey;size:36;column:{{ITEM_COL}}\"`" + `
	Status    string    ` + "`json:\"status\" gorm:\"column:status;not null;default:'plan_to_watch'\"`" + `
	Rating    int       ` + "`json:\"rating\" gorm:\"column:rating;default:0\"`" + `
	Notes     string    ` + "`json:\"notes\" gorm:\"column:notes\"`" + `
	CreatedAt time.Time ` + "`json:\"created_at\" gorm:\"column:created_at\"`" + `
	UpdatedAt time.Time ` + "`json:\"updated_at\" gorm:\"column:updated_at\"`" + `
}

func (User{{STRUCT}}) TableName() string {
	return "user_{{NAME}}"
}

// {{STRUCT}}ListItem is a catalog item joined with the current user's list fields.
type {{STRUCT}}ListItem struct {
	ID        string    ` + "`json:\"id\" gorm:\"column:id\"`" + `
	Title     string    ` + "`json:\"title\" gorm:\"column:title\"`" + `
	Status    string    ` + "`json:\"status\" gorm:\"column:status\"`" + `
	Rating    int       ` + "`json:\"rating\" gorm:\"column:rating\"`" + `
	Notes     string    ` + "`json:\"notes\" gorm:\"column:notes\"`" + `
	CreatedAt time.Time ` + "`json:\"created_at\" gorm:\"column:created_at\"`" + `
	UpdatedAt time.Time ` + "`json:\"updated_at\" gorm:\"column:updated_at\"`" + `
}
`
	r := strings.NewReplacer("{{NAME}}", name, "{{STRUCT}}", structName, "{{ITEM_COL}}", itemCol)
	return r.Replace(tmpl)
}

func generateHandlerGo(name, displayName, description, endpoint string) string {
	structName := getStructName(name)
	itemCol := getSingularName(name) + "_id"
	tmpl := `package {{NAME}}

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
	"mtvl/internal/auth"
	"mtvl/internal/core"
	"mtvl/internal/idgen"
)

type Module struct {
	db *gorm.DB
}

func NewModule(db *gorm.DB) *Module {
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

		sub.Get("/list", m.listUserItems)
		sub.Post("/list", m.addItemToList)
		sub.Post("/list/bulk-delete", m.bulkRemoveFromList)
		sub.Post("/list/bulk-status", m.bulkStatusItems)
		sub.Post("/bulk-status", m.bulkStatusItems)
		sub.Get("/list/{id}", m.getUserItem)
		sub.Put("/list/{id}", m.updateUserItem)
		sub.Delete("/list/{id}", m.removeItemFromList)

		sub.Get("/{id}", m.getItem)
		sub.Put("/{id}", m.updateItem)
		sub.Delete("/{id}", m.deleteItem)
	})
}

func (m *Module) listItems(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.GetUserFromContext(r.Context()); !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	qParam := strings.TrimSpace(r.URL.Query().Get("q"))
	sortByParam := strings.TrimSpace(r.URL.Query().Get("sort_by"))
	orderParam := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("order")))
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	query := m.db.WithContext(r.Context()).Model(&{{STRUCT}}{})

	if qParam != "" {
		pattern := "%" + strings.ToLower(qParam) + "%"
		query = query.Where("LOWER(title) LIKE ?", pattern)
	}

	validSortColumns := map[string]string{
		"id":         "id",
		"title":      "title",
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

	var total int64
	if isPaginated {
		if err := query.Count(&total).Error; err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	query = query.Order(sortCol + " " + strings.ToUpper(orderParam))

	if isPaginated {
		offset := (page - 1) * limit
		query = query.Offset(offset).Limit(limit)
	}

	items := make([]{{STRUCT}}, 0)
	if err := query.Find(&items).Error; err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if isPaginated {
		totalPages := (int(total) + limit - 1) / limit
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
	if _, ok := auth.GetUserFromContext(r.Context()); !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		IDs []string ` + "`json:\"ids\"`" + `
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		respondError(w, http.StatusBadRequest, "Invalid request body: ids array required")
		return
	}

	var deletedCount int64
	err := m.db.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("{{ITEM_COL}} IN ?", req.IDs).Delete(&User{{STRUCT}}{}).Error; err != nil {
			return err
		}
		res := tx.Where("id IN ?", req.IDs).Delete(&{{STRUCT}}{})
		deletedCount = res.RowsAffected
		return res.Error
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "Items deleted successfully",
		"deleted_count": deletedCount,
	})
}

func (m *Module) createItem(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.GetUserFromContext(r.Context()); !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		Title string ` + "`json:\"title\"`" + `
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

	now := time.Now()
	item := {{STRUCT}}{
		Title:     req.Title,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := m.db.WithContext(r.Context()).Create(&item).Error; err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, item)
}

func (m *Module) getItem(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.GetUserFromContext(r.Context()); !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	id, ok := idgen.Parse(chi.URLParam(r, "id"))
	if !ok {
		respondError(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	var item {{STRUCT}}
	err := m.db.WithContext(r.Context()).Where("id = ?", id).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respondError(w, http.StatusNotFound, "Item not found")
		return
	} else if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, item)
}

func (m *Module) updateItem(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.GetUserFromContext(r.Context()); !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	id, ok := idgen.Parse(chi.URLParam(r, "id"))
	if !ok {
		respondError(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	var req struct {
		Title string ` + "`json:\"title\"`" + `
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	now := time.Now()
	res := m.db.WithContext(r.Context()).Model(&{{STRUCT}}{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"title":      req.Title,
			"updated_at": now,
		})

	if res.Error != nil {
		respondError(w, http.StatusInternalServerError, res.Error.Error())
		return
	}

	if res.RowsAffected == 0 {
		respondError(w, http.StatusNotFound, "Item not found")
		return
	}

	m.getItem(w, r)
}

func (m *Module) deleteItem(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.GetUserFromContext(r.Context()); !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	id, ok := idgen.Parse(chi.URLParam(r, "id"))
	if !ok {
		respondError(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	var deleted int64
	err := m.db.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("{{ITEM_COL}} = ?", id).Delete(&User{{STRUCT}}{}).Error; err != nil {
			return err
		}
		res := tx.Where("id = ?", id).Delete(&{{STRUCT}}{})
		deleted = res.RowsAffected
		return res.Error
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if deleted == 0 {
		respondError(w, http.StatusNotFound, "Item not found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Item deleted successfully"})
}

func (m *Module) userItemQuery(r *http.Request, userID int64) *gorm.DB {
	return m.db.WithContext(r.Context()).
		Table("user_{{NAME}}").
		Select("{{NAME}}.id AS id, {{NAME}}.title AS title, user_{{NAME}}.status AS status, user_{{NAME}}.rating AS rating, user_{{NAME}}.notes AS notes, user_{{NAME}}.created_at AS created_at, user_{{NAME}}.updated_at AS updated_at").
		Joins("JOIN {{NAME}} ON {{NAME}}.id = user_{{NAME}}.{{ITEM_COL}}").
		Where("user_{{NAME}}.user_id = ?", userID)
}

func (m *Module) listUserItems(w http.ResponseWriter, r *http.Request) {
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

	query := m.userItemQuery(r, user.ID)

	if statusFilter != "" {
		query = query.Where("user_{{NAME}}.status = ?", statusFilter)
	}

	if qParam != "" {
		pattern := "%" + strings.ToLower(qParam) + "%"
		query = query.Where("(LOWER({{NAME}}.title) LIKE ? OR LOWER(user_{{NAME}}.notes) LIKE ?)", pattern, pattern)
	}

	validSortColumns := map[string]string{
		"id":         "{{NAME}}.id",
		"title":      "{{NAME}}.title",
		"status":     "user_{{NAME}}.status",
		"rating":     "user_{{NAME}}.rating",
		"created_at": "user_{{NAME}}.created_at",
		"updated_at": "user_{{NAME}}.updated_at",
	}

	sortCol, valid := validSortColumns[sortByParam]
	if !valid {
		sortCol = "user_{{NAME}}.updated_at"
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

	var total int64
	if isPaginated {
		if err := query.Count(&total).Error; err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	query = query.Order(sortCol + " " + strings.ToUpper(orderParam))

	if isPaginated {
		offset := (page - 1) * limit
		query = query.Offset(offset).Limit(limit)
	}

	items := make([]{{STRUCT}}ListItem, 0)
	if err := query.Scan(&items).Error; err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if isPaginated {
		totalPages := (int(total) + limit - 1) / limit
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

func (m *Module) addItemToList(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		ID     string ` + "`json:\"id\"`" + `
		Status string ` + "`json:\"status\"`" + `
		Rating int    ` + "`json:\"rating\"`" + `
		Notes  string ` + "`json:\"notes\"`" + `
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	id, ok := idgen.Parse(req.ID)
	if !ok {
		respondError(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	var item {{STRUCT}}
	err := m.db.WithContext(r.Context()).Where("id = ?", id).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respondError(w, http.StatusNotFound, "Item not found")
		return
	} else if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if req.Status == "" {
		req.Status = "plan_to_watch"
	}

	now := time.Now()
	link := User{{STRUCT}}{
		UserID:    user.ID,
		ItemID:    id,
		Status:    req.Status,
		Rating:    req.Rating,
		Notes:     req.Notes,
		CreatedAt: now,
		UpdatedAt: now,
	}

	err = m.db.WithContext(r.Context()).Where("user_id = ? AND {{ITEM_COL}} = ?", user.ID, id).First(&User{{STRUCT}}{}).Error
	if err == nil {
		respondError(w, http.StatusConflict, "Item is already on your list")
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := m.db.WithContext(r.Context()).Create(&link).Error; err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	m.respondUserItem(w, r, user.ID, id, http.StatusCreated)
}

func (m *Module) getUserItem(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	id, ok := idgen.Parse(chi.URLParam(r, "id"))
	if !ok {
		respondError(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	m.respondUserItem(w, r, user.ID, id, http.StatusOK)
}

func (m *Module) updateUserItem(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	id, ok := idgen.Parse(chi.URLParam(r, "id"))
	if !ok {
		respondError(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	var req struct {
		Status string ` + "`json:\"status\"`" + `
		Rating int    ` + "`json:\"rating\"`" + `
		Notes  string ` + "`json:\"notes\"`" + `
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	now := time.Now()
	res := m.db.WithContext(r.Context()).Model(&User{{STRUCT}}{}).
		Where("user_id = ? AND {{ITEM_COL}} = ?", user.ID, id).
		Updates(map[string]interface{}{
			"status":     req.Status,
			"rating":     req.Rating,
			"notes":      req.Notes,
			"updated_at": now,
		})
	if res.Error != nil {
		respondError(w, http.StatusInternalServerError, res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		respondError(w, http.StatusNotFound, "Item is not on your list")
		return
	}

	m.respondUserItem(w, r, user.ID, id, http.StatusOK)
}

func (m *Module) removeItemFromList(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	id, ok := idgen.Parse(chi.URLParam(r, "id"))
	if !ok {
		respondError(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	res := m.db.WithContext(r.Context()).Where("user_id = ? AND {{ITEM_COL}} = ?", user.ID, id).Delete(&User{{STRUCT}}{})
	if res.Error != nil {
		respondError(w, http.StatusInternalServerError, res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		respondError(w, http.StatusNotFound, "Item is not on your list")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Item removed from your list"})
}

func (m *Module) bulkRemoveFromList(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		IDs []string ` + "`json:\"ids\"`" + `
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		respondError(w, http.StatusBadRequest, "Invalid request body: ids array required")
		return
	}

	res := m.db.WithContext(r.Context()).Where("user_id = ? AND {{ITEM_COL}} IN ?", user.ID, req.IDs).Delete(&User{{STRUCT}}{})
	if res.Error != nil {
		respondError(w, http.StatusInternalServerError, res.Error.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "Items removed from your list",
		"deleted_count": res.RowsAffected,
	})
}

func (m *Module) bulkStatusItems(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		IDs    []string ` + "`json:\"ids\"`" + `
		Status string  ` + "`json:\"status\"`" + `
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 || strings.TrimSpace(req.Status) == "" {
		respondError(w, http.StatusBadRequest, "Invalid request body: ids array and status required")
		return
	}

	now := time.Now()
	res := m.db.WithContext(r.Context()).Model(&User{{STRUCT}}{}).
		Where("user_id = ? AND {{ITEM_COL}} IN ?", user.ID, req.IDs).
		Updates(map[string]interface{}{
			"status":     req.Status,
			"updated_at": now,
		})

	if res.Error != nil {
		respondError(w, http.StatusInternalServerError, res.Error.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "Items status updated successfully",
		"updated_count": res.RowsAffected,
	})
}

func (m *Module) respondUserItem(w http.ResponseWriter, r *http.Request, userID int64, itemID string, status int) {
	var item {{STRUCT}}ListItem
	err := m.userItemQuery(r, userID).Where("user_{{NAME}}.{{ITEM_COL}} = ?", itemID).Scan(&item).Error
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if item.ID == "" {
		respondError(w, http.StatusNotFound, "Item is not on your list")
		return
	}
	respondJSON(w, status, item)
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
		"{{ITEM_COL}}", itemCol,
	)
	return r.Replace(tmpl)
}

func generateHandlerTestGo(name, endpoint string) string {
	structName := getStructName(name)
	tmpl := `package {{NAME}}

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"mtvl/internal/auth"
)

func setupTestDB(t *testing.T) (*gorm.DB, *auth.User) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}

	if err := db.AutoMigrate(&auth.UserModel{}, &{{STRUCT}}{}, &User{{STRUCT}}{}); err != nil {
		t.Fatalf("failed to migrate tables: %v", err)
	}

	user := &auth.User{ID: 1, Username: "testuser", Email: "test@example.com"}
	return db, user
}

func TestModuleCRUD(t *testing.T) {
	db, user := setupTestDB(t)

	mod := NewModule(db)
	router := chi.NewRouter()

	authMw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithUserContext(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	mod.RegisterRoutes(router, authMw)

	// 1. Create catalog item
	body := []byte(` + "`{\"title\":\"Sample Item\"}`" + `)
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
	if created.ID == "" || created.Title != "Sample Item" {
		t.Errorf("unexpected item: %+v", created)
	}

	// 2. List catalog
	req = httptest.NewRequest("GET", "{{ENDPOINT}}", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	// 3. Add to user list
	addBody, _ := json.Marshal(map[string]string{"id": created.ID, "status": "completed"})
	req = httptest.NewRequest("POST", "{{ENDPOINT}}/list", bytes.NewBuffer(addBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for list add, got %d. Body: %s", rr.Code, rr.Body.String())
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
