package service

import "testing"

func TestSlackInboxCreateCommand(t *testing.T) {
	for _, tt := range []struct{ text, kind, body string }{
		{"<@BOT> create a ticket for the export failure", "ticket", "the export failure"},
		{"<@BOT> please file a bug for engineering", "bug", "engineering"},
		{"<@BOT> open a separate support ticket: Billing failed", "support", "Billing failed"},
		{"<@BOT> can you create a task about Search?", "task", "Search?"},
		{"<@BOT> create a ticket", "ticket", ""},
		{"<@BOT> open a separate ticket for invoices", "support", "invoices"},
		{"create a ticket", "", ""},
		{"<@OTHER> create a ticket", "", ""},
		{"We asked <@BOT> to open a ticket", "", ""},
		{"<@BOT> create a ticketing system", "", ""},
		{"<@BOT> thanks for your help", "", ""},
	} {
		kind, body := inboxCreateCommand(tt.text, "BOT")
		if kind != tt.kind || body != tt.body {
			t.Errorf("%q: got (%q, %q), want (%q, %q)", tt.text, kind, body, tt.kind, tt.body)
		}
	}
}
