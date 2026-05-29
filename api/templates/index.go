package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	mid "github.com/gophish/gophish-vercel/lib/middleware"
	"github.com/gophish/gophish-vercel/lib/models"
	"gorm.io/gorm"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	claims, err := mid.ValidateRequest(r)
	if err != nil {
		mid.ErrorJSON(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	db := models.GetDB()
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")

	if len(parts) == 3 {
		id, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			mid.ErrorJSON(w, http.StatusBadRequest, "Invalid id")
			return
		}
		handleSingle(w, r, db, claims.UserId, id)
		return
	}

	switch r.Method {
	case http.MethodGet:
		var ts []models.Template
		db.Where("user_id = ?", claims.UserId).Find(&ts)
		mid.JSON(w, http.StatusOK, ts)
	case http.MethodPost:
		handleCreate(w, r, db, claims.UserId)
	default:
		mid.ErrorJSON(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func handleCreate(w http.ResponseWriter, r *http.Request, db *gorm.DB, uid int64) {
	var t models.Template
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		mid.ErrorJSON(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if t.Name == "" {
		mid.ErrorJSON(w, http.StatusBadRequest, "Template name required")
		return
	}
	t.UserId = uid
	t.ModifiedDate = time.Now().UTC()
	if err := db.Create(&t).Error; err != nil {
		mid.ErrorJSON(w, http.StatusInternalServerError, "Failed to create template")
		return
	}
	mid.JSON(w, http.StatusCreated, t)
}

func handleSingle(w http.ResponseWriter, r *http.Request, db *gorm.DB, uid, id int64) {
	var t models.Template
	if err := db.Where("id = ? AND user_id = ?", id, uid).First(&t).Error; err != nil {
		mid.ErrorJSON(w, http.StatusNotFound, "Template not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		mid.JSON(w, http.StatusOK, t)
	case http.MethodPut:
		var updated models.Template
		if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
			mid.ErrorJSON(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		t.Name = updated.Name
		t.Subject = updated.Subject
		t.Text = updated.Text
		t.HTML = updated.HTML
		t.ModifiedDate = time.Now().UTC()
		db.Save(&t)
		mid.JSON(w, http.StatusOK, t)
	case http.MethodDelete:
		db.Delete(&t)
		mid.JSON(w, http.StatusOK, map[string]string{"message": "Template deleted"})
	default:
		mid.ErrorJSON(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}
