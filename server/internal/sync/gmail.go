package sync

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/mail"
	"net/textproto"
	"net/url"
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
	ID               string
	ThreadID         string
	Subject          string
	From             string
	FromName         string
	To               []string
	ToNames          map[string]string // email → display name
	CC               []string
	CCNames          map[string]string // email → display name
	Date             time.Time
	BodyText         string
	BodyHTML         string
	HistoryID        string
	LabelIDs         []string
	RFCMessageID     string
	InReplyTo        string
	ReferencesHeader string
}

// GmailAttachment is an outgoing MIME attachment. Data is kept in memory only
// for the duration of the Gmail API request.
type GmailAttachment struct {
	FileName    string
	ContentType string
	Data        []byte
}

// GmailSendResult contains the identifiers returned by Gmail after sending.
type GmailSendResult struct {
	ID       string
	ThreadID string
}

// GmailProfile contains mailbox metadata used for sync bookkeeping.
type GmailProfile struct {
	EmailAddress string
	HistoryID    string
}

// GmailHistoryResult contains message IDs changed since a history cursor and
// the latest mailbox cursor returned by Gmail.
type GmailHistoryResult struct {
	MessageIDs      []string
	LatestHistoryID string
}

// GmailAPIError captures non-success Gmail API responses.
type GmailAPIError struct {
	StatusCode int
	Body       string
}

func (e *GmailAPIError) Error() string {
	return fmt.Sprintf("api error (status %d): %s", e.StatusCode, e.Body)
}

// GmailSyncClient wraps the Gmail REST API for email sync operations.
type GmailSyncClient struct {
	oauthClient        *oauth.GmailOAuthClient
	emailRepo          *repository.CRMEmailRepository
	encryptionKey      []byte
	httpClient         *http.Client
	apiBaseURL         string
	calendarAPIBaseURL string
}

// NewGmailSyncClient creates a new GmailSyncClient.
func NewGmailSyncClient(oauthClient *oauth.GmailOAuthClient, emailRepo *repository.CRMEmailRepository, encryptionKey []byte) *GmailSyncClient {
	if oauthClient == nil {
		return nil
	}
	return &GmailSyncClient{
		oauthClient:        oauthClient,
		emailRepo:          emailRepo,
		encryptionKey:      encryptionKey,
		httpClient:         &http.Client{Timeout: 60 * time.Second},
		apiBaseURL:         gmailAPIBase,
		calendarAPIBaseURL: googleCalendarAPIBase,
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

// GetEmailAddress fetches the authenticated user's email address from the Gmail API.
func (c *GmailSyncClient) GetEmailAddress(ctx context.Context, accessToken string) (string, error) {
	profile, err := c.GetMailboxProfile(ctx, accessToken)
	if err != nil {
		return "", err
	}
	return profile.EmailAddress, nil
}

// GetMailboxProfile fetches the authenticated mailbox profile, including the
// current Gmail history cursor.
func (c *GmailSyncClient) GetMailboxProfile(ctx context.Context, accessToken string) (*GmailProfile, error) {
	var profile struct {
		EmailAddress string `json:"emailAddress"`
		HistoryID    string `json:"historyId"`
	}
	if err := c.apiGet(ctx, accessToken, c.userBaseURL()+"/profile", &profile); err != nil {
		return nil, fmt.Errorf("get gmail profile: %w", err)
	}
	return &GmailProfile{
		EmailAddress: profile.EmailAddress,
		HistoryID:    profile.HistoryID,
	}, nil
}

// ListMessages fetches messages from Gmail matching the query.
func (c *GmailSyncClient) ListMessages(ctx context.Context, accessToken, query string, maxResults int, pageToken string) ([]GmailMessage, string, error) {
	// First, list message IDs.
	params := url.Values{}
	params.Set("q", query)
	params.Set("maxResults", fmt.Sprintf("%d", maxResults))
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}
	listURL := c.userBaseURL() + "/messages?" + params.Encode()

	var listResp struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
		NextPageToken string `json:"nextPageToken"`
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
	detailURL := fmt.Sprintf("%s/messages/%s?format=full", c.userBaseURL(), messageID)

	var raw gmailRawMessage
	if err := c.apiGet(ctx, accessToken, detailURL, &raw); err != nil {
		return nil, fmt.Errorf("get message %s: %w", messageID, err)
	}

	return parseGmailMessage(&raw), nil
}

// SendMessage sends an email via Gmail API and returns the created message and thread IDs.
func (c *GmailSyncClient) SendMessage(ctx context.Context, accessToken, from string, to []string, cc []string, subject, bodyHTML string) (*GmailSendResult, error) {
	return c.sendMessage(ctx, accessToken, from, to, cc, subject, bodyHTML, "", "", "", nil)
}

func (c *GmailSyncClient) SendMessageWithAttachments(ctx context.Context, accessToken, from string, to []string, cc []string, subject, bodyHTML string, attachments []GmailAttachment) (*GmailSendResult, error) {
	return c.sendMessage(ctx, accessToken, from, to, cc, subject, bodyHTML, "", "", "", attachments)
}

// SendMessageWithMessageID preserves a durable host action identity for reconciliation.
// Gmail does not promise deduplicated sends: callers must claim the intent once.
func (c *GmailSyncClient) SendMessageWithMessageID(ctx context.Context, accessToken, from string, to, cc []string, subject, bodyHTML, messageID string) (*GmailSendResult, error) {
	if !validOutgoingMessageID(messageID) {
		return nil, fmt.Errorf("invalid outgoing message identity")
	}
	return c.sendMessage(ctx, accessToken, from, to, cc, subject, bodyHTML, "", "", "", nil, messageID)
}

// FindSentMessageByMessageID is read-only. Absence is inconclusive, never permission to resend.
func (c *GmailSyncClient) FindSentMessageByMessageID(ctx context.Context, accessToken, messageID string) (*GmailMessage, error) {
	if !validOutgoingMessageID(messageID) {
		return nil, fmt.Errorf("invalid outgoing message identity")
	}
	// The Gmail API documents rfc822msgid queries on users.messages.list.
	// https://developers.google.com/workspace/gmail/api/reference/rest/v1/users.messages/list
	messages, next, err := c.ListMessages(ctx, accessToken, "in:sent rfc822msgid:"+messageID, 2, "")
	if err != nil {
		return nil, err
	}
	if next != "" || len(messages) > 1 {
		return nil, fmt.Errorf("multiple sent messages match this action; inspection required")
	}
	if len(messages) == 1 && messages[0].RFCMessageID == messageID {
		return &messages[0], nil
	}
	return nil, nil
}

func validOutgoingMessageID(value string) bool {
	if len(value) < 5 || len(value) > 254 || !strings.HasPrefix(value, "<") || !strings.HasSuffix(value, ">") || strings.ContainsAny(value, "\r\n\t ") {
		return false
	}
	inner := value[1 : len(value)-1]
	parsed, err := mail.ParseAddress(inner)
	return err == nil && parsed.Address == inner
}

// SendThreadMessage sends a reply into an existing Gmail thread.
func (c *GmailSyncClient) SendThreadMessage(ctx context.Context, accessToken, from string, to []string, cc []string, subject, bodyHTML, threadID, inReplyTo, references string) (*GmailSendResult, error) {
	return c.sendMessage(ctx, accessToken, from, to, cc, subject, bodyHTML, threadID, inReplyTo, references, nil)
}

func (c *GmailSyncClient) SendThreadMessageWithAttachments(ctx context.Context, accessToken, from string, to []string, cc []string, subject, bodyHTML, threadID, inReplyTo, references string, attachments []GmailAttachment) (*GmailSendResult, error) {
	return c.sendMessage(ctx, accessToken, from, to, cc, subject, bodyHTML, threadID, inReplyTo, references, attachments)
}

func (c *GmailSyncClient) sendMessage(ctx context.Context, accessToken, from string, to []string, cc []string, subject, bodyHTML, threadID, inReplyTo, references string, attachments []GmailAttachment, messageIDs ...string) (*GmailSendResult, error) {
	if len(messageIDs) > 1 || len(messageIDs) == 1 && !validOutgoingMessageID(messageIDs[0]) {
		return nil, fmt.Errorf("invalid outgoing message identity")
	}
	fromHeader, err := safeMailHeaderAddress(from)
	if err != nil {
		return nil, fmt.Errorf("invalid from address: %w", err)
	}
	toHeaders, err := safeMailHeaderAddresses(to)
	if err != nil || len(toHeaders) == 0 {
		return nil, fmt.Errorf("at least one valid recipient is required")
	}
	ccHeaders, err := safeMailHeaderAddresses(cc)
	if err != nil {
		return nil, fmt.Errorf("invalid cc recipient: %w", err)
	}
	if strings.ContainsAny(subject, "\r\n") {
		return nil, fmt.Errorf("subject contains invalid line breaks")
	}

	// Build RFC 2822 MIME message.
	var b strings.Builder
	b.WriteString("From: " + fromHeader + "\r\n")
	b.WriteString("To: " + strings.Join(toHeaders, ", ") + "\r\n")
	if len(ccHeaders) > 0 {
		b.WriteString("Cc: " + strings.Join(ccHeaders, ", ") + "\r\n")
	}
	b.WriteString("Subject: " + mime.QEncoding.Encode("UTF-8", subject) + "\r\n")
	if len(messageIDs) == 1 {
		b.WriteString("Message-ID: " + messageIDs[0] + "\r\n")
	}
	if strings.TrimSpace(inReplyTo) != "" {
		b.WriteString("In-Reply-To: " + sanitizeMessageHeader(inReplyTo) + "\r\n")
	}
	if strings.TrimSpace(references) != "" {
		b.WriteString("References: " + sanitizeMessageHeader(references) + "\r\n")
	}
	b.WriteString("MIME-Version: 1.0\r\n")
	if len(attachments) == 0 {
		b.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
		b.WriteString("\r\n")
		b.WriteString(bodyHTML)
	} else {
		var mimeBody bytes.Buffer
		writer := multipart.NewWriter(&mimeBody)
		b.WriteString("Content-Type: multipart/mixed; boundary=\"" + writer.Boundary() + "\"\r\n\r\n")
		htmlHeader := textproto.MIMEHeader{}
		htmlHeader.Set("Content-Type", "text/html; charset=UTF-8")
		htmlHeader.Set("Content-Transfer-Encoding", "8bit")
		htmlPart, partErr := writer.CreatePart(htmlHeader)
		if partErr != nil {
			return nil, fmt.Errorf("create HTML MIME part: %w", partErr)
		}
		if _, partErr = io.WriteString(htmlPart, bodyHTML); partErr != nil {
			return nil, fmt.Errorf("write HTML MIME part: %w", partErr)
		}
		for _, attachment := range attachments {
			contentType := strings.TrimSpace(attachment.ContentType)
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			params := map[string]string{"name": attachment.FileName}
			header := textproto.MIMEHeader{}
			header.Set("Content-Type", mime.FormatMediaType(contentType, params))
			header.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": attachment.FileName}))
			header.Set("Content-Transfer-Encoding", "base64")
			part, partErr := writer.CreatePart(header)
			if partErr != nil {
				return nil, fmt.Errorf("create attachment MIME part: %w", partErr)
			}
			encodedAttachment := base64.StdEncoding.EncodeToString(attachment.Data)
			for len(encodedAttachment) > 76 {
				if _, partErr = io.WriteString(part, encodedAttachment[:76]+"\r\n"); partErr != nil {
					return nil, partErr
				}
				encodedAttachment = encodedAttachment[76:]
			}
			if _, partErr = io.WriteString(part, encodedAttachment); partErr != nil {
				return nil, partErr
			}
		}
		if err := writer.Close(); err != nil {
			return nil, fmt.Errorf("close MIME message: %w", err)
		}
		b.Write(mimeBody.Bytes())
	}

	// Base64url encode the message.
	encoded := base64.URLEncoding.EncodeToString([]byte(b.String()))

	sendURL := fmt.Sprintf("%s/messages/send", c.userBaseURL())
	payloadData := map[string]string{"raw": encoded}
	if strings.TrimSpace(threadID) != "" {
		payloadData["threadId"] = strings.TrimSpace(threadID)
	}
	payload, err := json.Marshal(payloadData)
	if err != nil {
		return nil, fmt.Errorf("encode send request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", sendURL, strings.NewReader(string(payload)))
	if err != nil {
		return nil, fmt.Errorf("create send request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send message: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read send response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("send failed (status %d): %s", resp.StatusCode, string(body))
	}

	var sendResp struct {
		ID       string `json:"id"`
		ThreadID string `json:"threadId"`
	}
	if err := json.Unmarshal(body, &sendResp); err != nil {
		return nil, fmt.Errorf("parse send response: %w", err)
	}

	return &GmailSendResult{
		ID:       sendResp.ID,
		ThreadID: sendResp.ThreadID,
	}, nil
}

func sanitizeMessageHeader(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(value), "\r", ""), "\n", "")
}

func safeMailHeaderAddresses(addresses []string) ([]string, error) {
	result := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if strings.TrimSpace(address) == "" {
			continue
		}
		header, err := safeMailHeaderAddress(address)
		if err != nil {
			return nil, err
		}
		result = append(result, header)
	}
	return result, nil
}

func safeMailHeaderAddress(value string) (string, error) {
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("address contains invalid line breaks")
	}
	address, err := mail.ParseAddress(strings.TrimSpace(value))
	if err != nil || address.Address == "" {
		return "", fmt.Errorf("invalid email address")
	}
	return address.String(), nil
}

// ListHistory returns unique message IDs changed since the provided Gmail
// history cursor, along with the newest cursor returned by Gmail.
func (c *GmailSyncClient) ListHistory(ctx context.Context, accessToken, startHistoryID string) (*GmailHistoryResult, error) {
	params := url.Values{}
	params.Set("startHistoryId", startHistoryID)
	params.Set("maxResults", "500")

	nextPageToken := ""
	seen := map[string]struct{}{}
	messageIDs := make([]string, 0, 64)
	latestHistoryID := startHistoryID

	for {
		if nextPageToken != "" {
			params.Set("pageToken", nextPageToken)
		} else {
			params.Del("pageToken")
		}

		historyURL := c.userBaseURL() + "/history?" + params.Encode()
		var resp struct {
			HistoryID     string `json:"historyId"`
			NextPageToken string `json:"nextPageToken"`
			History       []struct {
				Messages []struct {
					ID string `json:"id"`
				} `json:"messages"`
				MessagesAdded []struct {
					Message struct {
						ID string `json:"id"`
					} `json:"message"`
				} `json:"messagesAdded"`
				LabelsAdded []struct {
					Message struct {
						ID string `json:"id"`
					} `json:"message"`
				} `json:"labelsAdded"`
				LabelsRemoved []struct {
					Message struct {
						ID string `json:"id"`
					} `json:"message"`
				} `json:"labelsRemoved"`
			} `json:"history"`
		}
		if err := c.apiGet(ctx, accessToken, historyURL, &resp); err != nil {
			return nil, fmt.Errorf("list history: %w", err)
		}

		if resp.HistoryID != "" {
			latestHistoryID = resp.HistoryID
		}
		for _, item := range resp.History {
			for _, message := range item.Messages {
				appendHistoryMessageID(message.ID, seen, &messageIDs)
			}
			for _, entry := range item.MessagesAdded {
				appendHistoryMessageID(entry.Message.ID, seen, &messageIDs)
			}
			for _, entry := range item.LabelsAdded {
				appendHistoryMessageID(entry.Message.ID, seen, &messageIDs)
			}
			for _, entry := range item.LabelsRemoved {
				appendHistoryMessageID(entry.Message.ID, seen, &messageIDs)
			}
		}

		if resp.NextPageToken == "" {
			break
		}
		nextPageToken = resp.NextPageToken
	}

	return &GmailHistoryResult{
		MessageIDs:      messageIDs,
		LatestHistoryID: latestHistoryID,
	}, nil
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
		return &GmailAPIError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	return nil
}

func (c *GmailSyncClient) userBaseURL() string {
	if c.apiBaseURL != "" {
		return c.apiBaseURL
	}
	return gmailAPIBase
}

func appendHistoryMessageID(id string, seen map[string]struct{}, target *[]string) {
	if id == "" {
		return
	}
	if _, ok := seen[id]; ok {
		return
	}
	seen[id] = struct{}{}
	*target = append(*target, id)
}

// ── Gmail API response types ──

type gmailRawMessage struct {
	ID        string   `json:"id"`
	ThreadID  string   `json:"threadId"`
	HistoryID string   `json:"historyId"`
	LabelIDs  []string `json:"labelIds"`
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
		LabelIDs:  append([]string(nil), raw.LabelIDs...),
	}

	// Parse headers.
	for _, h := range raw.Payload.Headers {
		switch strings.ToLower(h.Name) {
		case "subject":
			msg.Subject = h.Value
		case "from":
			addr, err := mail.ParseAddress(h.Value)
			if err == nil {
				msg.From = strings.ToLower(strings.TrimSpace(addr.Address))
				msg.FromName = addr.Name
			} else {
				fromAddrs, fromNames := parseAddressListWithNames(h.Value)
				if len(fromAddrs) > 0 {
					msg.From = fromAddrs[0]
					msg.FromName = fromNames[strings.ToLower(fromAddrs[0])]
				}
			}
		case "to":
			msg.To, msg.ToNames = parseAddressListWithNames(h.Value)
		case "cc":
			msg.CC, msg.CCNames = parseAddressListWithNames(h.Value)
		case "date":
			if t, err := mail.ParseDate(h.Value); err == nil {
				msg.Date = t
			}
		case "message-id":
			msg.RFCMessageID = strings.TrimSpace(h.Value)
		case "in-reply-to":
			msg.InReplyTo = strings.TrimSpace(h.Value)
		case "references":
			msg.ReferencesHeader = strings.TrimSpace(h.Value)
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
	addrs, _ := parseAddressListWithNames(value)
	return addrs
}

func parseAddressListWithNames(value string) ([]string, map[string]string) {
	names := make(map[string]string)
	addrs, err := mail.ParseAddressList(value)
	if err != nil {
		// Fallback: parse each comma-separated chunk independently and drop invalid values.
		parts := strings.Split(value, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			addr, parseErr := mail.ParseAddress(p)
			if parseErr != nil {
				continue
			}
			email := strings.ToLower(strings.TrimSpace(addr.Address))
			if email == "" {
				continue
			}
			result = append(result, email)
			if addr.Name != "" {
				names[email] = addr.Name
			}
		}
		return result, names
	}
	result := make([]string, 0, len(addrs))
	for _, a := range addrs {
		result = append(result, a.Address)
		if a.Name != "" {
			names[strings.ToLower(a.Address)] = a.Name
		}
	}
	return result, names
}
