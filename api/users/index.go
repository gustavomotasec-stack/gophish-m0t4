package handler

import (
	"encoding/json"
	"net/http"
	"time"

	mid "github.com/gophish/gophish-vercel/lib/middleware"
	"github.com/gophish/gophish-vercel/lib/models"
	"github.com/google/uuid"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	claims, err := mid.ValidateRequest(r)
	if err != nil {
		mid.ErrorJSON(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	db := models.GetDB()
	var user models.User
	if err := db.First(&user, claims.UserId).Error; err != nil {
		mid.ErrorJSON(w, http.StatusNotFound, "User not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		mid.JSON(w, http.StatusOK, map[string]interface{}{
			"id":       user.Id,
			"username": user.Username,
			"api_key":  user.ApiKey,
		})

	case http.MethodPut:
		var body struct {
			CurrentPassword string `json:"current_password"`
			NewPassword     string `json:"new_password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			mid.ErrorJSON(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		if !user.CheckPassword(body.CurrentPassword) {
			mid.ErrorJSON(w, http.StatusBadRequest, "Invalid current password")
			return
		}
		if err := user.SetPassword(body.NewPassword); err != nil {
			mid.ErrorJSON(w, http.StatusInternalServerError, "Failed to set password")
			return
		}
		// Regenerate API key on password change
		user.ApiKey = uuid.New().String()
		user.ModifiedDate = time.Now().UTC()
		db.Save(&user)
		mid.JSON(w, http.StatusOK, map[string]string{"message": "Password updated"})

	default:
		mid.ErrorJSON(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}
