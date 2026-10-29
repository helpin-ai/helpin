package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// setCCMode applies a copy mode to every signed-in session of the fixture.
func setCCMode(mode string, accesses ...*PortalAccess) {
	for _, access := range accesses {
		access.Workspace.Settings.PortalCCMode = mode
	}
}

func TestPortalCCIsOffByDefault(t *testing.T) {
	p := setupCompanyPortal(t)
	ctx := context.Background()
	if _, err := p.svc.SetRequestCC(ctx, p.bobIn, p.bobRequest.Reference, []string{"alice@acme.test"}); !errors.Is(err, ErrPortalCCDisabled) {
		t.Fatalf("set cc with copying off = %v", err)
	}
	if _, err := p.svc.CreateRequest(ctx, p.bobIn, "Hi", "Hello", nil, nil, PortalRequestFields{CCEmails: []string{"alice@acme.test"}}); !errors.Is(err, ErrPortalCCDisabled) {
		t.Fatalf("create with cc while copying off = %v", err)
	}
	detail, err := p.svc.RequestDetail(ctx, p.bobIn, p.bobRequest.Reference)
	if err != nil || detail.CanEditCC {
		t.Fatalf("detail can edit cc = %v, %v", detail.CanEditCC, err)
	}
}

func TestPortalCCColleaguesMode(t *testing.T) {
	p := setupCompanyPortal(t)
	ctx := context.Background()
	p.exec(t, `UPDATE crm_companies SET domain = 'www.acme.test' WHERE id = ?`, p.acme)
	setCCMode(model.SupportPortalCCColleagues, p.bobIn, p.aliceIn)

	options, err := p.svc.CCOptions(ctx, p.bobIn)
	if err != nil || len(options.Colleagues) != 1 || options.Colleagues[0].Email != "alice@acme.test" || len(options.Domains) != 1 || options.Domains[0] != "acme.test" {
		t.Fatalf("options = %#v, %v", options, err)
	}
	for _, tc := range []struct {
		name   string
		emails []string
		want   error
	}{
		{"outside address", []string{"carol@other.test"}, ErrPortalCCInvalid},
		{"not an address", []string{"alice"}, ErrPortalCCInvalid},
		{"too many", []string{"a1@acme.test", "a2@acme.test", "a3@acme.test", "a4@acme.test", "a5@acme.test", "a6@acme.test", "a7@acme.test", "a8@acme.test", "a9@acme.test", "a10@acme.test", "a11@acme.test"}, ErrPortalCCInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := p.svc.SetRequestCC(ctx, p.bobIn, p.bobRequest.Reference, tc.emails); !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
	// Members, company domain addresses, and the requester (dropped) pass.
	detail, err := p.svc.SetRequestCC(ctx, p.bobIn, p.bobRequest.Reference, []string{"Alice@Acme.test", "dave@acme.test", "bob@acme.test"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(detail.CC, ",") != "alice@acme.test,dave@acme.test" || !detail.CanEditCC {
		t.Fatalf("cc = %v can edit %v", detail.CC, detail.CanEditCC)
	}
	// Only the requester changes who is copied.
	if _, err := p.svc.SetRequestCC(ctx, p.aliceIn, p.bobRequest.Reference, nil); !errors.Is(err, ErrPortalCCDenied) {
		t.Fatalf("colleague changing cc = %v", err)
	}
	var events []string
	if err := p.db.Model(&model.SupportMessage{}).Where("conversation_id = ? AND system_event_type = ?", p.bobConversationID(t), model.SystemEventEmailRecipientsUpdated).Pluck("content", &events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || !strings.Contains(events[0], "Bob Example added alice@acme.test to Cc.") {
		t.Fatalf("timeline = %q", events)
	}
}

func TestPortalCCCopiedPersonSeesAndReplies(t *testing.T) {
	p := setupCompanyPortal(t)
	ctx := context.Background()
	sender := p.svc.sender.(*recordingPortalSender)
	createNamedPortalContact(t, p.db, p.ws.ID, "Carol", "carol@other.test", portalAccess(model.PortalAccessAllowed))
	carolIn := signInPortal(t, p.svc, p.ws, issuePortalLink(t, p.db, p.ws.ID, "carol@other.test", time.Now().Add(-time.Hour)))
	setCCMode(model.SupportPortalCCAny, p.bobIn, carolIn)
	sentBefore := len(sender.sent)

	if _, err := p.svc.SetRequestCC(ctx, p.bobIn, p.bobRequest.Reference, []string{"carol@other.test"}); err != nil {
		t.Fatal(err)
	}
	notices := sender.sent[sentBefore:]
	if len(notices) != 1 || notices[0].to != "carol@other.test" || !strings.Contains(notices[0].subject, "copied you on request") || !strings.Contains(notices[0].text, "/requests/"+p.bobRequest.Reference) {
		t.Fatalf("notices = %#v", notices)
	}

	copied, err := p.svc.CopiedRequests(ctx, carolIn, "")
	if err != nil || len(copied) != 1 || copied[0].Reference != p.bobRequest.Reference {
		t.Fatalf("copied requests = %v, %v", requestReferences(copied), err)
	}
	if mine, _ := p.svc.Requests(ctx, carolIn, ""); len(mine) != 0 {
		t.Fatalf("copied request listed as Carol's own: %v", requestReferences(mine))
	}
	detail, err := p.svc.RequestDetail(ctx, carolIn, p.bobRequest.Reference)
	if err != nil || !detail.Copied || detail.CanEditCC || detail.CanRate {
		t.Fatalf("copied detail = %+v, %v", detail, err)
	}
	if _, err := p.svc.Reply(ctx, carolIn, p.bobRequest.Reference, "Adding the March numbers.", nil); err != nil {
		t.Fatalf("copied reply: %v", err)
	}
	if _, err := p.svc.SetRequestCC(ctx, carolIn, p.bobRequest.Reference, nil); !errors.Is(err, ErrPortalCCDenied) {
		t.Fatalf("copied person changing cc = %v", err)
	}
	var views int64
	p.db.Model(&model.SupportPortalAuditEvent{}).Where("event_type = ?", model.SupportPortalAuditCopiedRequestViewed).Count(&views)
	if views == 0 {
		t.Fatal("copied view not audited")
	}

	// Re-adding within a day sends no second notice.
	if _, err := p.svc.SetRequestCC(ctx, p.bobIn, p.bobRequest.Reference, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := p.svc.SetRequestCC(ctx, p.bobIn, p.bobRequest.Reference, []string{"carol@other.test"}); err != nil {
		t.Fatal(err)
	}
	if len(sender.sent) != sentBefore+1 {
		t.Fatalf("notices after re-adding = %d, want 1", len(sender.sent)-sentBefore)
	}

	// Removed, or with copying turned off, the request is gone again.
	if _, err := p.svc.SetRequestCC(ctx, p.bobIn, p.bobRequest.Reference, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := p.svc.RequestDetail(ctx, carolIn, p.bobRequest.Reference); !errors.Is(err, ErrPortalRequestNotFound) {
		t.Fatalf("removed person detail = %v", err)
	}
	if _, err := p.svc.SetRequestCC(ctx, p.bobIn, p.bobRequest.Reference, []string{"carol@other.test"}); err != nil {
		t.Fatal(err)
	}
	setCCMode("", carolIn)
	if _, err := p.svc.RequestDetail(ctx, carolIn, p.bobRequest.Reference); !errors.Is(err, ErrPortalRequestNotFound) {
		t.Fatalf("detail with copying off = %v", err)
	}
	if copied, _ := p.svc.CopiedRequests(ctx, carolIn, ""); len(copied) != 0 {
		t.Fatalf("copied list with copying off = %v", requestReferences(copied))
	}
}

func TestPortalCCOnNewRequest(t *testing.T) {
	p := setupCompanyPortal(t)
	ctx := context.Background()
	setCCMode(model.SupportPortalCCColleagues, p.aliceIn)
	if _, err := p.svc.CreateRequest(ctx, p.aliceIn, "Hi", "Hello", nil, nil, PortalRequestFields{CCEmails: []string{"eve@evil.test"}}); !errors.Is(err, ErrPortalCCInvalid) {
		t.Fatalf("outside cc on create = %v", err)
	}
	request, err := p.svc.CreateRequest(ctx, p.aliceIn, "Hi", "Hello", nil, nil, PortalRequestFields{CCEmails: []string{"bob@acme.test"}})
	if err != nil {
		t.Fatal(err)
	}
	detail, err := p.svc.RequestDetail(ctx, p.aliceIn, request.Reference)
	if err != nil || len(detail.CC) != 1 || detail.CC[0] != "bob@acme.test" {
		t.Fatalf("new request cc = %v, %v", detail.CC, err)
	}
}
