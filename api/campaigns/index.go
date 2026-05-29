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
	// /api/campaigns/           → index
	// /api/campaigns/summary    → summary list
	// /api/campaigns/{id}       → single campaign
	// /api/campaigns/{id}/results, /complete, /whatsapp, /summary

	if len(parts) >= 3 && parts[2] == "summary" && len(parts) == 3 {
		handleSummaryList(w, r, db, claims.UserId)
		return
	}

	if len(parts) == 3 {
		idStr := parts[2]
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			mid.ErrorJSON(w, http.StatusBadRequest, "Invalid campaign id")
			return
		}
		handleSingle(w, r, db, claims.UserId, id)
		return
	}

	if len(parts) == 4 {
		id, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			mid.ErrorJSON(w, http.StatusBadRequest, "Invalid campaign id")
			return
		}
		switch parts[3] {
		case "results":
			handleResults(w, r, db, claims.UserId, id)
		case "summary":
			handleSingleSummary(w, r, db, claims.UserId, id)
		case "complete":
			handleComplete(w, r, db, claims.UserId, id)
		case "whatsapp":
			handleWhatsApp(w, r, db, claims.UserId, id)
		default:
			mid.ErrorJSON(w, http.StatusNotFound, "Not found")
		}
		return
	}

	// /api/campaigns/ — list or create
	switch r.Method {
	case http.MethodGet:
		handleList(w, r, db, claims.UserId)
	case http.MethodPost:
		handleCreate(w, r, db, claims.UserId)
	default:
		mid.ErrorJSON(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// ─── list ────────────────────────────────────────────────────────────────────

func handleList(w http.ResponseWriter, r *http.Request, db *gorm.DB, uid int64) {
	var cs []models.Campaign
	db.Where("user_id = ?", uid).Find(&cs)
	for i := range cs {
		loadCampaignDetails(db, &cs[i])
	}
	mid.JSON(w, http.StatusOK, cs)
}

// ─── summary list ─────────────────────────────────────────────────────────────

func handleSummaryList(w http.ResponseWriter, r *http.Request, db *gorm.DB, uid int64) {
	var rows []models.Campaign
	db.Where("user_id = ?", uid).Find(&rows)
	summaries := make([]models.CampaignSummary, 0, len(rows))
	for _, c := range rows {
		s := models.CampaignSummary{
			Id: c.Id, Name: c.Name, Status: c.Status,
			CreatedDate: c.CreatedDate, LaunchDate: c.LaunchDate,
			SendByDate: c.SendByDate, CompletedDate: c.CompletedDate,
			Stats: models.GetCampaignStats(db, c.Id),
		}
		summaries = append(summaries, s)
	}
	mid.JSON(w, http.StatusOK, models.CampaignSummaries{Total: int64(len(summaries)), Campaigns: summaries})
}

// ─── create ───────────────────────────────────────────────────────────────────

func handleCreate(w http.ResponseWriter, r *http.Request, db *gorm.DB, uid int64) {
	var c models.Campaign
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		mid.ErrorJSON(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := c.Validate(); err != nil {
		mid.ErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	// Resolve template
	var tmpl models.Template
	if err := db.Where("name = ? AND user_id = ?", c.Template.Name, uid).First(&tmpl).Error; err != nil {
		mid.ErrorJSON(w, http.StatusBadRequest, models.ErrTemplateNotFound.Error())
		return
	}
	// Resolve page
	var page models.Page
	if err := db.Where("name = ? AND user_id = ?", c.Page.Name, uid).First(&page).Error; err != nil {
		mid.ErrorJSON(w, http.StatusBadRequest, models.ErrPageNotFound.Error())
		return
	}
	// Resolve SMTP
	var smtp models.SMTP
	if err := db.Where("name = ? AND user_id = ?", c.SMTP.Name, uid).First(&smtp).Error; err != nil {
		mid.ErrorJSON(w, http.StatusBadRequest, models.ErrSMTPNotFound.Error())
		return
	}

	now := time.Now().UTC()
	c.UserId = uid
	c.CreatedDate = now
	c.CompletedDate = time.Time{}
	c.TemplateId = tmpl.Id
	c.PageId = page.Id
	c.SMTPId = smtp.Id
	c.Status = models.CampaignQueued
	if c.LaunchDate.IsZero() {
		c.LaunchDate = now
	} else {
		c.LaunchDate = c.LaunchDate.UTC()
	}
	if !c.SendByDate.IsZero() {
		c.SendByDate = c.SendByDate.UTC()
	}
	if c.LaunchDate.Before(now) || c.LaunchDate.Equal(now) {
		c.Status = models.CampaignInProgress
	}

	if err := db.Create(&c).Error; err != nil {
		mid.ErrorJSON(w, http.StatusInternalServerError, "Failed to create campaign")
		return
	}

	// Save campaign-group relationships and create results + maillogs
	groups := c.Groups
	totalRecipients := 0
	for _, g := range groups {
		var grp models.Group
		if err := db.Where("name = ? AND user_id = ?", g.Name, uid).First(&grp).Error; err != nil {
			continue
		}
		db.Create(&models.CampaignGroup{CampaignId: c.Id, GroupId: grp.Id})

		var targets []models.Target
		db.Joins("JOIN group_targets ON group_targets.target_id = targets.id").
			Where("group_targets.group_id = ?", grp.Id).Find(&targets)
		totalRecipients += len(targets)
	}

	// Re-fetch groups to get targets for result creation
	recipientIndex := 0
	seen := map[string]bool{}
	for _, g := range groups {
		var grp models.Group
		if err := db.Where("name = ? AND user_id = ?", g.Name, uid).First(&grp).Error; err != nil {
			continue
		}
		var targets []models.Target
		db.Joins("JOIN group_targets ON group_targets.target_id = targets.id").
			Where("group_targets.group_id = ?", grp.Id).Find(&targets)
		for _, t := range targets {
			if seen[t.Email] {
				continue
			}
			seen[t.Email] = true
			sendDate := generateSendDate(c, recipientIndex, totalRecipients)
			rid := models.GenerateRId()
			status := models.StatusScheduled
			processing := false
			if sendDate.Before(now) || sendDate.Equal(now) {
				status = models.StatusSending
				processing = true
			}
			result := models.Result{
				FirstName: t.FirstName, LastName: t.LastName,
				Email: t.Email, Position: t.Position,
				Status: status, CampaignId: c.Id, UserId: uid,
				RId: rid, SendDate: sendDate, ModifiedDate: now,
			}
			db.Create(&result)
			db.Create(&models.MailLog{
				UserId: uid, CampaignId: c.Id,
				RId: rid, SendDate: sendDate, Processing: processing,
			})
			recipientIndex++
		}
	}

	// Add "Campaign Created" event
	db.Create(&models.Event{
		CampaignId: c.Id, Time: now, Message: "Campaign Created",
	})

	loadCampaignDetails(db, &c)
	mid.JSON(w, http.StatusCreated, c)
}

// ─── single ───────────────────────────────────────────────────────────────────

func handleSingle(w http.ResponseWriter, r *http.Request, db *gorm.DB, uid, id int64) {
	var c models.Campaign
	if err := db.Where("id = ? AND user_id = ?", id, uid).First(&c).Error; err != nil {
		mid.ErrorJSON(w, http.StatusNotFound, "Campaign not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		loadCampaignDetails(db, &c)
		mid.JSON(w, http.StatusOK, c)
	case http.MethodDelete:
		db.Where("campaign_id = ?", id).Delete(&models.Result{})
		db.Where("campaign_id = ?", id).Delete(&models.Event{})
		db.Where("campaign_id = ?", id).Delete(&models.MailLog{})
		db.Where("campaign_id = ?", id).Delete(&models.CampaignGroup{})
		db.Delete(&c)
		mid.JSON(w, http.StatusOK, map[string]string{"message": "Campaign deleted"})
	default:
		mid.ErrorJSON(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// ─── results ─────────────────────────────────────────────────────────────────

func handleResults(w http.ResponseWriter, r *http.Request, db *gorm.DB, uid, id int64) {
	var c models.Campaign
	if err := db.Where("id = ? AND user_id = ?", id, uid).First(&c).Error; err != nil {
		mid.ErrorJSON(w, http.StatusNotFound, "Campaign not found")
		return
	}
	var results []models.Result
	db.Where("campaign_id = ? AND user_id = ?", id, uid).Find(&results)
	var events []models.Event
	db.Where("campaign_id = ?", id).Find(&events)
	cr := models.CampaignResults{
		Id: c.Id, Name: c.Name, Status: c.Status,
		Results: results, Events: events,
	}
	mid.JSON(w, http.StatusOK, cr)
}

// ─── summary single ───────────────────────────────────────────────────────────

func handleSingleSummary(w http.ResponseWriter, r *http.Request, db *gorm.DB, uid, id int64) {
	var c models.Campaign
	if err := db.Where("id = ? AND user_id = ?", id, uid).First(&c).Error; err != nil {
		mid.ErrorJSON(w, http.StatusNotFound, "Campaign not found")
		return
	}
	s := models.CampaignSummary{
		Id: c.Id, Name: c.Name, Status: c.Status,
		CreatedDate: c.CreatedDate, LaunchDate: c.LaunchDate,
		Stats: models.GetCampaignStats(db, c.Id),
	}
	mid.JSON(w, http.StatusOK, s)
}

// ─── complete ─────────────────────────────────────────────────────────────────

func handleComplete(w http.ResponseWriter, r *http.Request, db *gorm.DB, uid, id int64) {
	var c models.Campaign
	if err := db.Where("id = ? AND user_id = ?", id, uid).First(&c).Error; err != nil {
		mid.ErrorJSON(w, http.StatusNotFound, "Campaign not found")
		return
	}
	if c.Status == models.CampaignComplete {
		mid.JSON(w, http.StatusOK, map[string]string{"message": "Campaign already completed"})
		return
	}
	db.Where("campaign_id = ?", id).Delete(&models.MailLog{})
	now := time.Now().UTC()
	db.Model(&c).Updates(map[string]interface{}{
		"status": models.CampaignComplete, "completed_date": now,
	})
	mid.JSON(w, http.StatusOK, map[string]string{"message": "Campaign completed"})
}

// ─── whatsapp ─────────────────────────────────────────────────────────────────

func handleWhatsApp(w http.ResponseWriter, r *http.Request, db *gorm.DB, uid, id int64) {
	if r.Method != http.MethodPost {
		mid.ErrorJSON(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var body struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Count < 1 || body.Count > 1000 {
		mid.ErrorJSON(w, http.StatusBadRequest, "count must be between 1 and 1000")
		return
	}
	var c models.Campaign
	if err := db.Where("id = ? AND user_id = ?", id, uid).First(&c).Error; err != nil {
		mid.ErrorJSON(w, http.StatusNotFound, "Campaign not found")
		return
	}
	if c.Status != models.CampaignInProgress {
		mid.ErrorJSON(w, http.StatusBadRequest, models.ErrCampaignNotInProgress.Error())
		return
	}
	links, err := models.GenerateWhatsAppLinks(db, id, uid, body.Count, c.URL)
	if err != nil {
		mid.ErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	mid.JSON(w, http.StatusOK, links)
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func loadCampaignDetails(db *gorm.DB, c *models.Campaign) {
	db.Where("id = ?", c.TemplateId).First(&c.Template)
	db.Where("id = ?", c.PageId).First(&c.Page)
	db.Where("id = ?", c.SMTPId).First(&c.SMTP)
	db.Where("campaign_id = ?", c.Id).Find(&c.Results)
	db.Where("campaign_id = ?", c.Id).Find(&c.Events)

	var cgs []models.CampaignGroup
	db.Where("campaign_id = ?", c.Id).Find(&cgs)
	for _, cg := range cgs {
		var g models.Group
		db.First(&g, cg.GroupId)
		c.Groups = append(c.Groups, g)
	}
}

func generateSendDate(c models.Campaign, idx, total int) time.Time {
	if c.SendByDate.IsZero() || c.SendByDate.Equal(c.LaunchDate) || total == 0 {
		return c.LaunchDate
	}
	totalMinutes := c.SendByDate.Sub(c.LaunchDate).Minutes()
	minutesPerEmail := totalMinutes / float64(total)
	offset := int(minutesPerEmail * float64(idx))
	return c.LaunchDate.Add(time.Duration(offset) * time.Minute)
}
