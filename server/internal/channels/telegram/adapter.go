package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/channels"
)

// Provider is the provider name stored on accounts.
const Provider = "telegram"

// Bot API limits: https://core.telegram.org/bots/api
const (
	_maxText     = 4096
	_maxCaption  = 1024
	_maxDownload = 20 << 20
	_maxUpload   = 50 << 20
)

// SecretHeader carries the webhook secret registered with setWebhook.
const SecretHeader = "X-Telegram-Bot-Api-Secret-Token"

// Adapter implements channels.Adapter for Telegram bots.
type Adapter struct {
	api *client
}

var _ channels.Adapter = (*Adapter)(nil)

// NewAdapter talks to the Bot API at baseURL (empty means the public API).
func NewAdapter(baseURL string) *Adapter {
	return &Adapter{api: newClient(baseURL)}
}

// Provider implements channels.Adapter.
func (a *Adapter) Provider() string { return Provider }

// Capabilities implements channels.Adapter.
func (a *Adapter) Capabilities() channels.Capabilities {
	return channels.Capabilities{MaxTextLength: _maxText, MaxCaptionLength: _maxCaption, MaxDownloadSize: _maxDownload, MaxUploadSize: _maxUpload}
}

// ValidateToken implements channels.Adapter.
func (a *Adapter) ValidateToken(ctx context.Context, token string) (channels.AccountInfo, error) {
	me, err := a.api.getMe(ctx, token)
	if err != nil {
		if result := classify(err); result.Code == channels.CodeAuth {
			return channels.AccountInfo{}, channels.ErrAuth
		}
		return channels.AccountInfo{}, fmt.Errorf("validate telegram bot: %w", err)
	}
	return channels.AccountInfo{ExternalAccountID: strconv.FormatInt(me.ID, 10), DisplayName: me.FirstName, Handle: me.Username}, nil
}

// RegisterWebhook implements channels.Adapter.
func (a *Adapter) RegisterWebhook(ctx context.Context, token, webhookURL, secret string) error {
	if err := a.api.setWebhook(ctx, token, webhookURL, secret); err != nil {
		return fmt.Errorf("register telegram webhook: %w", err)
	}
	return nil
}

// UnregisterWebhook implements channels.Adapter.
func (a *Adapter) UnregisterWebhook(ctx context.Context, token string) error {
	if err := a.api.deleteWebhook(ctx, token); err != nil {
		return fmt.Errorf("unregister telegram webhook: %w", err)
	}
	return nil
}

// PresentedSecret implements channels.Adapter.
func (a *Adapter) PresentedSecret(header http.Header) string { return header.Get(SecretHeader) }

type update struct {
	UpdateID int64    `json:"update_id"`
	Message  *message `json:"message"`
}

type user struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

type fileRef struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
	MimeType string `json:"mime_type"`
	FileSize int64  `json:"file_size"`
}

type message struct {
	MessageID int64 `json:"message_id"`
	From      *user `json:"from"`
	Chat      struct {
		ID   int64  `json:"id"`
		Type string `json:"type"`
	} `json:"chat"`
	Date            int64     `json:"date"`
	Text            string    `json:"text"`
	Entities        []entity  `json:"entities"`
	Caption         string    `json:"caption"`
	CaptionEntities []entity  `json:"caption_entities"`
	Photo           []fileRef `json:"photo"`
	Document        *fileRef  `json:"document"`
	Video           *fileRef  `json:"video"`
	Audio           *fileRef  `json:"audio"`
	Voice           *fileRef  `json:"voice"`
	VideoNote       *fileRef  `json:"video_note"`
	Animation       *fileRef  `json:"animation"`
}

// _unsupportedFields are message types Helpin cannot display, named for the
// teammate note.
var _unsupportedFields = []struct{ field, label string }{
	{"sticker", "sticker"}, {"location", "location"}, {"venue", "venue"}, {"contact", "contact card"},
	{"poll", "poll"}, {"dice", "dice"}, {"game", "game"}, {"story", "story"}, {"invoice", "invoice"},
}

// ParseWebhook implements channels.Adapter. Telegram sends one update per
// request.
func (a *Adapter) ParseWebhook(body []byte) ([]channels.Event, error) {
	var u update
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, fmt.Errorf("parse telegram update: %w", err)
	}
	if u.UpdateID == 0 {
		return nil, errors.New("parse telegram update: missing update_id")
	}
	event := channels.Event{EventKey: strconv.FormatInt(u.UpdateID, 10), Kind: channels.EventIgnored}
	m := u.Message
	switch {
	case m == nil:
		event.IgnoredReason = "not a new message"
		return []channels.Event{event}, nil
	case m.Chat.Type != "private":
		event.IgnoredReason = "not a private chat"
		return []channels.Event{event}, nil
	case m.From == nil || m.From.IsBot:
		event.IgnoredReason = "not from a person"
		return []channels.Event{event}, nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &struct {
		Message *map[string]json.RawMessage `json:"message"`
	}{Message: &raw}); err != nil {
		return nil, fmt.Errorf("parse telegram message: %w", err)
	}

	chatID := strconv.FormatInt(m.Chat.ID, 10)
	event.ExternalUserID = strconv.FormatInt(m.From.ID, 10)
	event.ExternalChatID = chatID
	event.ExternalMessageKey = chatID + ":" + strconv.FormatInt(m.MessageID, 10)
	event.DisplayName = strings.TrimSpace(m.From.FirstName + " " + m.From.LastName)
	event.Username = m.From.Username
	event.SentAt = time.Unix(m.Date, 0).UTC()
	if m.Text != "" {
		event.Text = entitiesToMarkdown(m.Text, m.Entities)
	} else {
		event.Text = entitiesToMarkdown(m.Caption, m.CaptionEntities)
	}
	event.Files = messageFiles(m)
	if event.Text != "" || len(event.Files) > 0 {
		event.Kind = channels.EventMessage
		return []channels.Event{event}, nil
	}
	for _, candidate := range _unsupportedFields {
		if _, ok := raw[candidate.field]; ok {
			event.Kind = channels.EventUnsupported
			event.UnsupportedType = candidate.label
			return []channels.Event{event}, nil
		}
	}
	event.Kind = channels.EventUnsupported
	event.UnsupportedType = "message"
	return []channels.Event{event}, nil
}

// messageFiles lists the downloadable files. Animations also carry a document
// copy for older clients, so only one is kept.
func messageFiles(m *message) []channels.InboundFile {
	var files []channels.InboundFile
	add := func(ref *fileRef, fallbackName, fallbackType string) {
		if ref == nil || ref.FileID == "" {
			return
		}
		name := strings.TrimSpace(ref.FileName)
		if name == "" {
			name = fallbackName
		}
		contentType := strings.TrimSpace(ref.MimeType)
		if contentType == "" {
			contentType = fallbackType
		}
		files = append(files, channels.InboundFile{FileID: ref.FileID, FileName: name, ContentType: contentType, Size: ref.FileSize})
	}
	if len(m.Photo) > 0 {
		largest := m.Photo[len(m.Photo)-1]
		add(&largest, "photo.jpg", "image/jpeg")
	}
	switch {
	case m.Animation != nil:
		add(m.Animation, "animation.mp4", "video/mp4")
	case m.Document != nil:
		add(m.Document, "document", "application/octet-stream")
	}
	add(m.Video, "video.mp4", "video/mp4")
	add(m.Audio, "audio.mp3", "audio/mpeg")
	add(m.Voice, "voice.ogg", "audio/ogg")
	add(m.VideoNote, "video-note.mp4", "video/mp4")
	return files
}

// Render implements channels.Adapter: text is split to fit, and each file is
// its own part with the first text as its caption when it fits.
func (a *Adapter) Render(msg channels.Outgoing) []channels.Part {
	texts := splitMarkdown(msg.Text, _maxText)
	var parts []channels.Part
	for i, file := range msg.Files {
		file := file
		part := channels.Part{File: &file}
		if i == 0 && len(texts) > 0 && utf16Len(markdownToPlain(texts[0])) <= _maxCaption {
			part.Text, part.PlainText = markdownToTelegramHTML(texts[0]), markdownToPlain(texts[0])
			texts = texts[1:]
		}
		parts = append(parts, part)
	}
	for _, text := range texts {
		parts = append(parts, channels.Part{Text: markdownToTelegramHTML(text), PlainText: markdownToPlain(text)})
	}
	return parts
}

// SendPart implements channels.Adapter. plain sends PlainText without parse
// mode, for the retry after a formatting rejection.
func (a *Adapter) SendPart(ctx context.Context, token, chatID string, part channels.Part, plain bool) channels.SendResult {
	text, html := part.Text, true
	if plain {
		text, html = part.PlainText, false
	}
	var sent sentMessage
	var err error
	if part.File != nil {
		if int64(len(part.File.Data)) > _maxUpload {
			return channels.SendResult{Outcome: channels.Rejected, Code: channels.CodeTooLarge, Message: "file exceeds Telegram's upload limit"}
		}
		sent, err = a.api.sendFile(ctx, token, chatID, *part.File, text, html)
	} else {
		sent, err = a.api.sendMessage(ctx, token, chatID, text, html)
	}
	if err != nil {
		return classify(err)
	}
	return channels.SendResult{Outcome: channels.Accepted, ExternalKey: chatID + ":" + strconv.FormatInt(sent.MessageID, 10)}
}

// DownloadFile implements channels.Adapter.
func (a *Adapter) DownloadFile(ctx context.Context, token, fileID string, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 || maxBytes > _maxDownload {
		maxBytes = _maxDownload
	}
	data, err := a.api.download(ctx, token, fileID, maxBytes)
	if err != nil {
		return nil, fmt.Errorf("download telegram file: %w", err)
	}
	return data, nil
}

func isPhoto(contentType string) bool {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg", "image/png", "image/webp":
		return true
	}
	return false
}

func sanitizeFileName(name string) string {
	name = path.Base(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == '"' {
			return -1
		}
		return r
	}, name)
	if name == "" || name == "." || name == "/" {
		return "file"
	}
	return name
}
