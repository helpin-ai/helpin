package service

import (
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestBuildCRMBehaviorProfilesMergesKeysPerRecord(t *testing.T) {
	now := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	userID, contactID, companyID := "user-42", "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"
	foreignContact := "99999999-9999-9999-9999-999999999999"
	index := &repository.CRMBehaviorIdentityIndex{
		Links: []repository.CRMBehaviorIdentityLink{
			{AnonymousID: "anon-1", ExternalUserID: &userID, ContactID: &contactID, CompanyID: &companyID},
			// A link naming a contact outside the workspace is ignored.
			{AnonymousID: "anon-2", ContactID: &foreignContact},
		},
		ContactEmails: map[string][]string{
			"ada@example.com":    {contactID},
			"shared@example.com": {"33333333-3333-3333-3333-333333333333", "44444444-4444-4444-4444-444444444444"},
			"solo@example.com":   {"55555555-5555-5555-5555-555555555555"},
		},
		ContactIDs: map[string]bool{
			contactID: true, "33333333-3333-3333-3333-333333333333": true,
			"44444444-4444-4444-4444-444444444444": true, "55555555-5555-5555-5555-555555555555": true,
		},
		CompanyKeys:       map[string]string{"acct-7": companyID, companyID: companyID},
		CompanyKeyMethods: map[string]string{"acct-7": model.CRMBehaviorMatchExternalID, companyID: model.CRMBehaviorMatchRecordID},
	}
	at := func(days int) time.Time { return now.AddDate(0, 0, days) }
	stats := []repository.BehaviorKeyStats{
		{Key: repository.BehaviorKeyUserID + userID, FirstSeenAt: at(-40), LastSeenAt: at(-2), RecentSessionIDs: []string{"s1", "s2"}},
		{Key: repository.BehaviorKeyEmail + "ada@example.com", FirstSeenAt: at(-50), LastSeenAt: at(-3), RecentSessionIDs: []string{"s2", "s3"}},
		{Key: repository.BehaviorKeyAnonymousID + "anon-1", FirstSeenAt: at(-60), LastSeenAt: at(-1), RecentSessionIDs: []string{"s4"}},
		{Key: repository.BehaviorKeyAnonymousID + "anon-2", FirstSeenAt: at(-6), LastSeenAt: at(-6)},
		{Key: repository.BehaviorKeyEmail + "shared@example.com", FirstSeenAt: at(-5), LastSeenAt: at(-1)},
		{Key: repository.BehaviorKeyEmail + "solo@example.com", FirstSeenAt: at(-5), LastSeenAt: at(-4), RecentSessionIDs: []string{"x"}},
		{Key: repository.BehaviorKeyCompanyID + "acct-7", FirstSeenAt: at(-90), LastSeenAt: at(-10), RecentSessionIDs: []string{"s9"}},
		{Key: repository.BehaviorKeyUserID + "stranger", FirstSeenAt: at(-1), LastSeenAt: at(0)},
		// The SDK may identify a visitor with the CRM contact's own id.
		{Key: repository.BehaviorKeyUserID + contactID, FirstSeenAt: at(-20), LastSeenAt: at(-20)},
	}

	profiles, result := buildCRMBehaviorProfiles("ws-1", index, stats, now)
	byID := map[string]model.CRMBehaviorProfile{}
	for _, profile := range profiles {
		byID[profile.EntityType+"/"+profile.EntityID] = profile
	}
	ada := byID["contact/"+contactID]
	if !ada.FirstSeenAt.Equal(at(-60)) || !ada.LastSeenAt.Equal(at(-1)) || ada.Sessions30d != 4 ||
		ada.MatchMethods != "email,identity_link,record_id" || ada.WorkspaceID != "ws-1" || !ada.ComputedAt.Equal(now) {
		t.Fatalf("ada profile = %+v", ada)
	}
	company := byID["company/"+companyID]
	if !company.FirstSeenAt.Equal(at(-90)) || company.Sessions30d != 4 || company.MatchMethods != "external_id,identity_link" {
		t.Fatalf("company profile = %+v", company)
	}
	if _, ok := byID["contact/"+foreignContact]; ok {
		t.Fatal("a link to a contact outside the workspace must not create a profile")
	}
	if _, ok := byID["contact/33333333-3333-3333-3333-333333333333"]; ok {
		t.Fatal("an email shared by several contacts must not match any of them")
	}
	solo := byID["contact/55555555-5555-5555-5555-555555555555"]
	if solo.MatchMethods != model.CRMBehaviorMatchEmail {
		t.Fatalf("solo profile = %+v", solo)
	}
	if result.Contacts != 2 || result.Companies != 1 || result.EmailOnlyContacts != 1 ||
		result.IdentifiedKeys != 7 || result.UnmatchedKeys != 2 {
		t.Fatalf("result = %+v", result)
	}
}
