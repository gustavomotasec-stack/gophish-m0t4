package handler

import (
	"encoding/json"
	"io"
	"net/http"

	mid "github.com/gophish/gophish-vercel/lib/middleware"
	"github.com/gophish/gophish-vercel/lib/models"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	if _, err := mid.ValidateRequest(r); err != nil {
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
	_ = models.GetDB()

	var body struct {
		URL                     string `json:"url"`
		IncludeExternalResources bool  `json:"include_external_resources"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.URL == "" {
		mid.ErrorJSON(w, http.StatusBadRequest, "url required")
		return
	}

	resp, err := http.Get(body.URL)
	if err != nil {
		mid.ErrorJSON(w, http.StatusBadRequest, "Failed to fetch URL: "+err.Error())
		return
	}
	defer resp.Body.Close()
	htmlBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		mid.ErrorJSON(w, http.StatusInternalServerError, "Failed to read response")
		return
	}
	mid.JSON(w, http.StatusOK, map[string]string{"html": string(htmlBytes)})
}
