package mailer

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"net/smtp"
	"net/url"
	"strings"
)

type EmailContext struct {
	FirstName   string
	LastName    string
	Email       string
	Position    string
	TrackingURL string
	URL         string
	From        string
	RId         string
}

// SendEmail sends a phishing email for the given result
func SendEmail(smtpHost string, smtpPort int, smtpUser, smtpPass, fromAddr string,
	ignoreCert bool, to, subject, htmlBody, textBody string) error {

	addr := fmt.Sprintf("%s:%d", smtpHost, smtpPort)

	var auth smtp.Auth
	if smtpUser != "" {
		auth = smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	}

	// Build MIME message
	boundary := "GoPhishBoundary42"
	var msg strings.Builder
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString(fmt.Sprintf("From: %s\r\n", fromAddr))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", boundary))

	if textBody != "" {
		msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
		msg.WriteString(textBody + "\r\n")
	}

	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	msg.WriteString(htmlBody + "\r\n")
	msg.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	if ignoreCert {
		tlsCfg := &tls.Config{InsecureSkipVerify: true, ServerName: smtpHost}
		conn, err := tls.Dial("tcp", addr, tlsCfg)
		if err != nil {
			return err
		}
		client, err := smtp.NewClient(conn, smtpHost)
		if err != nil {
			return err
		}
		defer client.Close()
		if auth != nil {
			if err = client.Auth(auth); err != nil {
				return err
			}
		}
		if err = client.Mail(fromAddr); err != nil {
			return err
		}
		if err = client.Rcpt(to); err != nil {
			return err
		}
		wc, err := client.Data()
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(wc, msg.String())
		wc.Close()
		return err
	}

	return smtp.SendMail(addr, auth, fromAddr, []string{to}, []byte(msg.String()))
}

// RenderTemplate processes a GoPhish-style template with the given context
func RenderTemplate(tmplStr string, ctx EmailContext) (string, error) {
	// Replace GoPhish template variables
	r := strings.NewReplacer(
		"{{.FirstName}}", ctx.FirstName,
		"{{.LastName}}", ctx.LastName,
		"{{.Email}}", ctx.Email,
		"{{.Position}}", ctx.Position,
		"{{.From}}", ctx.From,
		"{{.TrackingURL}}", ctx.TrackingURL,
		"{{.URL}}", ctx.URL,
		"{{.RId}}", ctx.RId,
	)
	result := r.Replace(tmplStr)

	// Also try Go template parsing for more complex templates
	t, err := template.New("").Parse(result)
	if err != nil {
		return result, nil // return simple replacement result on parse error
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return result, nil
	}
	return buf.String(), nil
}

// BuildTrackingURL creates the phishing URL with the rid parameter
func BuildTrackingURL(baseURL, rid string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return baseURL + "?rid=" + rid
	}
	q := u.Query()
	q.Set("rid", rid)
	u.RawQuery = q.Encode()
	return u.String()
}

// BuildOpenTrackingPixel returns an img tag for open tracking
func BuildOpenTrackingPixel(trackURL string) string {
	return fmt.Sprintf(`<img src="%s" style="display:none" alt="" width="1" height="1">`, trackURL)
}
