package sync

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/oauth"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	gmailAPIBase = "https://gmail.googleapis.com/gmail/v1/users/me"
)

// GmailMessage represents a parsed Gmail message.
type GmailMessage struct {
	ID        string
	ThreadID  string
	Subject   string
	From      string
	FromName  string
	To        []string
	CC        []string
	Date      time.Time
	BodyText  string
	BodyHTML  string
	HistoryID string
}

// GmailSyncClient wraps the Gmail REST API for email sync operations.
type GmailSyncClient struct {
	oauthClient   *oauth.GmailOAuthClient
	emailRepo     *repository.CRMEmailRepository
	encryptionKey []byte
	httpClient    *http.Client
}

// NewGmailSyncClient creates a new GmailSyncClient.
func NewGmailSyncClient(oauthClient *oauth.GmailOAuthClient, emailRepo *repository.CRMEmailRepository, encryptionKey []byte) *GmailSyncClient {
	if oauthClient == nil {
		return nil
	}
	return &GmailSyncClient{
		oauthClient:   oauthClient,
		emailRepo:     emailRepo,
		encryptionKey: encryptionKey,
		httpClient:    &http.Client{Timeout: 60 * time.Second},
	}
}

// GetValidToken returns a valid access token for the account, refreshing if expired.
func (c *GmailSyncClient) GetValidToken(ctx context.Context, account *model.CRMEmailAccount) (string, error) {
	if account.AccessTokenEncrypted == nil {
		return "", fmt.Errorf("no access token stored for account %s", account.ID)
	}

	accessToken, err := crypto.DecryptString(*account.AccessTokenEncrypted, c.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("decrypt access token: %w", err)
	}

	// If token hasn't expired yet, return it.
	if account.TokenExpiresAt != nil && account.TokenExpiresAt.After(time.Now().Add(1*time.Minute)) {
		return accessToken, nil
	}

	// Token expired — refresh it.
	if account.RefreshTokenEncrypted == nil {
		return "", fmt.Errorf("no refresh token stored for account %s", account.ID)
	}

	refreshToken, err := crypto.DecryptString(*account.RefreshTokenEncrypted, c.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("decrypt refresh token: %w", err)
	}

	newPair, err := c.oauthClient.RefreshToken(ctx, refreshToken)
	if err != nil {
		return "", fmt.Errorf("refresh token: %w", err)
	}

	// Encrypt and store new tokens.
	encAccess, err := crypto.EncryptString(newPair.AccessToken, c.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("encrypt new access token: %w", err)
	}
	account.AccessTokenEncrypted = &encAccess

	if newPair.RefreshToken != refreshToken {
		encRefresh, err := crypto.EncryptString(newPair.RefreshToken, c.encryptionKey)
		if err != nil {
			return "", fmt.Errorf("encrypt new refresh token: %w", err)
		}
		account.RefreshTokenEncrypted = &encRefresh
	}

	account.TokenExpiresAt = &newPair.ExpiresAt
	if err := c.emailRepo.UpdateAccount(ctx, account); err != nil {
		slog.ErrorContext(ctx, "failed to update refreshed tokens", "error", err, "account_id", account.ID)
	}

	return newPair.AccessToken, nil
}

// ListMessages fetches messages from Gmail matching the query.
func (c *GmailSyncClient) ListMessages(ctx context.Context, accessToken, query string, maxResults int, pageToken string) ([]GmailMessage, string, error) {
	// First, list message IDs.
	listURL := fmt.Sprintf("%s/messages?q=%s&maxResults=%d", gmailAPIBase, query, maxResults)
	if pageToken != "" {
		listURL += "&pageToken=" + pageToken
	}

	var listResp struct {
		Messages      []struct{ ID string `json:"id"` } `json:"messages"`
		NextPageToken string                             `json:"nextPageToken"`
	}
	if err := c.apiGet(ctx, accessToken, listURL, &listResp); err != nil {
		return nil, "", fmt.Errorf("list messages: %w", err)
	}

	// Fetch full details for each message.
	messages := make([]GmailMessage, 0, len(listResp.Messages))
	for _, m := range listResp.Messages {
		msg, err := c.GetMessageDetail(ctx, accessToken, m.ID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get message detail", "error", err, "message_id", m.ID)
			continue
		}
		messages = append(messages, *msg)
	}

	return messages, listResp.NextPageToken, nil
}

// GetMessageDetail fetches a full message with body from Gmail.
func (c *GmailSyncClient) GetMessageDetail(ctx context.Context, accessToken, messageID string) (*GmailMessage, error) {
	detailURL := fmt.Sprintf("%s/messages/%s?format=full", gmailAPIBase, messageID)

	var raw gmailRawMessage
	if err := c.apiGet(ctx, accessToken, detailURL, &raw); err != nil {
		return nil, fmt.Errorf("get message %s: %w", messageID, err)
	}

	return parseGmailMessage(&raw), nil
}

// SendMessage sends an email via Gmail API and returns the message ID.
func (c *GmailSyncClient) SendMessage(ctx context.Context, accessToken, from string, to []string, cc []string, subject, bodyHTML string) (string, error) {
	// Build RFC 2822 MIME message.
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + strings.Join(to, ", ") + "\r\n")
	if len(cc) > 0 {
		b.WriteString("Cc: " + strings.Join(cc, ", ") + "\r\n")
	}
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	b.WriteString("\r\n")
	b.WriteString(bodyHTML)

	// Base64url encode the message.
	encoded := base64.URLEncoding.EncodeToString([]byte(b.String()))

	sendURL := fmt.Sprintf("%s/messages/send", gmailAPIBase)
	payload := fmt.Sprintf(`{"raw":"%s"}`, encoded)

	req, err := http.NewRequestWithContext(ctx, "POST", sendURL, strings.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create send request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send message: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read send response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("send failed (status %d): %s", resp.StatusCode, string(body))
	}

	var sendResp struct {
		ID       string `json:"id"`
		ThreadID string `json:"threadId"`
	}
	if err := json.Unmarshal(body, &sendResp); err != nil {
		return "", fmt.Errorf("parse send response: %w", err)
	}

	return sendResp.ID, nil
}

// apiGet performs an authenticated GET request to the Gmail API.
func (c *GmailSyncClient) apiGet(ctx context.Context, accessToken, url string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("api request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	return nil
}

// ── Gmail API response types ──

type gmailRawMessage struct {
	ID        string `json:"id"`
	ThreadID  string `json:"threadId"`
	HistoryID string `json:"historyId"`
	Payload   struct {
		Headers []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"headers"`
		MimeType string           `json:"mimeType"`
		Body     gmailMessageBody `json:"body"`
		Parts    []gmailPart      `json:"parts"`
	} `json:"payload"`
	InternalDate string `json:"internalDate"`
}

type gmailMessageBody struct {
	Size int    `json:"size"`
	Data string `json:"data"`
}

type gmailPart struct {
	MimeType string           `json:"mimeType"`
	Body     gmailMessageBody `json:"body"`
	Parts    []gmailPart      `json:"parts"`
}

func parseGmailMessage(raw *gmailRawMessage) *GmailMessage {
	msg := &GmailMessage{
		ID:        raw.ID,
		ThreadID:  raw.ThreadID,
		HistoryID: raw.HistoryID,
	}

	// Parse headers.
	for _, h := range raw.Payload.Headers {
		switch strings.ToLower(h.Name) {
		case "subject":
			msg.Subject = h.Value
		case "from":
			addr, err := mail.ParseAddress(h.Value)
			if err == nil {
				msg.From = addr.Address
				msg.FromName = addr.Name
			} else {
				msg.From = h.Value
			}
		case "to":
			msg.To = parseAddressList(h.Value)
		case "cc":
			msg.CC = parseAddressList(h.Value)
		case "date":
			if t, err := mail.ParseDate(h.Value); err == nil {
				msg.Date = t
			}
		}
	}

	// Fallback date from internalDate (milliseconds since epoch).
	if msg.Date.IsZero() && raw.InternalDate != "" {
		var ms int64
		fmt.Sscanf(raw.InternalDate, "%d", &ms)
		if ms > 0 {
			msg.Date = time.Unix(ms/1000, 0)
		}
	}

	// Extract body.
	msg.BodyText, msg.BodyHTML = extractBody(raw.Payload.MimeType, raw.Payload.Body, raw.Payload.Parts)

	return msg
}

func extractBody(mimeType string, body gmailMessageBody, parts []gmailPart) (text, html string) {
	mediaType, _, _ := mime.ParseMediaType(mimeType)

	if mediaType == "text/plain" && body.Data != "" {
		decoded, err := base64.URLEncoding.DecodeString(body.Data)
		if err == nil {
			text = string(decoded)
		}
	}
	if mediaType == "text/html" && body.Data != "" {
		decoded, err := base64.URLEncoding.DecodeString(body.Data)
		if err == nil {
			html = string(decoded)
		}
	}

	for _, part := range parts {
		partText, partHTML := extractBody(part.MimeType, part.Body, part.Parts)
		if text == "" {
			text = partText
		}
		if html == "" {
			html = partHTML
		}
	}

	return text, html
}

func parseAddressList(value string) []string {
	addrs, err := mail.ParseAddressList(value)
	if err != nil {
		// Fallback: split by comma.
		parts := strings.Split(value, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				result = append(result, p)
			}
		}
		return result
	}
	result := make([]string, 0, len(addrs))
	for _, a := range addrs {
		result = append(result, a.Address)
	}
	return result
}
