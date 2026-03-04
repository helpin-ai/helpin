package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// Client is a lightweight Postmark email client.
type Client struct {
	serverToken string
	fromEmail   string
	httpClient  *http.Client
}

// NewClient creates a new Postmark client. Returns nil if serverToken is empty (feature disabled).
func NewClient(serverToken, fromEmail string) *Client {
	if serverToken == "" {
		return nil
	}
	return &Client{
		serverToken: serverToken,
		fromEmail:   fromEmail,
		httpClient:  &http.Client{},
	}
}

type postmarkRequest struct {
	From     string `json:"From"`
	To       string `json:"To"`
	Subject  string `json:"Subject"`
	HtmlBody string `json:"HtmlBody"`
	TextBody string `json:"TextBody"`
}

// SendEmail sends an email via the Postmark API.
func (c *Client) SendEmail(to, subject, htmlBody, textBody string) error {
	payload := postmarkRequest{
		From:     c.fromEmail,
		To:       to,
		Subject:  subject,
		HtmlBody: htmlBody,
		TextBody: textBody,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal email request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.postmarkapp.com/email", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create email request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Postmark-Server-Token", c.serverToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("postmark API returned status %d", resp.StatusCode)
	}

	return nil
}

// SendInviteEmail sends a workspace invitation email.
func (c *Client) SendInviteEmail(to, inviterName, workspaceName, joinURL string) error {
	subject := fmt.Sprintf("%s invited you to join %s on Teampulse", inviterName, workspaceName)

	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; padding: 0; background-color: #f4f4f5;">
  <div style="max-width: 480px; margin: 40px auto; background: #fff; border-radius: 8px; border: 1px solid #e4e4e7; overflow: hidden;">
    <div style="padding: 32px 24px; text-align: center;">
      <h1 style="font-size: 20px; font-weight: 600; color: #18181b; margin: 0 0 8px;">You're invited to join</h1>
      <h2 style="font-size: 24px; font-weight: 700; color: #18181b; margin: 0 0 16px;">%s</h2>
      <p style="color: #71717a; font-size: 14px; margin: 0 0 24px;">%s has invited you to collaborate on Teampulse.</p>
      <a href="%s" style="display: inline-block; background: #18181b; color: #fff; text-decoration: none; padding: 12px 32px; border-radius: 6px; font-size: 14px; font-weight: 500;">Join Workspace</a>
      <p style="color: #a1a1aa; font-size: 12px; margin: 24px 0 0;">This invitation expires in 7 days.</p>
    </div>
  </div>
</body>
</html>`, workspaceName, inviterName, joinURL)

	textBody := fmt.Sprintf(`%s invited you to join %s on Teampulse.

Click the link below to join:
%s

This invitation expires in 7 days.`, inviterName, workspaceName, joinURL)

	return c.SendEmail(to, subject, htmlBody, textBody)
}
