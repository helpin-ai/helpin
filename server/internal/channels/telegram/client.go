// Package telegram implements the Telegram Bot API channel adapter.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptrace"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/helpin-ai/helpin/server/internal/channels"
	"github.com/helpin-ai/helpin/server/internal/netguard"
)

// DefaultAPIBaseURL is the public Bot API.
const DefaultAPIBaseURL = "https://api.telegram.org"

const _requestTimeout = 30 * time.Second

// client calls the Bot API. Tokens appear only in request paths and are
// scrubbed from every error.
type client struct {
	baseURL string
	http    *http.Client
}

// newClient talks to baseURL. A non-default base URL (a local Bot API server
// or a test fake) may be on a private network; the public API is reached
// through the netguard dialer.
func newClient(baseURL string) *client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = DefaultAPIBaseURL
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = netguard.DialContext(baseURL != DefaultAPIBaseURL)
	return &client{baseURL: baseURL, http: &http.Client{Timeout: _requestTimeout, Transport: transport}}
}

// apiError is a Bot API failure response.
type apiError struct {
	status      int
	code        int
	description string
	retryAfter  time.Duration
}

func (e *apiError) Error() string {
	return fmt.Sprintf("telegram api %d: %s", e.code, e.description)
}

// requestError wraps a transport failure and records whether the request
// reached the wire, which decides whether a retry is safe.
type requestError struct {
	err     error
	written bool
}

func (e *requestError) Error() string { return e.err.Error() }
func (e *requestError) Unwrap() error { return e.err }

type envelope struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	ErrorCode   int             `json:"error_code"`
	Description string          `json:"description"`
	Parameters  struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters"`
}

// call posts a JSON or multipart body to a Bot API method and decodes result.
func (c *client) call(ctx context.Context, token, method, contentType string, body []byte, result any) error {
	ctx, cancel := context.WithTimeout(ctx, _requestTimeout)
	defer cancel()
	var written atomic.Bool
	ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		WroteRequest: func(httptrace.WroteRequestInfo) { written.Store(true) },
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/bot"+token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return scrub(err, token)
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := c.http.Do(req)
	if err != nil {
		return &requestError{err: scrub(err, token), written: written.Load()}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return &requestError{err: scrub(err, token), written: true}
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return &apiError{status: resp.StatusCode, code: resp.StatusCode, description: "unreadable response"}
	}
	if !env.OK {
		code := env.ErrorCode
		if code == 0 {
			code = resp.StatusCode
		}
		return &apiError{status: resp.StatusCode, code: code, description: env.Description, retryAfter: time.Duration(env.Parameters.RetryAfter) * time.Second}
	}
	if result == nil {
		return nil
	}
	if err := json.Unmarshal(env.Result, result); err != nil {
		return fmt.Errorf("decode telegram %s result: %w", method, err)
	}
	return nil
}

func (c *client) callJSON(ctx context.Context, token, method string, params any, result any) error {
	body, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.call(ctx, token, method, "application/json", body, result)
}

type botUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

func (c *client) getMe(ctx context.Context, token string) (botUser, error) {
	var me botUser
	err := c.callJSON(ctx, token, "getMe", struct{}{}, &me)
	return me, err
}

func (c *client) setWebhook(ctx context.Context, token, webhookURL, secret string) error {
	return c.callJSON(ctx, token, "setWebhook", map[string]any{
		"url":             webhookURL,
		"secret_token":    secret,
		"allowed_updates": []string{"message"},
	}, nil)
}

func (c *client) deleteWebhook(ctx context.Context, token string) error {
	return c.callJSON(ctx, token, "deleteWebhook", struct{}{}, nil)
}

type sentMessage struct {
	MessageID int64 `json:"message_id"`
	Chat      struct {
		ID int64 `json:"id"`
	} `json:"chat"`
}

func (c *client) sendMessage(ctx context.Context, token, chatID, text string, html bool) (sentMessage, error) {
	params := map[string]any{"chat_id": chatID, "text": text}
	if html {
		params["parse_mode"] = "HTML"
	}
	var sent sentMessage
	err := c.callJSON(ctx, token, "sendMessage", params, &sent)
	return sent, err
}

// sendFile uploads one photo or document with an optional caption.
func (c *client) sendFile(ctx context.Context, token, chatID string, file channels.OutgoingFile, caption string, html bool) (sentMessage, error) {
	method, field := "sendDocument", "document"
	if isPhoto(file.ContentType) {
		method, field = "sendPhoto", "photo"
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	_ = form.WriteField("chat_id", chatID)
	if caption != "" {
		_ = form.WriteField("caption", caption)
		if html {
			_ = form.WriteField("parse_mode", "HTML")
		}
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, field, sanitizeFileName(file.Name)))
	header.Set("Content-Type", file.ContentType)
	part, err := form.CreatePart(header)
	if err != nil {
		return sentMessage{}, err
	}
	if _, err := part.Write(file.Data); err != nil {
		return sentMessage{}, err
	}
	if err := form.Close(); err != nil {
		return sentMessage{}, err
	}
	var sent sentMessage
	err = c.call(ctx, token, method, form.FormDataContentType(), body.Bytes(), &sent)
	return sent, err
}

// download fetches a file the customer sent, refusing anything over maxBytes.
func (c *client) download(ctx context.Context, token, fileID string, maxBytes int64) ([]byte, error) {
	var file struct {
		FilePath string `json:"file_path"`
		FileSize int64  `json:"file_size"`
	}
	if err := c.callJSON(ctx, token, "getFile", map[string]string{"file_id": fileID}, &file); err != nil {
		return nil, err
	}
	if file.FileSize > maxBytes {
		return nil, ErrFileTooLarge
	}
	if file.FilePath == "" {
		return nil, errors.New("telegram returned no file path")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*_requestTimeout)
	defer cancel()
	fileURL := c.baseURL + "/file/bot" + token + "/" + escapePath(file.FilePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, scrub(err, token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, scrub(err, token)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram file download: status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, scrub(err, token)
	}
	if int64(len(data)) > maxBytes {
		return nil, ErrFileTooLarge
	}
	return data, nil
}

// ErrFileTooLarge means a customer file exceeds what bots may download.
var ErrFileTooLarge = errors.New("file is larger than the provider allows")

func escapePath(path string) string {
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}

// scrub removes the bot token from an error, since net/http errors include
// the request URL.
func scrub(err error, token string) error {
	if err == nil || token == "" {
		return err
	}
	return errors.New(strings.ReplaceAll(err.Error(), token, "<token>"))
}

// classify maps a send error to what it proves about delivery.
func classify(err error) channels.SendResult {
	var api *apiError
	if errors.As(err, &api) {
		message := truncate(api.description, 300)
		switch {
		case api.code == http.StatusTooManyRequests:
			return channels.SendResult{Outcome: channels.NotSent, Code: "rate_limited", Message: message, RetryAfter: api.retryAfter}
		case api.code == http.StatusUnauthorized || api.code == http.StatusNotFound:
			return channels.SendResult{Outcome: channels.Rejected, Code: channels.CodeAuth, Message: message}
		case api.code == http.StatusForbidden:
			return channels.SendResult{Outcome: channels.Rejected, Code: channels.CodeBlocked, Message: message}
		case api.code == http.StatusBadRequest && strings.Contains(strings.ToLower(api.description), "can't parse entities"):
			return channels.SendResult{Outcome: channels.Rejected, Code: channels.CodeParse, Message: message}
		case api.code == http.StatusRequestEntityTooLarge:
			return channels.SendResult{Outcome: channels.Rejected, Code: channels.CodeTooLarge, Message: message}
		case api.code >= 400 && api.code < 500:
			return channels.SendResult{Outcome: channels.Rejected, Code: channels.CodeBadRequest, Message: message}
		default:
			return channels.SendResult{Outcome: channels.Unknown, Code: "provider_error_" + strconv.Itoa(api.code), Message: message}
		}
	}
	var req *requestError
	if errors.As(err, &req) && !req.written {
		return channels.SendResult{Outcome: channels.NotSent, Code: "network", Message: truncate(req.Error(), 300)}
	}
	return channels.SendResult{Outcome: channels.Unknown, Code: "network", Message: truncate(err.Error(), 300)}
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
