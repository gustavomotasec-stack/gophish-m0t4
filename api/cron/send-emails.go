package handler

import (
	"net/http"
	"os"
	"time"

	"github.com/gophish/gophish-vercel/lib/mailer"
	mid "github.com/gophish/gophish-vercel/lib/middleware"
	"github.com/gophish/gophish-vercel/lib/models"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	// Verify cron secret to prevent unauthorized calls
	secret := os.Getenv("CRON_SECRET")
	if secret != "" && r.Header.Get("authorization") != "Bearer "+secret {
		mid.ErrorJSON(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	db := models.GetDB()
	now := time.Now().UTC()

	// Find all maillogs ready to send
	var logs []models.MailLog
	db.Where("send_date <= ? AND processing = false", now).Find(&logs)

	sent := 0
	failed := 0

	for _, ml := range logs {
		// Mark as processing to avoid double-send
		db.Model(&ml).Update("processing", true)

		var result models.Result
		if err := db.Where("r_id = ?", ml.RId).First(&result).Error; err != nil {
			db.Delete(&ml)
			continue
		}

		var campaign models.Campaign
		if err := db.Where("id = ?", ml.CampaignId).First(&campaign).Error; err != nil {
			db.Delete(&ml)
			continue
		}
		if campaign.Status == models.CampaignComplete {
			db.Delete(&ml)
			continue
		}

		var tmpl models.Template
		db.Where("id = ?", campaign.TemplateId).First(&tmpl)

		var smtp models.SMTP
		db.Where("id = ?", campaign.SMTPId).First(&smtp)

		// Build tracking URLs
		trackURL := mailer.BuildTrackingURL(campaign.URL, result.RId)
		ctx := mailer.EmailContext{
			FirstName:   result.FirstName,
			LastName:    result.LastName,
			Email:       result.Email,
			Position:    result.Position,
			TrackingURL: trackURL,
			URL:         trackURL,
			From:        smtp.FromAddress,
			RId:         result.RId,
		}

		subject, _ := mailer.RenderTemplate(tmpl.Subject, ctx)
		htmlBody, _ := mailer.RenderTemplate(tmpl.HTML, ctx)
		textBody, _ := mailer.RenderTemplate(tmpl.Text, ctx)

		// Add open tracking pixel to HTML
		if htmlBody != "" {
			// Use a separate pixel endpoint
			pixelURL := campaign.URL
			if pixelURL != "" {
				htmlBody += mailer.BuildOpenTrackingPixel(trackURL + "&pixel=1")
			}
		}

		err := mailer.SendEmail(
			smtp.Host, smtp.Port,
			smtp.Username, smtp.Password,
			smtp.FromAddress,
			smtp.IgnoreCertErrors,
			result.Email, subject, htmlBody, textBody,
		)

		if err != nil {
			// Mark result as error
			db.Model(&result).Updates(map[string]interface{}{
				"status": models.StatusError, "modified_date": time.Now().UTC(),
			})
			db.Create(&models.Event{
				CampaignId: campaign.Id, Email: result.Email,
				Time: time.Now().UTC(), Message: "Error Sending Email",
				Details: `{"error":"` + err.Error() + `"}`,
			})
			failed++
		} else {
			// Mark result as sent
			db.Model(&result).Updates(map[string]interface{}{
				"status": models.EventSent, "modified_date": time.Now().UTC(),
			})
			db.Create(&models.Event{
				CampaignId: campaign.Id, Email: result.Email,
				Time: time.Now().UTC(), Message: models.EventSent,
			})
			sent++
		}

		// Remove from maillog regardless of success/failure
		db.Delete(&ml)
	}

	mid.JSON(w, http.StatusOK, map[string]interface{}{
		"processed": len(logs),
		"sent":      sent,
		"failed":    failed,
		"timestamp": now,
	})
}
