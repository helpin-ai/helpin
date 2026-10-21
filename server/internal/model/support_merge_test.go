package model

import (
	"strings"
	"testing"
)

func TestSameSupportCustomer(t *testing.T) {
	ptr := func(s string) *string { return &s }
	tests := []struct {
		name string
		a, b SupportConversation
		want bool
	}{
		{"unknown is not a customer identity", SupportConversation{}, SupportConversation{}, false},
		{"contact identity", SupportConversation{CRMContactID: ptr("contact")}, SupportConversation{CRMContactID: ptr("contact")}, true},
		{"email ignores case and whitespace", SupportConversation{CustomerEmail: ptr(" SAM@example.test ")}, SupportConversation{CustomerEmail: ptr("sam@example.test")}, true},
		{"different known contacts", SupportConversation{CRMContactID: ptr("a"), CustomerEmail: ptr("same@example.test")}, SupportConversation{CRMContactID: ptr("b"), CustomerEmail: ptr("same@example.test")}, false},
		{"anonymous browser identity", SupportConversation{AnonymousID: ptr("browser")}, SupportConversation{AnonymousID: ptr("browser")}, true},
		{"names do not grant identity", SupportConversation{CustomerName: ptr("Sam")}, SupportConversation{CustomerName: ptr("Sam")}, false},
		{"unconfirmed address", SupportConversation{CustomerEmail: ptr("sam@example.test"), PrimaryRecipientState: "unconfirmed"}, SupportConversation{CustomerEmail: ptr("sam@example.test")}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SameSupportCustomer(&tt.a, &tt.b); got != tt.want {
				t.Errorf("same customer = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSupportMergeRoot(t *testing.T) {
	for _, tt := range []struct {
		name        string
		edges       map[string]string
		start, want string
		invalid     bool
	}{
		{"standalone", nil, "a", "a", false},
		{"chain", map[string]string{"a": "b", "b": "c"}, "a", "c", false},
		{"cycle", map[string]string{"a": "b", "b": "a"}, "a", "", true},
		{"self", map[string]string{"a": "a"}, "a", "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SupportMergeRoot(tt.start, tt.edges)
			if (err != nil) != tt.invalid || got != tt.want {
				t.Fatalf("root = %q, %v; want %q, invalid=%v", got, err, tt.want, tt.invalid)
			}
		})
	}
}

func TestMergeSharedEmailHidesQuotedInlineAttachments(t *testing.T) {
	msg := SupportMessage{SenderType: "customer", MessageType: "reply", ViaChannel: ptrMergeTest("email"), Content: "New reply with PRIVATE quoted text", Metadata: `{"merge_public_text":"New reply ![image](cid:new-image)"}`, Attachments: []SupportAttachmentPayload{{ID: "quoted", ContentID: "old-image"}, {ID: "new-inline", ContentID: "new-image"}, {ID: "attached-file"}}}
	visible := PublicWidgetMessage(&msg)
	if visible == nil || len(visible.Attachments) != 2 || visible.Attachments[0].ID != "new-inline" || strings.Contains(visible.Content, "PRIVATE") {
		t.Fatalf("quoted history leaked: %+v", visible)
	}
}
func ptrMergeTest(s string) *string { return &s }
