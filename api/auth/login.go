package handler

import (
	"encoding/json"
	"net/http"
	"time"

	mid "github.com/gophish/gophish-vercel/lib/middleware"
	"github.com/gophish/gophish-vercel/lib/models"
	"github.com/google/uuid"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		mid.ErrorJSON(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		mid.ErrorJSON(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	db := models.GetDB()
	var user models.User
	if err := db.Where("username = ?", req.Username).First(&user).Error; err != nil {
		mid.ErrorJSON(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}
	if !user.CheckPassword(req.Password) {
		mid.ErrorJSON(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	// Generate API key if not set
	if user.ApiKey == "" {
		user.ApiKey = uuid.New().String()
		db.Save(&user)
	}

	token, err := mid.GenerateToken(user.Id, user.ApiKey)
	if err != nil {
		mid.ErrorJSON(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	// Set httpOnly cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "gophish_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(24 * time.Hour),
	})

	mid.JSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
		"user": map[string]interface{}{
			"id":       user.Id,
			"username": user.Username,
			"api_key":  user.ApiKey,
		},
	})
}
