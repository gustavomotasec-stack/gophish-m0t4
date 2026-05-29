package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	mid "github.com/gophish/gophish-vercel/lib/middleware"
	"github.com/gophish/gophish-vercel/lib/models"
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
		var p models.Page
		if err := db.Where("id = ? AND user_id = ?", id, claims.UserId).First(&p).Error; err != nil {
			mid.ErrorJSON(w, http.StatusNotFound, "Page not found")
			return
		}
		switch r.Method {
		case http.MethodGet:
			mid.JSON(w, http.StatusOK, p)
		case http.MethodPut:
			var updated models.Page
			json.NewDecoder(r.Body).Decode(&updated)
			p.Name = updated.Name
			p.HTML = updated.HTML
			p.CaptureCredentials = updated.CaptureCredentials
			p.CapturePasswords = updated.CapturePasswords
			p.RedirectURL = updated.RedirectURL
			p.ModifiedDate = time.Now().UTC()
			db.Save(&p)
			mid.JSON(w, http.StatusOK, p)
		case http.MethodDelete:
			db.Delete(&p)
			mid.JSON(w, http.StatusOK, map[string]string{"message": "Page deleted"})
		default:
			mid.ErrorJSON(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
		return
	}

	switch r.Method {
	case http.MethodGet:
		var pages []models.Page
		db.Where("user_id = ?", claims.UserId).Find(&pages)
		mid.JSON(w, http.StatusOK, pages)
	case http.MethodPost:
		var p models.Page
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			mid.ErrorJSON(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		if p.Name == "" {
			mid.ErrorJSON(w, http.StatusBadRequest, "Page name required")
			return
		}
		p.UserId = claims.UserId
		p.ModifiedDate = time.Now().UTC()
		db.Create(&p)
		mid.JSON(w, http.StatusCreated, p)
	default:
		mid.ErrorJSON(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}
