package books

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
		Category:    "books",
		DisplayName: "Books",
		Description: "Track books read",
		Endpoint:    "/api/v1/books",
	}
}

func (m *Module) RegisterRoutes(r chi.Router, authMw func(http.Handler) http.Handler) {
	r.Route("/api/v1/books", func(sub chi.Router) {
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

	query := m.db.WithContext(r.Context()).Model(&Book{})

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

	items := make([]Book, 0)
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
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		respondError(w, http.StatusBadRequest, "Invalid request body: ids array required")
		return
	}

	var deletedCount int64
	err := m.db.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("book_id IN ?", req.IDs).Delete(&UserBook{}).Error; err != nil {
			return err
		}
		res := tx.Where("id IN ?", req.IDs).Delete(&Book{})
		deletedCount = res.RowsAffected
		return res.Error
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "Books deleted successfully",
		"deleted_count": deletedCount,
	})
}

func (m *Module) createItem(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.GetUserFromContext(r.Context()); !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		Title string `json:"title"`
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
	item := Book{
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

	var item Book
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
		Title string `json:"title"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	now := time.Now()
	res := m.db.WithContext(r.Context()).Model(&Book{}).
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
		if err := tx.Where("book_id = ?", id).Delete(&UserBook{}).Error; err != nil {
			return err
		}
		res := tx.Where("id = ?", id).Delete(&Book{})
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

func (m *Module) userBookQuery(r *http.Request, userID int64) *gorm.DB {
	return m.db.WithContext(r.Context()).
		Table("user_books").
		Select("books.id AS id, books.title AS title, user_books.status AS status, user_books.rating AS rating, user_books.notes AS notes, user_books.created_at AS created_at, user_books.updated_at AS updated_at").
		Joins("JOIN books ON books.id = user_books.book_id").
		Where("user_books.user_id = ?", userID)
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

	query := m.userBookQuery(r, user.ID)

	if statusFilter != "" {
		query = query.Where("user_books.status = ?", statusFilter)
	}

	if qParam != "" {
		pattern := "%" + strings.ToLower(qParam) + "%"
		query = query.Where("(LOWER(books.title) LIKE ? OR LOWER(user_books.notes) LIKE ?)", pattern, pattern)
	}

	validSortColumns := map[string]string{
		"id":         "books.id",
		"title":      "books.title",
		"status":     "user_books.status",
		"rating":     "user_books.rating",
		"created_at": "user_books.created_at",
		"updated_at": "user_books.updated_at",
	}

	sortCol, valid := validSortColumns[sortByParam]
	if !valid {
		sortCol = "user_books.updated_at"
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

	items := make([]BookListItem, 0)
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
		ID     string `json:"id"`
		Status string `json:"status"`
		Rating int    `json:"rating"`
		Notes  string `json:"notes"`
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

	var item Book
	err := m.db.WithContext(r.Context()).Where("id = ?", id).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respondError(w, http.StatusNotFound, "Item not found")
		return
	} else if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if req.Status == "" {
		req.Status = "plan_to_read"
	}

	now := time.Now()
	link := UserBook{
		UserID:    user.ID,
		BookID:    id,
		Status:    req.Status,
		Rating:    req.Rating,
		Notes:     req.Notes,
		CreatedAt: now,
		UpdatedAt: now,
	}

	err = m.db.WithContext(r.Context()).Where("user_id = ? AND book_id = ?", user.ID, id).First(&UserBook{}).Error
	if err == nil {
		respondError(w, http.StatusConflict, "Book is already on your list")
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
		Status string `json:"status"`
		Rating int    `json:"rating"`
		Notes  string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	now := time.Now()
	res := m.db.WithContext(r.Context()).Model(&UserBook{}).
		Where("user_id = ? AND book_id = ?", user.ID, id).
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
		respondError(w, http.StatusNotFound, "Book is not on your list")
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

	res := m.db.WithContext(r.Context()).Where("user_id = ? AND book_id = ?", user.ID, id).Delete(&UserBook{})
	if res.Error != nil {
		respondError(w, http.StatusInternalServerError, res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		respondError(w, http.StatusNotFound, "Book is not on your list")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Book removed from your list"})
}

func (m *Module) bulkRemoveFromList(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		respondError(w, http.StatusBadRequest, "Invalid request body: ids array required")
		return
	}

	res := m.db.WithContext(r.Context()).Where("user_id = ? AND book_id IN ?", user.ID, req.IDs).Delete(&UserBook{})
	if res.Error != nil {
		respondError(w, http.StatusInternalServerError, res.Error.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "Books removed from your list",
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
		IDs    []string `json:"ids"`
		Status string   `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 || strings.TrimSpace(req.Status) == "" {
		respondError(w, http.StatusBadRequest, "Invalid request body: ids array and status required")
		return
	}

	now := time.Now()
	res := m.db.WithContext(r.Context()).Model(&UserBook{}).
		Where("user_id = ? AND book_id IN ?", user.ID, req.IDs).
		Updates(map[string]interface{}{
			"status":     req.Status,
			"updated_at": now,
		})

	if res.Error != nil {
		respondError(w, http.StatusInternalServerError, res.Error.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "Books status updated successfully",
		"updated_count": res.RowsAffected,
	})
}

func (m *Module) respondUserItem(w http.ResponseWriter, r *http.Request, userID int64, bookID string, status int) {
	var item BookListItem
	err := m.userBookQuery(r, userID).Where("user_books.book_id = ?", bookID).Scan(&item).Error
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if item.ID == "" {
		respondError(w, http.StatusNotFound, "Book is not on your list")
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
