package main

import (
	"bufio"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadMCPMessageSupportsJSONLineFrames(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n"))

	msg, format, err := readMCPMessage(reader)
	if err != nil {
		t.Fatalf("read message: %v", err)
	}
	if format != frameJSONLine {
		t.Fatalf("expected JSON-line frame, got %q", format)
	}
	if string(msg) != `{"jsonrpc":"2.0","id":1,"method":"initialize"}` {
		t.Fatalf("unexpected message %q", string(msg))
	}
}

func TestReadMCPMessageSupportsContentLengthFrames(t *testing.T) {
	payload := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	reader := bufio.NewReader(strings.NewReader(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(payload), payload)))

	msg, format, err := readMCPMessage(reader)
	if err != nil {
		t.Fatalf("read message: %v", err)
	}
	if format != frameContentLength {
		t.Fatalf("expected content-length frame, got %q", format)
	}
	if string(msg) != payload {
		t.Fatalf("unexpected message %q", string(msg))
	}
}

func TestHandleInitializeResponse(t *testing.T) {
	resp := handleRequest(nil, "", "", rpcRequest{
		JSONRPC: "2.0",
		ID:      []byte(`1`),
		Method:  "initialize",
	})

	if resp.Error != nil {
		t.Fatalf("initialize returned error: %+v", resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected initialize result")
	}
}

func TestListToolsReportsNonJSONAPIResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>frontend</html>"))
	}))
	t.Cleanup(server.Close)

	_, err := listTools(server.Client(), server.URL, "token")
	if err == nil {
		t.Fatal("expected non-JSON error")
	}
	if !strings.Contains(err.Error(), "Helpin API returned non-JSON response") || !strings.Contains(err.Error(), "text/html") {
		t.Fatalf("unexpected error: %v", err)
	}
}
