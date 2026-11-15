package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/channels"
)

const testToken = "123456:SECRET-token"

// fakeBotAPI answers Bot API methods with canned handlers.
func fakeBotAPI(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prefix := "/bot" + testToken + "/"
		if strings.HasPrefix(r.URL.Path, "/file/bot"+testToken+"/") {
			handlers["file"](w, r)
			return
		}
		if !strings.HasPrefix(r.URL.Path, prefix) {
			http.Error(w, `{"ok":false,"error_code":404,"description":"Not Found"}`, http.StatusNotFound)
			return
		}
		method := strings.TrimPrefix(r.URL.Path, prefix)
		handler, ok := handlers[method]
		if !ok {
			t.Errorf("unexpected method %s", method)
			http.Error(w, "unexpected", http.StatusInternalServerError)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	return server
}

func reply(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

func TestValidateTokenAndWebhookRegistration(t *testing.T) {
	var webhook map[string]any
	server := fakeBotAPI(t, map[string]http.HandlerFunc{
		"getMe": reply(200, `{"ok":true,"result":{"id":424242,"is_bot":true,"first_name":"Acme Support","username":"acme_support_bot"}}`),
		"setWebhook": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&webhook)
			reply(200, `{"ok":true,"result":true}`)(w, r)
		},
		"deleteWebhook": reply(200, `{"ok":true,"result":true}`),
	})
	a := NewAdapter(server.URL)
	info, err := a.ValidateToken(context.Background(), testToken)
	if err != nil {
		t.Fatal(err)
	}
	if info != (channels.AccountInfo{ExternalAccountID: "424242", DisplayName: "Acme Support", Handle: "acme_support_bot"}) {
		t.Errorf("info = %+v", info)
	}
	if err := a.RegisterWebhook(context.Background(), testToken, "https://helpin.example/api/webhooks/channels/telegram/acct", "s3cret_value"); err != nil {
		t.Fatal(err)
	}
	if webhook["secret_token"] != "s3cret_value" || webhook["url"] != "https://helpin.example/api/webhooks/channels/telegram/acct" {
		t.Errorf("webhook params = %v", webhook)
	}
	if updates, _ := webhook["allowed_updates"].([]any); len(updates) != 1 || updates[0] != "message" {
		t.Errorf("allowed_updates = %v", webhook["allowed_updates"])
	}
	if err := a.UnregisterWebhook(context.Background(), testToken); err != nil {
		t.Fatal(err)
	}
}

func TestValidateTokenRejectedCredentials(t *testing.T) {
	for _, status := range []int{401, 404} {
		server := fakeBotAPI(t, map[string]http.HandlerFunc{
			"getMe": reply(status, `{"ok":false,"error_code":`+strconv.Itoa(status)+`,"description":"Unauthorized"}`),
		})
		if _, err := NewAdapter(server.URL).ValidateToken(context.Background(), testToken); !errors.Is(err, channels.ErrAuth) {
			t.Errorf("status %d: err = %v", status, err)
		}
	}
}

func TestSendPartOutcomes(t *testing.T) {
	part := channels.Part{Text: "<b>Hi</b>", PlainText: "Hi"}
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		outcome channels.Outcome
		code    string
		retry   time.Duration
	}{
		{"accepted", reply(200, `{"ok":true,"result":{"message_id":77,"chat":{"id":5551234}}}`), channels.Accepted, "", 0},
		{"rate limited", reply(429, `{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 7","parameters":{"retry_after":7}}`), channels.NotSent, "rate_limited", 7 * time.Second},
		{"blocked", reply(403, `{"ok":false,"error_code":403,"description":"Forbidden: bot was blocked by the user"}`), channels.Rejected, channels.CodeBlocked, 0},
		{"bad formatting", reply(400, `{"ok":false,"error_code":400,"description":"Bad Request: can't parse entities: Unsupported start tag"}`), channels.Rejected, channels.CodeParse, 0},
		{"chat not found", reply(400, `{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`), channels.Rejected, channels.CodeBadRequest, 0},
		{"token revoked", reply(401, `{"ok":false,"error_code":401,"description":"Unauthorized"}`), channels.Rejected, channels.CodeAuth, 0},
		{"server error", reply(500, `{"ok":false,"error_code":500,"description":"Internal Server Error"}`), channels.Unknown, "provider_error_500", 0},
		{"proxy page", reply(502, `<html>Bad Gateway</html>`), channels.Unknown, "provider_error_502", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotParseMode any
			server := fakeBotAPI(t, map[string]http.HandlerFunc{"sendMessage": func(w http.ResponseWriter, r *http.Request) {
				var params map[string]any
				_ = json.NewDecoder(r.Body).Decode(&params)
				gotParseMode = params["parse_mode"]
				tc.handler(w, r)
			}})
			result := NewAdapter(server.URL).SendPart(context.Background(), testToken, "5551234", part, false)
			if result.Outcome != tc.outcome || result.Code != tc.code || result.RetryAfter != tc.retry {
				t.Fatalf("result = %+v", result)
			}
			if tc.outcome == channels.Accepted && result.ExternalKey != "5551234:77" {
				t.Errorf("external key = %q", result.ExternalKey)
			}
			if gotParseMode != "HTML" {
				t.Errorf("parse_mode = %v", gotParseMode)
			}
			if strings.Contains(result.Message, testToken) {
				t.Error("token leaked into the result")
			}
		})
	}
}

func TestSendPartPlainFallbackOmitsParseMode(t *testing.T) {
	var params map[string]any
	server := fakeBotAPI(t, map[string]http.HandlerFunc{"sendMessage": func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&params)
		reply(200, `{"ok":true,"result":{"message_id":78,"chat":{"id":1}}}`)(w, r)
	}})
	NewAdapter(server.URL).SendPart(context.Background(), testToken, "1", channels.Part{Text: "<b>Hi</b>", PlainText: "Hi"}, true)
	if params["text"] != "Hi" || params["parse_mode"] != nil {
		t.Errorf("plain params = %v", params)
	}
}

func TestSendPartNetworkFailures(t *testing.T) {
	// Nothing listens: the request never reaches the wire, so a retry is safe.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closedURL := "http://" + listener.Addr().String()
	_ = listener.Close()
	result := NewAdapter(closedURL).SendPart(context.Background(), testToken, "1", channels.Part{Text: "x", PlainText: "x"}, false)
	if result.Outcome != channels.NotSent || strings.Contains(result.Message, testToken) {
		t.Errorf("refused = %+v", result)
	}

	// The server reads the request and never answers: it may have been sent.
	hang := make(chan struct{})
	server := fakeBotAPI(t, map[string]http.HandlerFunc{"sendMessage": func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		<-hang
	}})
	// Registered after the server, so it runs first and frees the handler.
	t.Cleanup(func() { close(hang) })
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	result = NewAdapter(server.URL).SendPart(ctx, testToken, "1", channels.Part{Text: "x", PlainText: "x"}, false)
	if result.Outcome != channels.Unknown || strings.Contains(result.Message, testToken) {
		t.Errorf("timeout after write = %+v", result)
	}
}

func TestSendPhotoUploadsMultipartWithCaption(t *testing.T) {
	var caption, parseMode, fileName string
	server := fakeBotAPI(t, map[string]http.HandlerFunc{"sendPhoto": func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		caption, parseMode = r.FormValue("caption"), r.FormValue("parse_mode")
		_, header, err := r.FormFile("photo")
		if err != nil {
			t.Fatal(err)
		}
		fileName = header.Filename
		reply(200, `{"ok":true,"result":{"message_id":90,"chat":{"id":1}}}`)(w, r)
	}})
	file := channels.OutgoingFile{Name: "../evil\"name.png", ContentType: "image/png", Data: []byte("png")}
	result := NewAdapter(server.URL).SendPart(context.Background(), testToken, "1", channels.Part{Text: "<b>label</b>", PlainText: "label", File: &file}, false)
	if result.Outcome != channels.Accepted || caption != "<b>label</b>" || parseMode != "HTML" || fileName != "evilname.png" {
		t.Errorf("result=%+v caption=%q mode=%q file=%q", result, caption, parseMode, fileName)
	}
}

func TestDownloadFileEnforcesSizeAndHidesToken(t *testing.T) {
	server := fakeBotAPI(t, map[string]http.HandlerFunc{
		"getFile": func(w http.ResponseWriter, r *http.Request) {
			var params map[string]string
			_ = json.NewDecoder(r.Body).Decode(&params)
			switch params["file_id"] {
			case "big-declared":
				reply(200, `{"ok":true,"result":{"file_id":"big-declared","file_size":30000000,"file_path":"documents/big.bin"}}`)(w, r)
			case "big-actual":
				reply(200, `{"ok":true,"result":{"file_id":"big-actual","file_size":10,"file_path":"documents/sneaky.bin"}}`)(w, r)
			default:
				reply(200, `{"ok":true,"result":{"file_id":"ok","file_size":5,"file_path":"photos/file 1.jpg"}}`)(w, r)
			}
		},
		"file": func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "sneaky.bin") {
				_, _ = io.WriteString(w, strings.Repeat("x", 100))
				return
			}
			if !strings.HasSuffix(r.URL.Path, "photos/file 1.jpg") {
				http.NotFound(w, r)
				return
			}
			_, _ = io.WriteString(w, "jpeg!")
		},
	})
	a := NewAdapter(server.URL)
	data, err := a.DownloadFile(context.Background(), testToken, "ok", 1024)
	if err != nil || string(data) != "jpeg!" {
		t.Fatalf("download = %q %v", data, err)
	}
	for _, id := range []string{"big-declared", "big-actual"} {
		limit := int64(_maxDownload)
		if id == "big-actual" {
			limit = 50
		}
		_, err := a.DownloadFile(context.Background(), testToken, id, limit)
		if !errors.Is(err, ErrFileTooLarge) {
			t.Errorf("%s: err = %v", id, err)
		}
	}
	_, err = NewAdapter("http://127.0.0.1:1").DownloadFile(context.Background(), testToken, "ok", 1024)
	if err == nil || strings.Contains(err.Error(), testToken) {
		t.Errorf("unreachable download err = %v", err)
	}
}
