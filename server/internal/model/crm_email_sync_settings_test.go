package model

import (
	"encoding/json"
	"testing"
)

func TestShouldFilterEmailParticipantsChecksOutboundRecipients(t *testing.T) {
	patterns, _ := json.Marshal([]string{"*@blocked.example", "vip@allowed.example"})
	settings := DefaultEmailSyncSettings()
	settings.FilterPatterns = patterns

	settings.FilterMode = "blocklist"
	if !ShouldFilterEmailParticipants(&settings, "owner@helpin.example", "owner@helpin.example", []string{"buyer@blocked.example"}, nil) {
		t.Fatal("expected an outbound blocked recipient to filter the message")
	}

	settings.FilterMode = "allowlist"
	if ShouldFilterEmailParticipants(&settings, "owner@helpin.example", "owner@helpin.example", []string{"vip@allowed.example"}, nil) {
		t.Fatal("expected an outbound allowlisted recipient to include the message")
	}
	if !ShouldFilterEmailParticipants(&settings, "owner@helpin.example", "owner@helpin.example", []string{"other@example.com"}, nil) {
		t.Fatal("expected a non-allowlisted outbound recipient to filter the message")
	}
}

func TestShouldFilterEmailParticipantsIgnoresMailboxIdentity(t *testing.T) {
	patterns, _ := json.Marshal([]string{"owner@helpin.example"})
	settings := DefaultEmailSyncSettings()
	settings.FilterMode = "blocklist"
	settings.FilterPatterns = patterns

	if ShouldFilterEmailParticipants(&settings, "owner@helpin.example", "buyer@example.com", []string{"owner@helpin.example"}, nil) {
		t.Fatal("mailbox identity must not cause an external conversation to be filtered")
	}
}
