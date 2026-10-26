package service

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type fakeTeamReplyConversations struct{ conv *model.SupportConversation }

func (f *fakeTeamReplyConversations) GetByID(_ context.Context, workspaceID, id, _, _ string) (*model.SupportConversation, error) {
	if f.conv == nil || f.conv.WorkspaceID != workspaceID || f.conv.ID != id {
		return nil, nil
	}
	copy := *f.conv
	return &copy, nil
}

type fakeTeamReplyWorkspaces struct{}

func (fakeTeamReplyWorkspaces) GetByID(_ context.Context, id string) (*model.Workspace, error) {
	return &model.Workspace{ID: id, Name: "Northwind"}, nil
}

type fakeTeamReplyInbox struct {
	settings model.SupportInboxSettings
	notes    []string
}

func (f *fakeTeamReplyInbox) GetInstallation(context.Context, string) (*model.SupportWidgetInstallation, *model.SupportInboxSettings, error) {
	settings := f.settings
	return nil, &settings, nil
}

func (f *fakeTeamReplyInbox) CreateConversationMessage(_ context.Context, _, _ string, req model.CreateMessageRequest, senderType string, _, _ *string, _ *string) (*model.SupportMessage, error) {
	if senderType != "customer" || req.MessageType != "reply" {
		return nil, errors.New("note must be a customer reply")
	}
	f.notes = append(f.notes, req.Content)
	return &model.SupportMessage{Content: req.Content}, nil
}

type fakeTeamReplyEscalator struct {
	conversations *fakeTeamReplyConversations
	reasons       []string
	briefs        []SupportHandoffBrief
}

func (f *fakeTeamReplyEscalator) EscalateToHumanForMessageWithIssue(_ context.Context, _, _, _, reason, _, _ string, briefs ...SupportHandoffBrief) error {
	f.reasons = append(f.reasons, reason)
	f.briefs = append(f.briefs, briefs...)
	now := time.Now()
	f.conversations.conv.AIState = strPtr("escalated")
	f.conversations.conv.CustomerRequestedHumanAt = &now
	return nil
}

func newTeamReplyTestService(t *testing.T) (*SupportTeamReplyService, *fakeTeamReplyConversations, *fakeTeamReplyInbox, *fakeTeamReplyEscalator) {
	t.Helper()
	conversations := &fakeTeamReplyConversations{conv: &model.SupportConversation{ID: "conv-1", WorkspaceID: "ws-1", Subject: "CSV export is missing the Notes column", CustomerEmail: strPtr("sarah@example.com"), CustomerName: strPtr("Sarah"), AIState: strPtr("pending")}}
	settings := model.DefaultSupportInboxSettings()
	settings.ReplyTimePreset = "few_hours"
	inbox := &fakeTeamReplyInbox{settings: settings}
	escalator := &fakeTeamReplyEscalator{conversations: conversations}
	svc := NewSupportTeamReplyService("secret", "https://app.example.com/", conversations, fakeTeamReplyWorkspaces{}, inbox, escalator)
	return svc, conversations, inbox, escalator
}

func teamReplyToken(t *testing.T, link string) string {
	t.Helper()
	parsed, err := url.Parse(link)
	if err != nil || !strings.HasPrefix(parsed.Path, "/team-reply/") {
		t.Fatalf("unexpected link %q", link)
	}
	token, err := url.PathUnescape(strings.TrimPrefix(parsed.Path, "/team-reply/"))
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestSupportTeamReplyLinkRejectsForgedTokens(t *testing.T) {
	svc, _, _, _ := newTeamReplyTestService(t)
	token := teamReplyToken(t, svc.TeamReplyURL("ws-1", "conv-1"))
	other := NewSupportTeamReplyService("another secret", "https://app.example.com", svc.conversations, svc.workspaces, svc.inbox, svc.escalator)
	encoded, _, _ := strings.Cut(token, ".")
	forged := svc.sign("ws-1", "conv-2", time.Now().Add(time.Hour))
	forgedPayload, _, _ := strings.Cut(forged, ".")
	_, signature, _ := strings.Cut(token, ".")
	for name, bad := range map[string]string{
		"other secret":      teamReplyToken(t, other.TeamReplyURL("ws-1", "conv-1")),
		"swapped payload":   forgedPayload + "." + signature,
		"missing signature": encoded,
		"garbage":           "not-a-token",
	} {
		if _, err := svc.Page(context.Background(), bad); !errors.Is(err, ErrSupportTeamReplyLinkInvalid) {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
	if _, err := svc.Page(context.Background(), token); err != nil {
		t.Fatal(err)
	}
}

func TestSupportTeamReplyPageNeverHandsOff(t *testing.T) {
	svc, _, _, escalator := newTeamReplyTestService(t)
	token := teamReplyToken(t, svc.TeamReplyURL("ws-1", "conv-1"))
	page, err := svc.Page(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if page.State != SupportTeamReplyStateOpen || page.Subject != "CSV export is missing the Notes column" || page.CustomerEmail != "sarah@example.com" || page.Branding["name"] != "Northwind" {
		t.Fatalf("page = %+v", page)
	}
	if page.ReplyTimeText != "We usually reply in a few hours." {
		t.Fatalf("reply time = %q", page.ReplyTimeText)
	}
	if len(escalator.reasons) != 0 {
		t.Fatal("opening the link handed the conversation off")
	}
}

func TestSupportTeamReplyRequestHandsOffWithoutHandoffEmail(t *testing.T) {
	svc, _, inbox, escalator := newTeamReplyTestService(t)
	token := teamReplyToken(t, svc.TeamReplyURL("ws-1", "conv-1"))
	page, err := svc.Request(context.Background(), token, "  The column is still empty.  ")
	if err != nil {
		t.Fatal(err)
	}
	if page.State != SupportTeamReplyStateDone {
		t.Fatalf("state = %s", page.State)
	}
	if len(escalator.reasons) != 1 || escalator.reasons[0] != "customer_requested_human" || !escalator.briefs[0].CustomerNotified {
		t.Fatalf("handoff = %v %+v", escalator.reasons, escalator.briefs)
	}
	if len(inbox.notes) != 1 || inbox.notes[0] != "The column is still empty." {
		t.Fatalf("notes = %q", inbox.notes)
	}
	// A double click shows the same confirmation without a second handoff.
	again, err := svc.Request(context.Background(), token, "The column is still empty.")
	if err != nil || again.State != SupportTeamReplyStateDone || len(escalator.reasons) != 1 || len(inbox.notes) != 1 {
		t.Fatalf("repeat press: %+v %v reasons=%d notes=%d", again, err, len(escalator.reasons), len(inbox.notes))
	}
}

func TestSupportTeamReplyStates(t *testing.T) {
	t.Run("already with the team", func(t *testing.T) {
		svc, conversations, _, escalator := newTeamReplyTestService(t)
		conversations.conv.HumanTakeover = boolPtr(true)
		token := teamReplyToken(t, svc.TeamReplyURL("ws-1", "conv-1"))
		for _, call := range []func() (*SupportTeamReplyPage, error){
			func() (*SupportTeamReplyPage, error) { return svc.Page(context.Background(), token) },
			func() (*SupportTeamReplyPage, error) { return svc.Request(context.Background(), token, "note") },
		} {
			page, err := call()
			if err != nil || page.State != SupportTeamReplyStateAlready {
				t.Fatalf("page = %+v %v", page, err)
			}
		}
		if len(escalator.reasons) != 0 {
			t.Fatal("handed off a conversation a person owns")
		}
	})
	t.Run("expired", func(t *testing.T) {
		svc, _, inbox, escalator := newTeamReplyTestService(t)
		token := teamReplyToken(t, svc.TeamReplyURL("ws-1", "conv-1"))
		svc.now = func() time.Time { return time.Now().Add(supportTeamReplyLinkTTL + time.Hour) }
		page, err := svc.Request(context.Background(), token, "note")
		if err != nil || page.State != SupportTeamReplyStateExpired || len(escalator.reasons) != 0 || len(inbox.notes) != 0 {
			t.Fatalf("expired link acted: %+v %v", page, err)
		}
	})
	t.Run("deleted conversation", func(t *testing.T) {
		svc, conversations, _, _ := newTeamReplyTestService(t)
		token := teamReplyToken(t, svc.TeamReplyURL("ws-1", "conv-1"))
		now := time.Now()
		conversations.conv.AnonymizedAt = &now
		if _, err := svc.Page(context.Background(), token); !errors.Is(err, ErrSupportTeamReplyLinkInvalid) {
			t.Fatalf("err = %v", err)
		}
	})
}
