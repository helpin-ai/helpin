package slack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type slackFileTransport func(*http.Request) (*http.Response, error)

func (f slackFileTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSlackFileTransferContract(t *testing.T) {
	c := NewClient()
	var calls int
	c.HTTP.Transport = slackFileTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		body := ""
		switch r.URL.Path {
		case "/api/files.getUploadURLExternal":
			if r.Method != "POST" || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || r.PostFormValue("filename") != "checkout.txt" || r.PostFormValue("length") != "3" || r.URL.RawQuery != "" || r.Header.Get("Authorization") != "Bearer token" {
				t.Fatalf("upload ticket request: %s %v", r.Method, r.URL)
			}
			body = `{"ok":true,"file_id":"F1","upload_url":"https://files.slack.com/upload"}`
		case "/upload":
			if r.Header.Get("Authorization") != "" {
				t.Fatal("token sent to upload URL")
			}
			b, _ := io.ReadAll(r.Body)
			if string(b) != "abc" {
				t.Fatalf("upload bytes %s", b)
			}
		case "/api/files.completeUploadExternal":
			var request struct {
				Channel string `json:"channel_id"`
				Thread  string `json:"thread_ts"`
				Files   []struct{ ID, Title string }
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Channel != "C1" || request.Thread != "1.0" || len(request.Files) != 1 || request.Files[0].ID != "F1" || request.Files[0].Title != "checkout.txt" {
				t.Fatalf("incorrect file destination or title: %+v %v", request, err)
			}
			body = `{"ok":true}`
		case "/api/files.info":
			body = `{"ok":true,"file":{"id":"F1","name":"checkout.txt","mimetype":"text/plain","size":3,"url_private_download":"https://files.slack.com/private"}}`
		case "/private":
			if r.Header.Get("Authorization") != "Bearer token" {
				t.Fatal("missing download authorization")
			}
			body = "abc"
		default:
			t.Fatalf("unexpected URL %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	if _, err := c.Upload(context.Background(), "token", "C1", "1.0", "checkout.txt", []byte("abc")); err != nil {
		t.Fatal(err)
	}
	file, data, err := c.DownloadFile(context.Background(), "token", "F1", 3)
	if err != nil || file.Name != "checkout.txt" || file.Mime != "text/plain" || string(data) != "abc" || calls != 5 {
		t.Fatalf("file=%+v data=%q calls=%d err=%v", file, data, calls, err)
	}
}

func TestSlackFileExceedsLimit(t *testing.T) {
	for _, size := range []int{4, 0} {
		t.Run(fmt.Sprintf("reported size %d", size), func(t *testing.T) {
			c := NewClient()
			downloaded := false
			c.HTTP.Transport = slackFileTransport(func(r *http.Request) (*http.Response, error) {
				body := fmt.Sprintf(`{"ok":true,"file":{"id":"F1","name":"large.txt","size":%d,"url_private":"https://files.slack.com/private"}}`, size)
				if r.URL.Path == "/private" {
					downloaded = true
					body = "abcd"
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})
			file, data, err := c.DownloadFile(context.Background(), "token", "F1", 3)
			if !errors.Is(err, ErrFileTooLarge) || file.Name != "large.txt" || data != nil || downloaded != (size == 0) {
				t.Fatalf("size=%d downloaded=%v data=%q file=%+v err=%v", size, downloaded, data, file, err)
			}
		})
	}
}
