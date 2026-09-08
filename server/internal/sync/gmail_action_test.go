package sync

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGmailPlaybookActionUsesStableMessageIdentity(t *testing.T) {
	const identity = "<crm-action-00000000-0000-4000-8000-000000000001@helpin.ai>"
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || !strings.HasSuffix(r.URL.Path, "/messages/send") {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		decoded, err := base64.URLEncoding.DecodeString(payload["raw"])
		if err != nil {
			t.Error(err)
		}
		if strings.Count(string(decoded), "Message-ID: "+identity) != 1 {
			t.Errorf("stable Message-ID missing from MIME")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"sent-1","threadId":"thread-1"}`))
	}))
	defer server.Close()
	client := &GmailSyncClient{httpClient: server.Client(), apiBaseURL: server.URL}
	result, err := client.SendMessageWithMessageID(context.Background(), "test-token", "owner@example.test", []string{"customer@example.test"}, nil, "Next step", "<p>Discuss the next step?</p>", identity)
	if err != nil || result == nil || result.ID != "sent-1" || calls != 1 {
		t.Fatalf("send: %#v %v calls=%d", result, err, calls)
	}
	_, err = client.SendMessageWithMessageID(context.Background(), "test-token", "owner@example.test", []string{"customer@example.test"}, nil, "Next step", "<p>Discuss?</p>", identity+"\r\nBcc: stranger@example.test")
	if err == nil || calls != 1 {
		t.Fatal("unsafe identity reached provider")
	}
}

func TestGmailPlaybookActionInspectionNeverSends(t *testing.T) {
	const identity = "<crm-action-00000000-0000-4000-8000-000000000001@helpin.ai>"
	for _, mode := range []string{"absent", "found", "ambiguous"} {
		t.Run(mode, func(t *testing.T) {
			posts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method != "GET" {
					posts++
					http.Error(w, "unexpected write", 400)
					return
				}
				if strings.HasSuffix(r.URL.Path, "/messages") {
					if r.URL.Query().Get("q") != "in:sent rfc822msgid:"+identity {
						t.Errorf("wrong inspection query: %s", r.URL.RawQuery)
					}
					switch mode {
					case "absent":
						_, _ = w.Write([]byte(`{"messages":[]}`))
					case "ambiguous":
						_, _ = w.Write([]byte(`{"messages":[],"nextPageToken":"more"}`))
					default:
						_, _ = w.Write([]byte(`{"messages":[{"id":"sent-1"}]}`))
					}
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "sent-1", "threadId": "thread-1", "payload": map[string]any{"mimeType": "text/html", "headers": []map[string]string{{"name": "Message-ID", "value": identity}, {"name": "Subject", "value": "Next step"}}, "body": map[string]string{"data": base64.RawURLEncoding.EncodeToString([]byte("<p>Discuss?</p>"))}}})
			}))
			defer server.Close()
			client := &GmailSyncClient{httpClient: server.Client(), apiBaseURL: server.URL}
			result, err := client.FindSentMessageByMessageID(context.Background(), "test-token", identity)
			if posts != 0 {
				t.Fatal("inspection attempted a write")
			}
			switch mode {
			case "absent":
				if err != nil || result != nil {
					t.Fatalf("absence was not inconclusive: %v", err)
				}
			case "ambiguous":
				if err == nil || result != nil {
					t.Fatal("ambiguous result claimed success")
				}
			case "found":
				if err != nil || result == nil || result.RFCMessageID != identity {
					t.Fatalf("correlation failed: %#v %v", result, err)
				}
			}
		})
	}
}
