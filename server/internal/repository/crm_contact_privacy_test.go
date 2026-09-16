package repository

import (
	"encoding/json"
	"testing"
)

func TestRedactContactIdentityJSONPreservesHistory(t *testing.T) {
	raw := json.RawMessage(`{"name":"Alice","delivery_to_email":"alice@example.com","email_cc":["friend@example.com"],"nested":{"IP_Address":"192.0.2.1","comment":"Alice asked for help","count":9007199254740993},"messages":[{"sender_display_name":"Alice","body":"Email me at alice@example.com"}],"rating":5,"subject":"Alice's account"}`)
	got, err := redactContactIdentityJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"messages":[{"body":"Email me at alice@example.com"}],"nested":{"comment":"Alice asked for help","count":9007199254740993},"rating":5,"subject":"Alice's account"}`
	if string(got) != want {
		t.Fatalf("history changed or identity remains: %s", got)
	}
	twice, err := redactContactIdentityJSON(got)
	if err != nil || string(twice) != string(got) {
		t.Fatalf("not idempotent: %s %v", twice, err)
	}
	if _, err := redactContactIdentityJSON(json.RawMessage(`{"broken"`)); err == nil {
		t.Fatal("accepted malformed metadata")
	}
}
