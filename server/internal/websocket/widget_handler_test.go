package websocket

import (
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestUnmarshalWidgetData_MessageSendData(t *testing.T) {
	tests := []struct {
		name        string
		data        map[string]any
		wantContent string
		wantErr     bool
	}{
		{
			name:        "valid content",
			data:        map[string]any{"content": "Hello, world!"},
			wantContent: "Hello, world!",
			wantErr:     false,
		},
		{
			name:        "empty content",
			data:        map[string]any{"content": ""},
			wantContent: "",
			wantErr:     false,
		},
		{
			name:        "missing content field",
			data:        map[string]any{},
			wantContent: "",
			wantErr:     false,
		},
		{
			name:        "nil data",
			data:        nil,
			wantContent: "",
			wantErr:     false,
		},
		{
			name:        "extra fields ignored",
			data:        map[string]any{"content": "Hi", "extra": "ignored"},
			wantContent: "Hi",
			wantErr:     false,
		},
		{
			name:        "wrong type for content",
			data:        map[string]any{"content": 123},
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := unmarshalWidgetData[model.WidgetMessageSendData](tt.data)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Content != tt.wantContent {
				t.Errorf("content = %q, want %q", result.Content, tt.wantContent)
			}
		})
	}
}

func TestUnmarshalWidgetData_TypingData(t *testing.T) {
	tests := []struct {
		name        string
		data        map[string]any
		wantContent string
	}{
		{
			name:        "with content preview",
			data:        map[string]any{"content": "I need help with..."},
			wantContent: "I need help with...",
		},
		{
			name:        "empty — typing without content",
			data:        map[string]any{},
			wantContent: "",
		},
		{
			name:        "nil data",
			data:        nil,
			wantContent: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := unmarshalWidgetData[model.WidgetTypingData](tt.data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Content != tt.wantContent {
				t.Errorf("content = %q, want %q", result.Content, tt.wantContent)
			}
		})
	}
}

func TestUnmarshalWidgetData_SessionUpgradeData(t *testing.T) {
	tests := []struct {
		name      string
		data      map[string]any
		wantEmail string
		wantName  string
		wantErr   bool
	}{
		{
			name:      "valid email and name",
			data:      map[string]any{"email": "user@example.com", "name": "Jane Doe"},
			wantEmail: "user@example.com",
			wantName:  "Jane Doe",
		},
		{
			name:      "email only",
			data:      map[string]any{"email": "user@example.com"},
			wantEmail: "user@example.com",
			wantName:  "",
		},
		{
			name:      "empty data",
			data:      map[string]any{},
			wantEmail: "",
			wantName:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := unmarshalWidgetData[model.WidgetSessionUpgradeData](tt.data)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Email != tt.wantEmail {
				t.Errorf("email = %q, want %q", result.Email, tt.wantEmail)
			}
			if result.Name != tt.wantName {
				t.Errorf("name = %q, want %q", result.Name, tt.wantName)
			}
		})
	}
}

func TestUnmarshalWidgetData_ConversationSelectData(t *testing.T) {
	tests := []struct {
		name   string
		data   map[string]any
		wantID string
	}{
		{
			name:   "valid conversation ID",
			data:   map[string]any{"conversation_id": "conv-abc-123"},
			wantID: "conv-abc-123",
		},
		{
			name:   "empty conversation ID",
			data:   map[string]any{"conversation_id": ""},
			wantID: "",
		},
		{
			name:   "missing field",
			data:   map[string]any{},
			wantID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := unmarshalWidgetData[model.WidgetConversationSelectData](tt.data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.ConversationID != tt.wantID {
				t.Errorf("conversation_id = %q, want %q", result.ConversationID, tt.wantID)
			}
		})
	}
}

func TestUnmarshalWidgetData_SessionCreateData(t *testing.T) {
	data := map[string]any{
		"anonymous_id": "anon-123",
		"page_url":     "https://example.com/pricing",
		"user_agent":   "Mozilla/5.0",
		"page_title":   "Pricing",
		"timezone":     "America/New_York",
		"locale":       "en-US",
	}

	result, err := unmarshalWidgetData[model.WidgetSessionCreateData](data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AnonymousID != "anon-123" {
		t.Errorf("anonymous_id = %q, want %q", result.AnonymousID, "anon-123")
	}
	if result.PageURL != "https://example.com/pricing" {
		t.Errorf("page_url = %q, want %q", result.PageURL, "https://example.com/pricing")
	}
	if result.UserAgent != "Mozilla/5.0" {
		t.Errorf("user_agent = %q, want %q", result.UserAgent, "Mozilla/5.0")
	}
	if result.PageTitle != "Pricing" {
		t.Errorf("page_title = %q, want %q", result.PageTitle, "Pricing")
	}
	if result.Timezone != "America/New_York" {
		t.Errorf("timezone = %q, want %q", result.Timezone, "America/New_York")
	}
	if result.Locale != "en-US" {
		t.Errorf("locale = %q, want %q", result.Locale, "en-US")
	}
}

func TestUnmarshalWidgetData_SessionRestoreData(t *testing.T) {
	data := map[string]any{"session_token": "tok_abc123"}
	result, err := unmarshalWidgetData[model.WidgetSessionRestoreData](data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SessionToken != "tok_abc123" {
		t.Errorf("session_token = %q, want %q", result.SessionToken, "tok_abc123")
	}
}

func TestUnmarshalWidgetData_RoundTripFromJSON(t *testing.T) {
	// Simulate what actually happens: JSON comes in as WidgetWSMessage, Data is map[string]any
	raw := `{"type":"message:send","data":{"content":"Hello from widget"}}`
	var msg model.WidgetWSMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("failed to unmarshal WidgetWSMessage: %v", err)
	}

	if msg.Type != "message:send" {
		t.Fatalf("type = %q, want %q", msg.Type, "message:send")
	}

	result, err := unmarshalWidgetData[model.WidgetMessageSendData](msg.Data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "Hello from widget" {
		t.Errorf("content = %q, want %q", result.Content, "Hello from widget")
	}
}
