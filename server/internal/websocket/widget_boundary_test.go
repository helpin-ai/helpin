package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ws "nhooyr.io/websocket"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestVisitorTypingNeverSendsDraftOnWire(t *testing.T) {
	hub := NewHub()
	ready := make(chan struct{})
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := ws.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		conv := "audit-conversation"
		client := &Client{Conn: conn, WorkspaceID: "audit-workspace", UserID: "widget:audit", IsWidget: true, ConversationID: &conv}
		hub.Register(client)
		defer hub.Unregister(client)
		close(ready)
		<-done
	}))
	defer srv.Close()
	defer close(done)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := ws.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	<-ready
	go hub.Broadcast(Event{Entity: "support_conversation", Action: "typing_started", EntityID: "audit-conversation", WorkspaceID: "audit-workspace", ActorID: "staff-audit", Data: json.RawMessage(`{"agent_name":"Support","content":"UNSENT_PRIVATE_DRAFT"}`)})
	_, b, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "UNSENT_PRIVATE_DRAFT") {
		t.Fatalf("visitor received unsent draft: %s", b)
	}

}

func TestVisitorEventMetadataAndPresenceBoundary(t *testing.T) {
	data := json.RawMessage(`{"id":"m","content":"Public answer","metadata":"{\"ai_model\":\"PRIVATE_MODEL\",\"link_previews\":[{\"url\":\"https://example.com\"}]}","email_bcc":["PRIVATE_BCC"],"future":"PRIVATE_FUTURE"}`)
	got := widgetSafeSupportMessageEventData(data)
	if strings.Contains(string(got), "PRIVATE_") {
		t.Fatalf("internal event fields leaked: %s", got)
	}
	if !strings.Contains(string(got), "Public answer") || !strings.Contains(string(got), "link_previews") {
		t.Fatalf("lost public event fields: %s", got)
	}
	for _, raw := range []string{`{"is_internal":true,"content":"Private note"}`, `{"system_event_type":"assigned","content":"Private assignment"}`, `{"metadata":"{\"delivery_mode\":\"email_only\"}"}`} {
		if len(widgetSafeSupportMessageEventData(json.RawMessage(raw))) != 0 {
			t.Errorf("private event accepted: %s", raw)
		}
	}
	hub := NewHub()
	client := &Client{IsWidget: true}
	event := Event{Entity: "support_teammate_presence", Action: "updated", EntityID: "other-staff"}
	if hub.shouldReceive(client, event) {
		t.Fatal("visitor subscribed to unknown staff")
	}
	hub.rememberWidgetTeammates(client, "public-agent")
	event.EntityID = "public-agent"
	if !hub.shouldReceive(client, event) {
		t.Fatal("visitor lost public teammate availability")
	}
	safe := widgetTeammatePresenceData(json.RawMessage(`{"user_id":"public-agent","status":"online","last_seen_at":"PRIVATE_TIMESTAMP"}`))
	if strings.Contains(string(safe), "PRIVATE_") {
		t.Fatal("presence leaked staff activity timestamp")
	}
}

type visitorBoundaryService struct {
	WidgetService
	messages      []model.SupportMessage
	conversations []model.SupportConversation
}

func (s *visitorBoundaryService) GetVisitorConversations(context.Context, string, string) ([]model.SupportConversation, error) {
	return s.conversations, nil
}
func (s *visitorBoundaryService) ListWidgetConversationMessages(context.Context, string, string) ([]model.SupportMessage, error) {
	return s.messages, nil
}
func (s *visitorBoundaryService) SetSessionConversation(context.Context, string, string) error {
	return nil
}
func (s *visitorBoundaryService) TouchWidgetSessionActivity(context.Context, string) error {
	return nil
}
func (s *visitorBoundaryService) MarkConversationReadByVisitor(context.Context, string, string, string) error {
	return nil
}
func (s *visitorBoundaryService) WidgetCreateMessage(context.Context, string, string, []string) (*model.SupportMessage, error) {
	return &s.messages[0], nil
}

func TestVisitorSessionAndSelectionUsePublicProjection(t *testing.T) {
	secret, conv := "PRIVATE_REFERENCE", "conversation"
	service := &visitorBoundaryService{
		messages:      []model.SupportMessage{{ID: "public", ConversationID: conv, Content: "Public answer", EmailBCC: model.DocsStringArray{"PRIVATE_BCC"}, Metadata: `{"ai_model":"PRIVATE_MODEL","link_security":[{"status":"safe"}],"capture_email":true}`}, {ID: "note", IsInternal: true, Content: "PRIVATE_NOTE"}},
		conversations: []model.SupportConversation{{ID: conv, Subject: "Public subject", CRMContactID: &secret, AIActiveRunID: &secret, ListLastMessagePreview: &secret}},
	}
	handler := NewWidgetHandler(NewHub(), service)
	session := &model.SupportWidgetSession{ID: "session", WorkspaceID: "workspace", AnonymousID: "visitor", ConversationID: &conv}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := ws.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		if err := handler.sendSessionJoined(r.Context(), conn, session); err != nil {
			return
		}
		handler.handleConnection(r.Context(), conn, session, "")
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := ws.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	readSafe := func(want string) {
		t.Helper()
		_, b, err := conn.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "PRIVATE_") || strings.Contains(string(b), "link_security") {
			t.Fatalf("private wire data: %s", b)
		}
		if !strings.Contains(string(b), want) {
			t.Fatalf("lost %q: %s", want, b)
		}
	}
	readSafe("session:joined")
	for _, frame := range []struct{ input, want string }{
		{`{"type":"conversation:select","data":{"conversation_id":"conversation"}}`, "Public answer"},
		{`{"type":"message:send","data":{"content":"Hello"}}`, "Public answer"},
		{`{"type":"conversations:list"}`, "Public subject"},
	} {
		if err := conn.Write(ctx, ws.MessageText, []byte(frame.input)); err != nil {
			t.Fatal(err)
		}
		readSafe(frame.want)
	}
}

func TestStaffTypingStillIncludesDraft(t *testing.T) {
	hub := NewHub()
	ready := make(chan struct{})
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := ws.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		client := &Client{Conn: conn, WorkspaceID: "workspace", UserID: "observer"}
		hub.Register(client)
		defer hub.Unregister(client)
		close(ready)
		<-done
	}))
	defer srv.Close()
	defer close(done)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := ws.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	<-ready
	go hub.Broadcast(Event{Entity: "support_conversation", Action: "typing_started", EntityID: "conversation", WorkspaceID: "workspace", ActorID: "writer", Data: json.RawMessage(`{"content":"STAFF_DRAFT"}`)})
	_, b, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "STAFF_DRAFT") {
		t.Fatal("staff draft preview was lost")
	}
}
