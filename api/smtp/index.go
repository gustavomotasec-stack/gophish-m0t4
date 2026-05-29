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
		var s models.SMTP
		if err := db.Where("id = ? AND user_id = ?", id, claims.UserId).First(&s).Error; err != nil {
			mid.ErrorJSON(w, http.StatusNotFound, "Sending profile not found")
			return
		}
		switch r.Method {
		case http.MethodGet:
			db.Where("smtp_id = ?", s.Id).Find(&s.Headers)
			mid.JSON(w, http.StatusOK, s)
		case http.MethodPut:
			var updated models.SMTP
			json.NewDecoder(r.Body).Decode(&updated)
			s.Name = updated.Name
			s.Host = updated.Host
			s.Port = updated.Port
			s.Username = updated.Username
			if updated.Password != "" {
				s.Password = updated.Password
			}
			s.FromAddress = updated.FromAddress
			s.IgnoreCertErrors = updated.IgnoreCertErrors
			s.ModifiedDate = time.Now().UTC()
			db.Save(&s)
			// Replace headers
			db.Where("smtp_id = ?", s.Id).Delete(&models.Header{})
			for _, h := range updated.Headers {
				h.SMTPId = s.Id
				db.Create(&h)
			}
			db.Where("smtp_id = ?", s.Id).Find(&s.Headers)
			mid.JSON(w, http.StatusOK, s)
		case http.MethodDelete:
			db.Where("smtp_id = ?", id).Delete(&models.Header{})
			db.Delete(&s)
			mid.JSON(w, http.StatusOK, map[string]string{"message": "Sending profile deleted"})
		default:
			mid.ErrorJSON(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
		return
	}

	switch r.Method {
	case http.MethodGet:
		var profiles []models.SMTP
		db.Where("user_id = ?", claims.UserId).Find(&profiles)
		for i := range profiles {
			db.Where("smtp_id = ?", profiles[i].Id).Find(&profiles[i].Headers)
		}
		mid.JSON(w, http.StatusOK, profiles)
	case http.MethodPost:
		var s models.SMTP
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
			mid.ErrorJSON(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		if s.Name == "" {
			mid.ErrorJSON(w, http.StatusBadRequest, "Profile name required")
			return
		}
		s.UserId = claims.UserId
		s.ModifiedDate = time.Now().UTC()
		db.Create(&s)
		for _, h := range s.Headers {
			h.SMTPId = s.Id
			db.Create(&h)
		}
		mid.JSON(w, http.StatusCreated, s)
	default:
		mid.ErrorJSON(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}
