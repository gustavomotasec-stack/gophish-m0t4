package models

import (
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ── DB singleton ─────────────────────────────────────────────────────────────

var (
	DB   *gorm.DB
	once sync.Once
)

func GetDB() *gorm.DB {
	once.Do(func() {
		// Neon usa POSTGRES_URL ou POSTGRES_URL_NON_POOLING, Vercel usava DATABASE_URL
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			dsn = os.Getenv("POSTGRES_URL")
		}
		if dsn == "" {
			dsn = os.Getenv("POSTGRES_URL_NON_POOLING")
		}
		var err error
		DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			panic("db connect: " + err.Error())
		}
		DB.AutoMigrate(
			&User{}, &Campaign{}, &CampaignGroup{},
			&Group{}, &Target{}, &GroupTarget{},
			&Template{}, &Attachment{},
			&Page{}, &SMTP{}, &Header{},
			&Result{}, &Event{}, &MailLog{},
		)
	})
	return DB
}

// ── Constants ─────────────────────────────────────────────────────────────────

const (
	CampaignQueued     = "Queued"
	CampaignInProgress = "In progress"
	CampaignComplete   = "Completed"

	EventSent       = "Email Sent"
	EventOpened     = "Email Opened"
	EventClicked    = "Clicked Link"
	EventDataSubmit = "Submitted Data"
	EventReported   = "Email Reported"

	RecipientParameter = "rid"

	StatusScheduled = "Scheduled"
	StatusSending   = "Sending"
	StatusError     = "Error"
)

// ── Errors ────────────────────────────────────────────────────────────────────

var (
	ErrCampaignNameNotSpecified = errors.New("Campaign name not specified")
	ErrGroupNotSpecified        = errors.New("No groups specified")
	ErrTemplateNotSpecified     = errors.New("No email template specified")
	ErrPageNotSpecified         = errors.New("No landing page specified")
	ErrSMTPNotSpecified         = errors.New("No sending profile specified")
	ErrTemplateNotFound         = errors.New("Template not found")
	ErrGroupNotFound            = errors.New("Group not found")
	ErrPageNotFound             = errors.New("Page not found")
	ErrSMTPNotFound             = errors.New("Sending profile not found")
	ErrInvalidSendByDate        = errors.New("The launch date must be before the send emails by date")
	ErrCampaignNotInProgress    = errors.New("Campaign must be in progress to generate WhatsApp links")
	ErrUsernameTaken            = errors.New("Username already taken")
)

// ── Models ────────────────────────────────────────────────────────────────────

type User struct {
	Id                     int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Username               string    `json:"username" gorm:"uniqueIndex;not null"`
	Hash                   string    `json:"-"`
	ApiKey                 string    `json:"api_key" gorm:"uniqueIndex"`
	PasswordChangeRequired bool      `json:"password_change_required"`
	AccountLocked          bool      `json:"account_locked"`
	ModifiedDate           time.Time `json:"modified_date"`
}

func (u *User) SetPassword(password string) error {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Hash = string(h)
	return nil
}

func (u *User) CheckPassword(password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.Hash), []byte(password)) == nil
}

type Campaign struct {
	Id            int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId        int64     `json:"-"`
	Name          string    `json:"name" gorm:"not null"`
	CreatedDate   time.Time `json:"created_date"`
	LaunchDate    time.Time `json:"launch_date"`
	SendByDate    time.Time `json:"send_by_date"`
	CompletedDate time.Time `json:"completed_date"`
	TemplateId    int64     `json:"-"`
	PageId        int64     `json:"-"`
	SMTPId        int64     `json:"-"`
	Status        string    `json:"status"`
	URL           string    `json:"url"`

	Template Template `json:"template" gorm:"-"`
	Page     Page     `json:"page" gorm:"-"`
	SMTP     SMTP     `json:"smtp" gorm:"-"`
	Results  []Result `json:"results,omitempty" gorm:"-"`
	Groups   []Group  `json:"groups,omitempty" gorm:"-"`
	Events   []Event  `json:"timeline,omitempty" gorm:"-"`
}

// CampaignGroup is the join table between campaigns and groups
type CampaignGroup struct {
	CampaignId int64 `gorm:"primaryKey"`
	GroupId    int64 `gorm:"primaryKey"`
}

type CampaignResults struct {
	Id      int64    `json:"id"`
	Name    string   `json:"name"`
	Status  string   `json:"status"`
	Results []Result `json:"results,omitempty"`
	Events  []Event  `json:"timeline,omitempty"`
}

type CampaignSummaries struct {
	Total     int64             `json:"total"`
	Campaigns []CampaignSummary `json:"campaigns"`
}

type CampaignSummary struct {
	Id            int64         `json:"id"`
	CreatedDate   time.Time     `json:"created_date"`
	LaunchDate    time.Time     `json:"launch_date"`
	SendByDate    time.Time     `json:"send_by_date"`
	CompletedDate time.Time     `json:"completed_date"`
	Status        string        `json:"status"`
	Name          string        `json:"name"`
	Stats         CampaignStats `json:"stats"`
}

type CampaignStats struct {
	Total         int64 `json:"total"`
	EmailsSent    int64 `json:"sent"`
	OpenedEmail   int64 `json:"opened"`
	ClickedLink   int64 `json:"clicked"`
	SubmittedData int64 `json:"submitted_data"`
	EmailReported int64 `json:"email_reported"`
	Error         int64 `json:"error"`
}

type Result struct {
	Id           int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	Email        string    `json:"email"`
	Position     string    `json:"position"`
	Status       string    `json:"status"`
	Reported     bool      `json:"reported"`
	CampaignId   int64     `json:"campaign_id"`
	UserId       int64     `json:"-"`
	RId          string    `json:"id_str" gorm:"uniqueIndex"`
	SendDate     time.Time `json:"send_date"`
	ModifiedDate time.Time `json:"modified_date"`
	IP           string    `json:"ip"`
	Latitude     float64   `json:"latitude"`
	Longitude    float64   `json:"longitude"`
	Source       string    `json:"source" gorm:"default:email"`
}

type Event struct {
	Id         int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	CampaignId int64     `json:"campaign_id"`
	Email      string    `json:"email"`
	Time       time.Time `json:"time"`
	Message    string    `json:"message"`
	Details    string    `json:"details"`
}

type EventDetails struct {
	Payload url.Values        `json:"payload"`
	Browser map[string]string `json:"browser"`
}

type Group struct {
	Id           int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId       int64     `json:"-"`
	Name         string    `json:"name"`
	ModifiedDate time.Time `json:"modified_date"`
	Targets      []Target  `json:"targets" gorm:"-"`
}

type GroupSummary struct {
	Id           int64     `json:"id"`
	Name         string    `json:"name"`
	ModifiedDate time.Time `json:"modified_date"`
	NumTargets   int64     `json:"num_targets"`
}

type GroupSummaries struct {
	Total  int64          `json:"total"`
	Groups []GroupSummary `json:"groups"`
}

type Target struct {
	Id        int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email" gorm:"uniqueIndex"`
	Position  string `json:"position"`
}

type GroupTarget struct {
	GroupId  int64 `gorm:"primaryKey"`
	TargetId int64 `gorm:"primaryKey"`
}

type Template struct {
	Id           int64        `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId       int64        `json:"-"`
	Name         string       `json:"name"`
	Subject      string       `json:"subject"`
	Text         string       `json:"text"`
	HTML         string       `json:"html"`
	ModifiedDate time.Time    `json:"modified_date"`
	Attachments  []Attachment `json:"attachments" gorm:"-"`
}

type Attachment struct {
	Id         int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	TemplateId int64  `json:"-"`
	Content    string `json:"content"`
	Type       string `json:"type"`
	Name       string `json:"name"`
}

type Page struct {
	Id                 int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId             int64     `json:"-"`
	Name               string    `json:"name"`
	HTML               string    `json:"html"`
	CaptureCredentials bool      `json:"capture_credentials"`
	CapturePasswords   bool      `json:"capture_passwords"`
	RedirectURL        string    `json:"redirect_url"`
	ModifiedDate       time.Time `json:"modified_date"`
}

type SMTP struct {
	Id                int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId            int64     `json:"-"`
	Name              string    `json:"name"`
	Host              string    `json:"host"`
	Port              int       `json:"port"`
	Username          string    `json:"username"`
	Password          string    `json:"password"`
	FromAddress       string    `json:"from_address"`
	IgnoreCertErrors  bool      `json:"ignore_cert_errors"`
	ModifiedDate      time.Time `json:"modified_date"`
	Headers           []Header  `json:"headers" gorm:"-"`
}

type Header struct {
	Id     int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	SMTPId int64  `json:"-"`
	Key    string `json:"key"`
	Value  string `json:"value"`
}

type MailLog struct {
	Id         int64     `gorm:"primaryKey;autoIncrement"`
	UserId     int64
	CampaignId int64
	RId        string
	SendDate   time.Time
	Processing bool `gorm:"default:false"`
}

// ── RId generation ────────────────────────────────────────────────────────────

const ridChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

func GenerateRId() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, 7)
	for i := range b {
		b[i] = ridChars[r.Intn(len(ridChars))]
	}
	return string(b)
}

// ── Campaign methods ──────────────────────────────────────────────────────────

func (c *Campaign) Validate() error {
	switch {
	case c.Name == "":
		return ErrCampaignNameNotSpecified
	case c.Template.Name == "":
		return ErrTemplateNotSpecified
	case c.Page.Name == "":
		return ErrPageNotSpecified
	case len(c.Groups) == 0:
		return ErrGroupNotSpecified
	case c.SMTP.Name == "":
		return ErrSMTPNotSpecified
	case !c.SendByDate.IsZero() && !c.LaunchDate.IsZero() && c.SendByDate.Before(c.LaunchDate):
		return ErrInvalidSendByDate
	}
	return nil
}

// WhatsAppLinkResult holds a generated rid and phishing URL
type WhatsAppLinkResult struct {
	RId  string `json:"rid"`
	URL  string `json:"url"`
	Name string `json:"name"`
}

// GenerateWhatsAppLinks creates N Result records with source="whatsapp"
func GenerateWhatsAppLinks(db *gorm.DB, campaignId, userId int64, count int, baseURL string) ([]WhatsAppLinkResult, error) {
	var existing int64
	db.Model(&Result{}).Where("campaign_id = ? AND source = ?", campaignId, "whatsapp").Count(&existing)

	now := time.Now().UTC()
	links := make([]WhatsAppLinkResult, 0, count)

	tx := db.Begin()
	for i := 0; i < count; i++ {
		seq := int(existing) + i + 1
		name := fmt.Sprintf("WhatsApp #%d", seq)
		rid := GenerateRId()
		r := &Result{
			FirstName:    name,
			Email:        fmt.Sprintf("whatsapp_%d_%d@internal", campaignId, seq),
			Status:       StatusSending,
			CampaignId:   campaignId,
			UserId:       userId,
			RId:          rid,
			SendDate:     now,
			ModifiedDate: now,
			Source:       "whatsapp",
		}
		if err := tx.Create(r).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		phishURL, _ := url.Parse(baseURL)
		q := phishURL.Query()
		q.Set(RecipientParameter, rid)
		phishURL.RawQuery = q.Encode()
		links = append(links, WhatsAppLinkResult{RId: rid, URL: phishURL.String(), Name: name})
	}
	tx.Commit()
	return links, nil
}

// GetCampaignStats returns aggregated stats for a campaign (email only)
func GetCampaignStats(db *gorm.DB, cid int64) CampaignStats {
	s := CampaignStats{}
	q := db.Model(&Result{}).Where("campaign_id = ? AND source != ?", cid, "whatsapp")
	q.Count(&s.Total)
	q.Where("status = ?", EventDataSubmit).Count(&s.SubmittedData)
	q.Where("status = ?", EventClicked).Count(&s.ClickedLink)
	q.Where("reported = ?", true).Count(&s.EmailReported)
	s.ClickedLink += s.SubmittedData
	q.Where("status = ?", EventOpened).Count(&s.OpenedEmail)
	s.OpenedEmail += s.ClickedLink
	q.Where("status = ?", EventSent).Count(&s.EmailsSent)
	s.EmailsSent += s.OpenedEmail
	q.Where("status = ?", StatusError).Count(&s.Error)
	return s
}
