package sync

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGmailSyncClient_SendMessageRejectsHeaderInjection(t *testing.T) {
	client := &GmailSyncClient{}
	if _, err := client.SendMessage(context.Background(), "token", "owner@example.com", []string{"buyer@example.com\r\nBcc: attacker@example.com"}, nil, "Hello", "<p>Hi</p>"); err == nil {
		t.Fatal("expected recipient header injection to be rejected")
	}
	if _, err := client.SendMessage(context.Background(), "token", "owner@example.com", []string{"buyer@example.com"}, nil, "Hello\r\nBcc: attacker@example.com", "<p>Hi</p>"); err == nil {
		t.Fatal("expected subject header injection to be rejected")
	}
}

func TestGmailSyncClient_SendMessageBuildsSafeMIMEPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var payload map[string]string
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		raw, err := base64.URLEncoding.DecodeString(payload["raw"])
		if err != nil {
			t.Fatalf("decode raw message: %v", err)
		}
		message := string(raw)
		if message == "" || !containsAll(message, "From: <owner@example.com>", "To: <buyer@example.com>", "Subject: Hello") {
			t.Fatalf("unexpected MIME message: %q", message)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"sent-1","threadId":"thread-1"}`))
	}))
	defer server.Close()

	client := &GmailSyncClient{httpClient: server.Client(), apiBaseURL: server.URL}
	result, err := client.SendMessage(context.Background(), "token", "owner@example.com", []string{"buyer@example.com"}, nil, "Hello", "<p>Hi</p>")
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if result.ID != "sent-1" || result.ThreadID != "thread-1" {
		t.Fatalf("result = %+v", result)
	}
}

func TestGmailSyncClient_SendThreadMessagePreservesGmailThreading(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var payload map[string]string
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload["threadId"] != "gmail-thread-1" {
			t.Fatalf("threadId = %q, want gmail-thread-1", payload["threadId"])
		}
		raw, err := base64.URLEncoding.DecodeString(payload["raw"])
		if err != nil {
			t.Fatalf("decode raw message: %v", err)
		}
		message := string(raw)
		if !containsAll(message, "In-Reply-To: <message-1@example.com>", "References: <older@example.com> <message-1@example.com>") {
			t.Fatalf("thread headers missing from MIME message: %q", message)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"sent-2","threadId":"gmail-thread-1"}`))
	}))
	defer server.Close()

	client := &GmailSyncClient{httpClient: server.Client(), apiBaseURL: server.URL}
	result, err := client.SendThreadMessage(
		context.Background(), "token", "owner@example.com", []string{"buyer@example.com"}, nil,
		"Re: Hello", "<p>Thanks</p>", "gmail-thread-1", "<message-1@example.com>",
		"<older@example.com> <message-1@example.com>",
	)
	if err != nil {
		t.Fatalf("SendThreadMessage: %v", err)
	}
	if result.ID != "sent-2" || result.ThreadID != "gmail-thread-1" {
		t.Fatalf("result = %+v", result)
	}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}

func TestParseAddressListWithNames_DropsInvalidEntriesInFallback(t *testing.T) {
	addrs, names := parseAddressListWithNames(`Alice Example <alice@example.com>, invalid-entry, Bob Example <bob@example.com>`)

	if len(addrs) != 2 {
		t.Fatalf("addrs = %v, want 2 valid addresses", addrs)
	}
	if addrs[0] != "alice@example.com" || addrs[1] != "bob@example.com" {
		t.Fatalf("addrs = %v, want normalized valid addresses only", addrs)
	}
	if names["alice@example.com"] != "Alice Example" {
		t.Fatalf("alice name = %q, want Alice Example", names["alice@example.com"])
	}
	if names["bob@example.com"] != "Bob Example" {
		t.Fatalf("bob name = %q, want Bob Example", names["bob@example.com"])
	}
}

func TestParseGmailMessage_PreservesLabelIDsAndNormalizesFrom(t *testing.T) {
	raw := &gmailRawMessage{
		ID:        "msg-1",
		ThreadID:  "thread-1",
		HistoryID: "history-1",
		LabelIDs:  []string{"INBOX", "SENT"},
	}
	raw.Payload.Headers = []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}{
		{Name: "From", Value: `Alice Example <ALICE@example.com>`},
		{Name: "To", Value: `Bob Example <bob@example.com>`},
		{Name: "Message-ID", Value: `<message-1@example.com>`},
		{Name: "In-Reply-To", Value: `<message-0@example.com>`},
		{Name: "References", Value: `<message-0@example.com>`},
	}

	msg := parseGmailMessage(raw)
	if msg.From != "alice@example.com" {
		t.Fatalf("from = %q, want alice@example.com", msg.From)
	}
	if len(msg.LabelIDs) != 2 || msg.LabelIDs[1] != "SENT" {
		t.Fatalf("label_ids = %v, want preserved labels", msg.LabelIDs)
	}
	if msg.RFCMessageID != "<message-1@example.com>" || msg.InReplyTo != "<message-0@example.com>" || msg.ReferencesHeader != "<message-0@example.com>" {
		t.Fatalf("reply headers = %q/%q/%q, want parsed RFC headers", msg.RFCMessageID, msg.InReplyTo, msg.ReferencesHeader)
	}
}

func TestGmailSyncClient_GetMailboxProfile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gmail/v1/users/me/profile" {
			t.Fatalf("path = %q, want /gmail/v1/users/me/profile", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"emailAddress":"owner@example.com","historyId":"hist-123"}`))
	}))
	defer server.Close()

	client := &GmailSyncClient{
		httpClient: server.Client(),
		apiBaseURL: server.URL + "/gmail/v1/users/me",
	}

	profile, err := client.GetMailboxProfile(context.Background(), "token")
	if err != nil {
		t.Fatalf("GetMailboxProfile: %v", err)
	}
	if profile.EmailAddress != "owner@example.com" || profile.HistoryID != "hist-123" {
		t.Fatalf("profile = %+v, want owner@example.com/hist-123", profile)
	}
}

func TestGmailSyncClient_ListHistoryDedupesAcrossPages(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		switch callCount {
		case 1:
			if r.URL.Query().Get("startHistoryId") != "hist-1" {
				t.Fatalf("startHistoryId = %q, want hist-1", r.URL.Query().Get("startHistoryId"))
			}
			_, _ = w.Write([]byte(`{
				"historyId":"hist-2",
				"nextPageToken":"page-2",
				"history":[
					{"messages":[{"id":"msg-1"}]},
					{"messagesAdded":[{"message":{"id":"msg-2"}}]}
				]
			}`))
		case 2:
			if r.URL.Query().Get("pageToken") != "page-2" {
				t.Fatalf("pageToken = %q, want page-2", r.URL.Query().Get("pageToken"))
			}
			_, _ = w.Write([]byte(`{
				"historyId":"hist-3",
				"history":[
					{"labelsAdded":[{"message":{"id":"msg-2"}}]},
					{"labelsRemoved":[{"message":{"id":"msg-3"}}]}
				]
			}`))
		default:
			t.Fatalf("unexpected history page request %d", callCount)
		}
	}))
	defer server.Close()

	client := &GmailSyncClient{
		httpClient: server.Client(),
		apiBaseURL: server.URL + "/gmail/v1/users/me",
	}

	result, err := client.ListHistory(context.Background(), "token", "hist-1")
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if result.LatestHistoryID != "hist-3" {
		t.Fatalf("LatestHistoryID = %q, want hist-3", result.LatestHistoryID)
	}
	if len(result.MessageIDs) != 3 {
		t.Fatalf("MessageIDs = %v, want 3 unique ids", result.MessageIDs)
	}
	if result.MessageIDs[0] != "msg-1" || result.MessageIDs[1] != "msg-2" || result.MessageIDs[2] != "msg-3" {
		t.Fatalf("MessageIDs = %v, want msg-1,msg-2,msg-3", result.MessageIDs)
	}
}
