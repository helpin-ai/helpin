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

// EmailHeader is a single custom Postmark header.
type EmailHeader struct {
	Name  string `json:"Name"`
	Value string `json:"Value"`
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
	From     string        `json:"From"`
	To       string        `json:"To"`
	Subject  string        `json:"Subject"`
	HtmlBody string        `json:"HtmlBody"`
	TextBody string        `json:"TextBody"`
	ReplyTo  string        `json:"ReplyTo,omitempty"`
	Headers  []EmailHeader `json:"Headers,omitempty"`
}

type postmarkResponse struct {
	ErrorCode   int    `json:"ErrorCode"`
	Message     string `json:"Message"`
	MessageID   string `json:"MessageID"`
	SubmittedAt string `json:"SubmittedAt"`
	To          string `json:"To"`
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
	_, err := c.send(payload)
	return err
}

// FromEmail returns the configured Postmark sender address.
func (c *Client) FromEmail() string {
	if c == nil {
		return ""
	}
	return c.fromEmail
}

// SetHTTPClient overrides the underlying HTTP client, primarily for tests.
func (c *Client) SetHTTPClient(httpClient *http.Client) {
	if c == nil || httpClient == nil {
		return
	}
	c.httpClient = httpClient
}

// SendEmailWithHeaders sends an email with a custom From/Reply-To and extra RFC headers.
// It returns the Postmark MessageID for durable logging.
func (c *Client) SendEmailWithHeaders(from, to, subject, htmlBody, textBody, replyTo string, headers []EmailHeader) (string, error) {
	payload := postmarkRequest{
		From:     from,
		To:       to,
		Subject:  subject,
		HtmlBody: htmlBody,
		TextBody: textBody,
		ReplyTo:  replyTo,
		Headers:  headers,
	}
	return c.send(payload)
}

func (c *Client) send(payload postmarkRequest) (string, error) {
	if c == nil {
		return "", fmt.Errorf("postmark client not configured")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal email request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.postmarkapp.com/email", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create email request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Postmark-Server-Token", c.serverToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send email: %w", err)
	}
	defer resp.Body.Close()

	var decoded postmarkResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", fmt.Errorf("decode postmark response: %w", err)
	}

	if resp.StatusCode >= 400 {
		if decoded.Message != "" {
			return "", fmt.Errorf("postmark API returned status %d: %s", resp.StatusCode, decoded.Message)
		}
		return "", fmt.Errorf("postmark API returned status %d", resp.StatusCode)
	}

	return decoded.MessageID, nil
}

// SendInviteEmail sends a workspace invitation email.
func (c *Client) SendInviteEmail(to, inviterName, workspaceName, joinURL string) error {
	subject := fmt.Sprintf("%s invited you to join %s on Helpin", inviterName, workspaceName)

	// Get the first letter of workspace name for the avatar.
	wsInitial := string([]rune(workspaceName)[0])

	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta http-equiv="X-UA-Compatible" content="IE=edge">
  <title>Workspace Invitation</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; margin: 0; padding: 0; background-color: #f0f0f3; -webkit-font-smoothing: antialiased;">
  <!-- Preheader text (hidden) -->
  <div style="display: none; max-height: 0; overflow: hidden;">
    %s has invited you to collaborate on %s &mdash; click to join the workspace.
  </div>

  <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" style="background-color: #f0f0f3;">
    <tr>
      <td align="center" style="padding: 48px 16px;">
        <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" style="max-width: 520px;">

          <!-- Logo -->
          <tr>
            <td align="center" style="padding-bottom: 32px;">
              <span style="font-size: 22px; font-weight: 700; color: #18181b; letter-spacing: -0.5px;">Helpin</span>
            </td>
          </tr>

          <!-- Main Card -->
          <tr>
            <td style="background: #ffffff; border-radius: 12px; box-shadow: 0 1px 3px rgba(0,0,0,0.06), 0 1px 2px rgba(0,0,0,0.04);">
              <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0">

                <!-- Top accent bar -->
                <tr>
                  <td style="height: 4px; background: linear-gradient(90deg, #18181b 0%%, #3b3b3f 100%%); border-radius: 12px 12px 0 0; font-size: 0; line-height: 0;">&nbsp;</td>
                </tr>

                <!-- Content -->
                <tr>
                  <td style="padding: 40px 36px 36px;">
                    <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0">

                      <!-- Workspace avatar -->
                      <tr>
                        <td align="center" style="padding-bottom: 24px;">
                          <div style="display: inline-block; width: 56px; height: 56px; line-height: 56px; border-radius: 14px; background-color: #18181b; color: #ffffff; font-size: 22px; font-weight: 700; text-align: center;">%s</div>
                        </td>
                      </tr>

                      <!-- Heading -->
                      <tr>
                        <td align="center" style="padding-bottom: 8px;">
                          <h1 style="margin: 0; font-size: 22px; font-weight: 700; color: #18181b; line-height: 1.3;">You're invited to join</h1>
                        </td>
                      </tr>

                      <!-- Workspace name -->
                      <tr>
                        <td align="center" style="padding-bottom: 16px;">
                          <h2 style="margin: 0; font-size: 26px; font-weight: 800; color: #18181b; line-height: 1.2;">%s</h2>
                        </td>
                      </tr>

                      <!-- Description -->
                      <tr>
                        <td align="center" style="padding-bottom: 32px;">
                          <p style="margin: 0; font-size: 15px; line-height: 1.6; color: #52525b;">
                            <strong style="color: #18181b;">%s</strong> has invited you to collaborate on this workspace. Join the team to get started.
                          </p>
                        </td>
                      </tr>

                      <!-- CTA Button -->
                      <tr>
                        <td align="center" style="padding-bottom: 24px;">
                          <table role="presentation" cellspacing="0" cellpadding="0" border="0">
                            <tr>
                              <td style="border-radius: 8px; background-color: #18181b;">
                                <a href="%s" target="_blank" style="display: inline-block; padding: 14px 40px; font-size: 15px; font-weight: 600; color: #ffffff; text-decoration: none; letter-spacing: 0.2px;">Accept Invitation</a>
                              </td>
                            </tr>
                          </table>
                        </td>
                      </tr>

                      <!-- Expiry notice -->
                      <tr>
                        <td align="center">
                          <p style="margin: 0; font-size: 13px; color: #a1a1aa;">This invitation expires in 7 days.</p>
                        </td>
                      </tr>

                    </table>
                  </td>
                </tr>
              </table>
            </td>
          </tr>

          <!-- Footer -->
          <tr>
            <td align="center" style="padding: 28px 16px 0;">
              <p style="margin: 0 0 6px; font-size: 12px; color: #a1a1aa; line-height: 1.5;">
                You received this email because someone invited you to a workspace on Helpin.
              </p>
              <p style="margin: 0; font-size: 12px; color: #a1a1aa;">
                If you didn't expect this, you can safely ignore it.
              </p>
            </td>
          </tr>

        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, inviterName, workspaceName, wsInitial, workspaceName, inviterName, joinURL)

	textBody := fmt.Sprintf(`%s invited you to join %s on Helpin.

Click the link below to join:
%s

This invitation expires in 7 days.`, inviterName, workspaceName, joinURL)

	return c.SendEmail(to, subject, htmlBody, textBody)
}
