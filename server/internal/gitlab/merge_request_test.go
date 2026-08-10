package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpdateMergeRequestDescription(t *testing.T) {
	var gotDescription string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v4/projects/99/merge_requests/7" {
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		gotDescription = payload["description"]
		_ = json.NewEncoder(w).Encode(MergeRequest{IID: 7, Title: "Existing MR", Description: gotDescription})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	mr, err := client.UpdateMergeRequestDescription(context.Background(), "token", 99, 7, "updated body")
	if err != nil {
		t.Fatalf("UpdateMergeRequestDescription: %v", err)
	}
	if gotDescription != "updated body" || mr == nil || mr.Description != "updated body" {
		t.Fatalf("description = %q, mr = %#v", gotDescription, mr)
	}
}
