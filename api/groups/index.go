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

	// /api/groups/summary
	if len(parts) == 3 && parts[2] == "summary" {
		handleSummaryList(w, db, claims.UserId)
		return
	}
	// /api/groups/{id}
	if len(parts) == 3 {
		id, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			mid.ErrorJSON(w, http.StatusBadRequest, "Invalid group id")
			return
		}
		// /api/groups/{id}/summary
		if len(parts) == 4 && parts[3] == "summary" {
			handleSingleSummary(w, db, claims.UserId, id)
			return
		}
		handleSingle(w, r, db, claims.UserId, id)
		return
	}

	switch r.Method {
	case http.MethodGet:
		handleList(w, db, claims.UserId)
	case http.MethodPost:
		handleCreate(w, r, db, claims.UserId)
	default:
		mid.ErrorJSON(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func handleList(w http.ResponseWriter, db *gorm.DB, uid int64) {
	var groups []models.Group
	db.Where("user_id = ?", uid).Find(&groups)
	for i := range groups {
		loadTargets(db, &groups[i])
	}
	mid.JSON(w, http.StatusOK, groups)
}

func handleSummaryList(w http.ResponseWriter, db *gorm.DB, uid int64) {
	var groups []models.Group
	db.Where("user_id = ?", uid).Find(&groups)
	summaries := make([]models.GroupSummary, 0, len(groups))
	for _, g := range groups {
		var count int64
		db.Model(&models.GroupTarget{}).Where("group_id = ?", g.Id).Count(&count)
		summaries = append(summaries, models.GroupSummary{
			Id: g.Id, Name: g.Name, ModifiedDate: g.ModifiedDate, NumTargets: count,
		})
	}
	mid.JSON(w, http.StatusOK, models.GroupSummaries{Total: int64(len(summaries)), Groups: summaries})
}

func handleCreate(w http.ResponseWriter, r *http.Request, db *gorm.DB, uid int64) {
	var g models.Group
	if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
		mid.ErrorJSON(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if g.Name == "" {
		mid.ErrorJSON(w, http.StatusBadRequest, "Group name required")
		return
	}
	g.UserId = uid
	g.ModifiedDate = time.Now().UTC()
	if err := db.Create(&g).Error; err != nil {
		mid.ErrorJSON(w, http.StatusInternalServerError, "Failed to create group")
		return
	}
	// Save targets
	for _, t := range g.Targets {
		var existing models.Target
		if db.Where("email = ?", t.Email).First(&existing).Error != nil {
			db.Create(&t)
			existing = t
		}
		db.Create(&models.GroupTarget{GroupId: g.Id, TargetId: existing.Id})
	}
	loadTargets(db, &g)
	mid.JSON(w, http.StatusCreated, g)
}

func handleSingle(w http.ResponseWriter, r *http.Request, db *gorm.DB, uid, id int64) {
	var g models.Group
	if err := db.Where("id = ? AND user_id = ?", id, uid).First(&g).Error; err != nil {
		mid.ErrorJSON(w, http.StatusNotFound, "Group not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		loadTargets(db, &g)
		mid.JSON(w, http.StatusOK, g)
	case http.MethodPut:
		var updated models.Group
		if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
			mid.ErrorJSON(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		g.Name = updated.Name
		g.ModifiedDate = time.Now().UTC()
		db.Save(&g)
		// Replace targets
		db.Where("group_id = ?", g.Id).Delete(&models.GroupTarget{})
		for _, t := range updated.Targets {
			var existing models.Target
			if db.Where("email = ?", t.Email).First(&existing).Error != nil {
				db.Create(&t)
				existing = t
			}
			db.Create(&models.GroupTarget{GroupId: g.Id, TargetId: existing.Id})
		}
		loadTargets(db, &g)
		mid.JSON(w, http.StatusOK, g)
	case http.MethodDelete:
		db.Where("group_id = ?", id).Delete(&models.GroupTarget{})
		db.Delete(&g)
		mid.JSON(w, http.StatusOK, map[string]string{"message": "Group deleted"})
	default:
		mid.ErrorJSON(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func handleSingleSummary(w http.ResponseWriter, db *gorm.DB, uid, id int64) {
	var g models.Group
	if err := db.Where("id = ? AND user_id = ?", id, uid).First(&g).Error; err != nil {
		mid.ErrorJSON(w, http.StatusNotFound, "Group not found")
		return
	}
	var count int64
	db.Model(&models.GroupTarget{}).Where("group_id = ?", g.Id).Count(&count)
	mid.JSON(w, http.StatusOK, models.GroupSummary{
		Id: g.Id, Name: g.Name, ModifiedDate: g.ModifiedDate, NumTargets: count,
	})
}

func loadTargets(db *gorm.DB, g *models.Group) {
	db.Joins("JOIN group_targets ON group_targets.target_id = targets.id").
		Where("group_targets.group_id = ?", g.Id).Find(&g.Targets)
}
