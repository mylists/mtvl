package books

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
		countQuery := "SELECT COUNT(*) FROM books WHERE " + whereStmt
		_ = m.db.QueryRowContext(r.Context(), countQuery, args...).Scan(&total)
	}

	query := "SELECT id, user_id, title, status, rating, notes, created_at, updated_at FROM books WHERE " + whereStmt + " ORDER BY " + sortCol + " " + strings.ToUpper(orderParam)

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

	items := make([]Book, 0)
	for rows.Next() {
		var item Book
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
		IDs []int64 `json:"ids"`
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

	query := "DELETE FROM books WHERE user_id = ? AND id IN (" + strings.Join(placeholders, ",") + ")"
	res, err := m.db.ExecContext(r.Context(), query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	rowsAffected, _ := res.RowsAffected()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "Books deleted successfully",
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
		IDs    []int64 `json:"ids"`
		Status string  `json:"status"`
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

	query := "UPDATE books SET status = ?, updated_at = ? WHERE user_id = ? AND id IN (" + strings.Join(placeholders, ",") + ")"
	res, err := m.db.ExecContext(r.Context(), query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	rowsAffected, _ := res.RowsAffected()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "Books status updated successfully",
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
		Title  string `json:"title"`
		Status string `json:"status"`
		Rating int    `json:"rating"`
		Notes  string `json:"notes"`
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
	query := `INSERT INTO books (user_id, title, status, rating, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
	res, err := m.db.ExecContext(r.Context(), query, user.ID, req.Title, req.Status, req.Rating, req.Notes, now, now)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	id, _ := res.LastInsertId()
	item := Book{
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

	var item Book
	query := `SELECT id, user_id, title, status, rating, notes, created_at, updated_at FROM books WHERE id = ? AND user_id = ?`
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
		Title  string `json:"title"`
		Status string `json:"status"`
		Rating int    `json:"rating"`
		Notes  string `json:"notes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	now := time.Now()
	query := `UPDATE books SET title = ?, status = ?, rating = ?, notes = ?, updated_at = ? WHERE id = ? AND user_id = ?`
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

	query := `DELETE FROM books WHERE id = ? AND user_id = ?`
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
