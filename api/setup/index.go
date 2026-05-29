// Package handler provides the one-time setup endpoint.
// POST /api/setup  {"username":"admin","password":"changeme"}
// Only works when no users exist in the database.
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
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		mid.ErrorJSON(w, http.StatusMethodNotAllowed, "POST only")
		return
	}

	db := models.GetDB()

	// Only allow if no users exist
	var count int64
	db.Model(&models.User{}).Count(&count)
	if count > 0 {
		mid.ErrorJSON(w, http.StatusForbidden, "Setup already completed")
		return
	}

	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Username == "" || body.Password == "" {
		mid.ErrorJSON(w, http.StatusBadRequest, "username and password required")
		return
	}

	user := models.User{
		Username:     body.Username,
		ApiKey:       uuid.New().String(),
		ModifiedDate: time.Now().UTC(),
	}
	if err := user.SetPassword(body.Password); err != nil {
		mid.ErrorJSON(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}
	if err := db.Create(&user).Error; err != nil {
		mid.ErrorJSON(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	mid.JSON(w, http.StatusCreated, map[string]interface{}{
		"message":  "Admin user created. You can now log in.",
		"username": user.Username,
		"api_key":  user.ApiKey,
	})
}
