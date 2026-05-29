package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gophish/gophish-vercel/lib/models"
	"gorm.io/gorm"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	db := models.GetDB()
	rid := r.URL.Query().Get(models.RecipientParameter)
	if rid == "" {
		http.NotFound(w, r)
		return
	}

	var result models.Result
	if err := db.Where("r_id = ?", rid).First(&result).Error; err != nil {
		http.NotFound(w, r)
		return
	}

	var campaign models.Campaign
	if err := db.Where("id = ?", result.CampaignId).First(&campaign).Error; err != nil {
		http.NotFound(w, r)
		return
	}
	if campaign.Status == models.CampaignComplete {
		http.NotFound(w, r)
		return
	}

	browser := map[string]string{
		"address":    r.RemoteAddr,
		"user-agent": r.Header.Get("User-Agent"),
	}

	if r.Method == http.MethodGet {
		// Detect 1×1 pixel request (email open)
		accept := r.Header.Get("Accept")
		if strings.HasSuffix(r.URL.Path, ".png") || strings.HasSuffix(r.URL.Path, ".gif") ||
			(accept != "" && !strings.Contains(accept, "text/html")) {
			recordEvent(db, &result, models.EventOpened, browser, nil)
			w.Header().Set("Content-Type", "image/gif")
			w.Write([]byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00,
				0x01, 0x00, 0x00, 0xff, 0x00, 0x2c, 0x00, 0x00,
				0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x00, 0x3b})
			return
		}

		// Link click — serve landing page
		recordEvent(db, &result, models.EventClicked, browser, nil)

		var page models.Page
		if err := db.Where("id = ?", campaign.PageId).First(&page).Error; err != nil {
			http.NotFound(w, r)
			return
		}
		html := page.HTML
		if page.CaptureCredentials {
			html = strings.ReplaceAll(html, "</form>",
				`<input type="hidden" name="rid" value="`+rid+`"></form>`)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
		return
	}

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad form data", http.StatusBadRequest)
			return
		}
		payload := r.Form
		payload.Del("rid")

		recordEvent(db, &result, models.EventDataSubmit, browser, map[string]interface{}{
			"payload": payload,
		})

		var page models.Page
		db.Where("id = ?", campaign.PageId).First(&page)

		if page.RedirectURL != "" {
			http.Redirect(w, r, page.RedirectURL, http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body><p>Obrigado.</p></body></html>"))
		return
	}
}

func recordEvent(db *gorm.DB, result *models.Result, message string,
	browser map[string]string, extra map[string]interface{}) {

	d := map[string]interface{}{"browser": browser}
	for k, v := range extra {
		d[k] = v
	}
	detailsBytes, _ := json.Marshal(d)

	event := models.Event{
		CampaignId: result.CampaignId,
		Email:      result.Email,
		Time:       time.Now().UTC(),
		Message:    message,
		Details:    string(detailsBytes),
	}
	db.Create(&event)

	// Only upgrade status, never downgrade
	priority := map[string]int{
		models.StatusScheduled: 0, models.StatusSending: 1,
		models.EventSent: 2, models.EventOpened: 3,
		models.EventClicked: 4, models.EventDataSubmit: 5,
	}
	if priority[message] > priority[result.Status] {
		db.Model(result).Updates(map[string]interface{}{
			"status": message, "modified_date": time.Now().UTC(),
		})
	}
}
