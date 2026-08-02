package handler

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeExternalMCPJSONIsStrictAndSingleObject(t *testing.T) {
	tests := []string{
		`{"name":"server","unknown":true}`,
		`{"name":"server"}{"name":"second"}`,
	}
	for _, body := range tests {
		req := httptest.NewRequest("POST", "/api/external-mcp/servers", strings.NewReader(body))
		rec := httptest.NewRecorder()
		var target struct {
			Name string `json:"name"`
		}
		if err := decodeExternalMCPJSON(rec, req, &target); err == nil {
			t.Fatalf("expected strict decoding error for %q", body)
		}
	}
}

func TestDecodeExternalMCPJSONAcceptsOneKnownObject(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/external-mcp/servers", strings.NewReader(`{"name":"server"}`))
	rec := httptest.NewRecorder()
	var target struct {
		Name string `json:"name"`
	}
	if err := decodeExternalMCPJSON(rec, req, &target); err != nil || target.Name != "server" {
		t.Fatalf("target=%#v err=%v", target, err)
	}
}
