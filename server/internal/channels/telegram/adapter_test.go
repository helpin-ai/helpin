package telegram

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/channels"
)

func parseFixture(t *testing.T, name string) channels.Event {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	events, err := NewAdapter("").ParseWebhook(body)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	if len(events) != 1 {
		t.Fatalf("parse %s: %d events", name, len(events))
	}
	return events[0]
}

func TestParsePrivateTextKeepsFormattingAndIdentity(t *testing.T) {
	e := parseFixture(t, "private_text.json")
	if e.Kind != channels.EventMessage || e.EventKey != "900000001" {
		t.Fatalf("event = %+v", e)
	}
	if e.ExternalUserID != "5551234" || e.ExternalChatID != "5551234" || e.ExternalMessageKey != "5551234:41" {
		t.Errorf("ids = %q %q %q", e.ExternalUserID, e.ExternalChatID, e.ExternalMessageKey)
	}
	if e.DisplayName != "Ada Lovelace" || e.Username != "ada_l" {
		t.Errorf("identity = %q %q", e.DisplayName, e.Username)
	}
	// The emoji is two UTF-16 units; offsets after it must still land right.
	want := "Hi 👋 **my order** is late, see [tracking](https://track.example.com/1)"
	if e.Text != want {
		t.Errorf("text = %q\nwant  %q", e.Text, want)
	}
	if e.SentAt.Unix() != 1763020800 {
		t.Errorf("sent at = %v", e.SentAt)
	}
}

func TestParseMessageFiles(t *testing.T) {
	for _, tc := range []struct {
		fixture, text, fileID, name, contentType string
		size                                     int64
	}{
		{"photo_caption.json", "_Broken_ box", "AgAC-large", "photo.jpg", "image/jpeg", 84000},
		{"document.json", "", "BQAC-doc", "invoice.pdf", "application/pdf", 20480},
		{"animation.json", "", "CgAC-anim", "oops.mp4", "video/mp4", 51200},
		{"voice.json", "", "AwAC-voice", "voice.ogg", "audio/ogg", 9100},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			e := parseFixture(t, tc.fixture)
			if e.Kind != channels.EventMessage || e.Text != tc.text {
				t.Fatalf("kind=%s text=%q", e.Kind, e.Text)
			}
			// Animations also carry a document copy; it must not become a second file.
			if len(e.Files) != 1 {
				t.Fatalf("files = %+v", e.Files)
			}
			f := e.Files[0]
			if f.FileID != tc.fileID || f.FileName != tc.name || f.ContentType != tc.contentType || f.Size != tc.size {
				t.Errorf("file = %+v", f)
			}
		})
	}
}

func TestParseUnsupportedAndIgnoredUpdates(t *testing.T) {
	sticker := parseFixture(t, "sticker.json")
	if sticker.Kind != channels.EventUnsupported || sticker.UnsupportedType != "sticker" || sticker.ExternalMessageKey != "5551234:46" {
		t.Errorf("sticker = %+v", sticker)
	}
	for _, fixture := range []string{"group_message.json", "edited_message.json"} {
		e := parseFixture(t, fixture)
		if e.Kind != channels.EventIgnored || e.EventKey == "" || e.IgnoredReason == "" {
			t.Errorf("%s = %+v", fixture, e)
		}
	}
	if _, err := NewAdapter("").ParseWebhook([]byte(`{"message":{}}`)); err == nil {
		t.Error("an update without update_id must be rejected")
	}
	if _, err := NewAdapter("").ParseWebhook([]byte(`not json`)); err == nil {
		t.Error("a malformed body must be rejected")
	}
}

func TestTwoChatsSharingMessageIDGetDistinctKeys(t *testing.T) {
	a, b := parseFixture(t, "private_text.json"), parseFixture(t, "other_chat_same_message_id.json")
	if a.ExternalMessageKey == b.ExternalMessageKey {
		t.Fatalf("message keys collide: %q", a.ExternalMessageKey)
	}
}

func TestRenderPutsFirstTextInCaptionWhenItFits(t *testing.T) {
	a := NewAdapter("")
	file := channels.OutgoingFile{Name: "label.png", ContentType: "image/png", Data: []byte("png")}
	parts := a.Render(channels.Outgoing{Text: "Here is your **label**", Files: []channels.OutgoingFile{file}})
	if len(parts) != 1 || parts[0].File == nil || parts[0].Text != "Here is your <b>label</b>" || parts[0].PlainText != "Here is your label" {
		t.Fatalf("parts = %+v", parts)
	}

	long := strings.Repeat("word ", 300) // over the 1024 caption limit
	parts = a.Render(channels.Outgoing{Text: long, Files: []channels.OutgoingFile{file}})
	if len(parts) != 2 || parts[0].File == nil || parts[0].Text != "" || parts[1].File != nil || parts[1].Text == "" {
		t.Fatalf("long caption parts = %d %+v", len(parts), parts[0])
	}

	if parts := a.Render(channels.Outgoing{Text: "   "}); len(parts) != 0 {
		t.Fatalf("blank text rendered %d parts", len(parts))
	}
}
