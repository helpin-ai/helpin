package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"nhooyr.io/websocket"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// portalSessionCookie is the customer portal session cookie (see the portal
// HTTP handler). Its path covers /api/public/portal/{slug}/ws.
const portalSessionCookie = "helpin_portal_session"

// portalSocketLifetime bounds a portal connection so access and the set of
// requests are re-checked on reconnect.
const portalSocketLifetime = 10 * time.Minute

// ErrPortalSocketUnauthorized rejects a portal socket without a valid session.
var ErrPortalSocketUnauthorized = errors.New("portal session is not valid")

// PortalScope limits a customer portal connection to the requests its
// customer owns. It maps conversation IDs to the public request references;
// conversation IDs are never sent to the client.
type PortalScope struct {
	References map[string]string
}

// PortalAuthenticator authenticates a portal socket and lists the customer's
// requests.
type PortalAuthenticator interface {
	AuthenticatePortalSocket(ctx context.Context, slug, sessionSecret string) (*model.PortalSocketGrant, error)
}

// PortalHandler upgrades /api/public/portal/{slug}/ws for signed-in customers.
type PortalHandler struct {
	hub            *Hub
	auth           PortalAuthenticator
	allowedOrigins []string
}

// NewPortalHandler creates the portal socket handler. appBaseURL is the
// portal's origin; browsers on other origins are refused.
func NewPortalHandler(hub *Hub, auth PortalAuthenticator, appBaseURL string) *PortalHandler {
	var origins []string
	if parsed, err := url.Parse(strings.TrimSpace(appBaseURL)); err == nil && parsed.Host != "" {
		origins = append(origins, parsed.Host)
	}
	return &PortalHandler{hub: hub, auth: auth, allowedOrigins: origins}
}

// PortalSocketSlug returns the slug when path is a portal socket path.
func PortalSocketSlug(path string) (string, bool) {
	rest, ok := strings.CutPrefix(path, "/api/public/portal/")
	if !ok {
		return "", false
	}
	slug, ok := strings.CutSuffix(rest, "/ws")
	if !ok || slug == "" || strings.Contains(slug, "/") {
		return "", false
	}
	return slug, true
}

// ServeHTTP authenticates the portal cookie, then streams change signals for
// the customer's requests. It sends no message content: the client refetches
// through the portal API, which enforces visibility.
func (h *PortalHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug, ok := PortalSocketSlug(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	cookie, err := r.Cookie(portalSessionCookie)
	if err != nil || cookie.Value == "" {
		http.Error(w, "portal session required", http.StatusUnauthorized)
		return
	}
	grant, err := h.auth.AuthenticatePortalSocket(r.Context(), slug, cookie.Value)
	if errors.Is(err, ErrPortalSocketUnauthorized) {
		http.Error(w, "portal session required", http.StatusUnauthorized)
		return
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "portal socket authentication failed", "error", err)
		http.Error(w, "portal temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.allowedOrigins})
	if err != nil {
		slog.WarnContext(r.Context(), "portal socket accept failed", "workspace_id", grant.WorkspaceID, "error", err)
		return
	}
	client := &Client{
		Conn:        conn,
		ConnID:      uuid.NewString(),
		UserID:      "portal:" + grant.IdentityID,
		WorkspaceID: grant.WorkspaceID,
		Portal:      &PortalScope{References: grant.References},
	}
	h.hub.Register(client)
	defer h.hub.Unregister(client)

	ctx, cancel := context.WithTimeout(r.Context(), portalSocketLifetime)
	defer cancel()
	// The client sends nothing; reading detects disconnects and answers pings.
	for {
		if _, _, err := conn.Read(ctx); err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				conn.Close(websocket.StatusNormalClosure, "reconnect")
			}
			return
		}
	}
}

// portalMessage is the portal wire format: a change or typing signal for one
// request, identified by its public reference.
type portalMessage struct {
	Type      string `json:"type"`
	Reference string `json:"reference"`
	Typing    *bool  `json:"typing,omitempty"`
}

// portalPayload returns the signal a portal client should receive, or nil.
// customerVisible reports whether a message event carries a customer-visible
// message (the widget projection is non-empty).
func portalPayload(scope *PortalScope, event Event, customerVisible bool) []byte {
	if event.TargetUserID != "" {
		return nil
	}
	signal := func(conversationID, kind string, typing *bool) []byte {
		reference, ok := scope.References[conversationID]
		if !ok {
			return nil
		}
		data, _ := json.Marshal(portalMessage{Type: kind, Reference: reference, Typing: typing})
		return data
	}
	on, off := true, false
	switch event.Entity {
	case "support_conversation_message":
		if !customerVisible {
			return nil // internal notes and drafts stay invisible, including their timing
		}
		return signal(event.ParentID, "request:changed", nil)
	case "support_conversation":
		switch {
		case isViewingEvent(event.Action):
			return nil
		case isTypingEvent(event.Action):
			if isWidgetActor(event.ActorID) || strings.HasPrefix(event.ActorID, "portal:") {
				return nil
			}
			if event.Action == "typing_started" {
				return signal(event.EntityID, "request:typing", &on)
			}
			return signal(event.EntityID, "request:typing", &off)
		default:
			return signal(event.EntityID, "request:changed", nil)
		}
	case SupportAIResponseStreamEntity:
		switch event.Action {
		case "response_started", "progress", "response_delta":
			return signal(event.ParentID, "request:typing", &on)
		case "response_completed":
			return signal(event.ParentID, "request:typing", &off)
		}
	}
	return nil
}
