// Package channels is the provider boundary for customer messaging channels
// (Telegram first). Adapters turn provider webhooks into normalised events and
// send rendered replies; services and repositories never branch on provider
// payloads.
package channels

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// ErrAuth means the provider rejected the account's credentials.
var ErrAuth = errors.New("channel credentials rejected")

// EventKind classifies a normalised webhook event.
type EventKind string

const (
	// EventMessage is a customer message to store.
	EventMessage EventKind = "message"
	// EventUnsupported is a customer message type Helpin cannot display; it is
	// stored as a note so the teammate knows something arrived.
	EventUnsupported EventKind = "unsupported"
	// EventIgnored is anything else (groups, bots, edits); it is acknowledged
	// and dropped.
	EventIgnored EventKind = "ignored"
)

// Event is one provider update, normalised.
type Event struct {
	Kind EventKind
	// EventKey identifies the provider update for webhook dedupe.
	EventKey string
	// ExternalUserID identifies the customer within the connected account.
	ExternalUserID string
	// ExternalChatID is where replies go.
	ExternalChatID string
	// ExternalMessageKey identifies the message within the connected account.
	ExternalMessageKey string
	DisplayName        string
	Username           string
	// Text is markdown.
	Text            string
	Files           []InboundFile
	UnsupportedType string
	SentAt          time.Time
	// IgnoredReason explains EventIgnored for logs.
	IgnoredReason string
}

// InboundFile references a customer file to download later.
type InboundFile struct {
	FileID      string
	FileName    string
	ContentType string
	Size        int64
}

// AccountInfo describes a validated account.
type AccountInfo struct {
	ExternalAccountID string
	DisplayName       string
	Handle            string
}

// OutgoingFile is an attachment to send.
type OutgoingFile struct {
	Name        string
	ContentType string
	Data        []byte
}

// Outgoing is a reply to deliver.
type Outgoing struct {
	// Text is markdown.
	Text  string
	Files []OutgoingFile
}

// Part is one provider request. A reply becomes several parts when it is
// longer than the provider allows or carries files.
type Part struct {
	// Text is the provider's rich format; PlainText is the fallback when the
	// provider rejects the formatting.
	Text      string
	PlainText string
	File      *OutgoingFile
}

// Outcome is what a send attempt proves about delivery.
type Outcome string

const (
	// Accepted means the provider took the message.
	Accepted Outcome = "accepted"
	// NotSent means the request provably did not execute; retrying is safe.
	NotSent Outcome = "not_sent"
	// Rejected means a definite refusal; retrying will not help.
	Rejected Outcome = "rejected"
	// Unknown means the provider may have taken the message; retrying could
	// deliver it twice.
	Unknown Outcome = "unknown"
)

// Error codes adapters report with Rejected.
const (
	CodeAuth       = "auth"
	CodeBlocked    = "blocked"
	CodeBadRequest = "bad_request"
	CodeParse      = "parse_error"
	CodeTooLarge   = "too_large"
)

// SendResult reports one part's send attempt.
type SendResult struct {
	Outcome     Outcome
	ExternalKey string
	Code        string
	Message     string
	RetryAfter  time.Duration
}

// Capabilities are a provider's limits, used by rendering and the composer.
type Capabilities struct {
	MaxTextLength    int
	MaxCaptionLength int
	MaxDownloadSize  int64
	MaxUploadSize    int64
}

// Adapter is one provider's implementation.
type Adapter interface {
	Provider() string
	Capabilities() Capabilities
	ValidateToken(ctx context.Context, token string) (AccountInfo, error)
	RegisterWebhook(ctx context.Context, token, url, secret string) error
	UnregisterWebhook(ctx context.Context, token string) error
	// PresentedSecret returns the webhook secret the request carries.
	PresentedSecret(header http.Header) string
	ParseWebhook(body []byte) ([]Event, error)
	Render(msg Outgoing) []Part
	SendPart(ctx context.Context, token, chatID string, part Part, plain bool) SendResult
	DownloadFile(ctx context.Context, token, fileID string, maxBytes int64) ([]byte, error)
}

// Registry maps provider names to adapters.
type Registry struct {
	adapters map[string]Adapter
}

// NewRegistry builds a registry from adapters.
func NewRegistry(adapters ...Adapter) *Registry {
	r := &Registry{adapters: make(map[string]Adapter, len(adapters))}
	for _, adapter := range adapters {
		r.adapters[adapter.Provider()] = adapter
	}
	return r
}

// Lookup returns the provider's adapter.
func (r *Registry) Lookup(provider string) (Adapter, bool) {
	if r == nil {
		return nil, false
	}
	adapter, ok := r.adapters[provider]
	return adapter, ok
}
