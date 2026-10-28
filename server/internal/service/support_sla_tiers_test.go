package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestSupportSLAPolicyMatchesCompany(t *testing.T) {
	gold, silver := "tier-gold", "tier-silver"
	conversation := model.SupportConversation{Priority: "medium", Channel: "email"}
	acme := &repository.SupportSLACompany{ID: "acme", TierID: &gold}
	untiered := &repository.SupportSLACompany{ID: "beta"}
	for _, tc := range []struct {
		name    string
		policy  model.SupportSLAPolicy
		company *repository.SupportSLACompany
		want    bool
	}{
		{"tier matches", model.SupportSLAPolicy{CompanyTierIDs: model.DocsStringArray{gold}}, acme, true},
		{"other tier", model.SupportSLAPolicy{CompanyTierIDs: model.DocsStringArray{silver}}, acme, false},
		{"no tier matches untiered company", model.SupportSLAPolicy{CompanyTierIDs: model.DocsStringArray{model.SupportSLANoTier}}, untiered, true},
		{"no tier matches no company", model.SupportSLAPolicy{CompanyTierIDs: model.DocsStringArray{model.SupportSLANoTier}}, nil, true},
		{"no tier skips tiered company", model.SupportSLAPolicy{CompanyTierIDs: model.DocsStringArray{model.SupportSLANoTier}}, acme, false},
		{"tier skips no company", model.SupportSLAPolicy{CompanyTierIDs: model.DocsStringArray{gold}}, nil, false},
		{"company matches", model.SupportSLAPolicy{CompanyIDs: model.DocsStringArray{"acme"}}, acme, true},
		{"other company", model.SupportSLAPolicy{CompanyIDs: model.DocsStringArray{"beta"}}, acme, false},
		{"company skips no company", model.SupportSLAPolicy{CompanyIDs: model.DocsStringArray{"acme"}}, nil, false},
		{"no company conditions", model.SupportSLAPolicy{}, nil, true},
	} {
		tc.policy.Enabled = true
		if got := supportSLAPolicyMatches(tc.policy, conversation, tc.company); got != tc.want {
			t.Errorf("%s: got %v", tc.name, got)
		}
	}
	if supportSLACompanyKey(nil) == supportSLACompanyKey(untiered) || supportSLACompanyKey(untiered) == supportSLACompanyKey(acme) {
		t.Fatal("company keys must differ by company and tier")
	}
}

func TestSupportSLAPolicyStart(t *testing.T) {
	created := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	now := created.Add(3 * time.Hour)
	tierChanged := created.Add(2 * time.Hour)
	gold := "gold"
	company := &repository.SupportSLACompany{ID: "acme", TierID: &gold, TierChangedAt: &tierChanged}
	key := supportSLACompanyKey(company)
	conversation := model.SupportConversation{CreatedAt: created}
	tierPolicy := model.SupportSLAPolicy{CompanyTierIDs: model.DocsStringArray{gold}}
	original := created.Add(time.Minute)

	for _, tc := range []struct {
		name     string
		policy   model.SupportSLAPolicy
		previous *model.SupportConversationSLARecord
		want     *time.Time
	}{
		{"priority change keeps the original start", tierPolicy, &model.SupportConversationSLARecord{CompanyKey: key, TargetsFrom: &original}, &original},
		{"company or tier change counts from now", tierPolicy, &model.SupportConversationSLARecord{CompanyKey: "company:none"}, &now},
		{"record from before tiers keeps the original start", tierPolicy, &model.SupportConversationSLARecord{TargetsFrom: &original}, &original},
		{"reattaching counts from now", model.SupportSLAPolicy{}, &model.SupportConversationSLARecord{CompanyKey: key, DetachedAt: &original}, &now},
		{"first tier policy counts from the tier change", tierPolicy, nil, &tierChanged},
		{"first default policy counts from the start", model.SupportSLAPolicy{}, nil, nil},
	} {
		_, from := supportSLAPolicyStart(tc.policy, conversation, company, tc.previous, key, now)
		if (from == nil) != (tc.want == nil) || (from != nil && !from.Equal(*tc.want)) {
			t.Errorf("%s: from %v, want %v", tc.name, from, tc.want)
		}
	}
	// A tier set before the conversation started does not move its start.
	before := created.Add(-time.Hour)
	early := &repository.SupportSLACompany{ID: "acme", TierID: &gold, TierChangedAt: &before}
	if _, from := supportSLAPolicyStart(tierPolicy, conversation, early, nil, key, now); from != nil {
		t.Fatalf("old tier moved the start to %v", from)
	}
}

type slaTierFixture struct {
	sla         *SupportSLAService
	db          *gorm.DB
	workspaceID string
	userID      string
	start       time.Time
	displayID   int
}

func newSLATierFixture(t *testing.T) *slaTierFixture {
	t.Helper()
	sla, db, workspaceID, userID, start := newSLATestService(t)
	seedWorkspaceMember(t, db, uuid.NewString(), workspaceID, userID, "agent@example.com", "Ada Agent", model.RoleOwner)
	sla.inbox.SetWorkspaceRepo(repository.NewWorkspaceRepository(db))
	return &slaTierFixture{sla: sla, db: db, workspaceID: workspaceID, userID: userID, start: start}
}

func (f *slaTierFixture) company(t *testing.T, name string) string {
	t.Helper()
	company := model.CRMCompany{ID: uuid.NewString(), WorkspaceID: f.workspaceID, DisplayID: name, Name: name}
	if err := f.db.Create(&company).Error; err != nil {
		t.Fatal(err)
	}
	return company.ID
}

// conversation creates an open conversation with a customer message at
// created, for companyID ("" for none).
func (f *slaTierFixture) conversation(t *testing.T, companyID string, created time.Time) string {
	t.Helper()
	f.displayID++
	conversation := model.SupportConversation{ID: uuid.NewString(), WorkspaceID: f.workspaceID, DisplayID: f.displayID, Subject: "Help",
		Status: model.SupportConversationStatusOpen, Priority: "medium", Channel: "email", CreatedAt: created}
	if companyID != "" {
		conversation.CRMCompanyID = &companyID
	}
	if err := f.db.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&model.SupportMessage{ID: uuid.NewString(), WorkspaceID: f.workspaceID, ConversationID: conversation.ID, SenderType: "customer",
		MessageType: "reply", Content: "Hi", CreatedAt: created, UpdatedAt: created}).Error; err != nil {
		t.Fatal(err)
	}
	return conversation.ID
}

// runQueue processes the queue as the worker would at at.
func (f *slaTierFixture) runQueue(t *testing.T, at time.Time) {
	t.Helper()
	f.sla.now = func() time.Time { return at }
	if _, err := f.sla.ProcessQueue(context.Background(), 50); err != nil {
		t.Fatal(err)
	}
}

func (f *slaTierFixture) queued(t *testing.T, conversationID string) bool {
	t.Helper()
	var count int64
	if err := f.db.Model(&model.SupportSLAWork{}).Where("conversation_id = ?", conversationID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count > 0
}

func (f *slaTierFixture) policy(t *testing.T, req model.SupportSLAPolicyRequest) *model.SupportSLAPolicy {
	t.Helper()
	policy, err := f.sla.CreatePolicy(context.Background(), f.workspaceID, f.userID, req)
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

func (f *slaTierFixture) tier(t *testing.T, name string) *model.SupportSLATier {
	t.Helper()
	tier, err := f.sla.CreateTier(context.Background(), f.workspaceID, model.SupportSLATierRequest{Name: name})
	if err != nil {
		t.Fatal(err)
	}
	return tier
}

func (f *slaTierFixture) setTier(t *testing.T, at time.Time, tierID *string, companyIDs ...string) {
	t.Helper()
	f.sla.now = func() time.Time { return at }
	if _, err := f.sla.SetCompanyTier(context.Background(), f.workspaceID, model.SetSupportSLATierRequest{CompanyIDs: companyIDs, TierID: tierID}); err != nil {
		t.Fatal(err)
	}
}

func TestSupportSLATiersManage(t *testing.T) {
	f := newSLATierFixture(t)
	ctx := context.Background()
	gold := f.tier(t, "  Gold   plus ")
	silver := f.tier(t, "Silver")
	if gold.Name != "Gold plus" || gold.Position != 0 || silver.Position != 1 {
		t.Fatalf("tiers %+v %+v", gold, silver)
	}
	for name, req := range map[string]model.SupportSLATierRequest{
		"duplicate": {Name: "gold PLUS"},
		"reserved":  {Name: "None"},
		"empty":     {Name: "  "},
		"too long":  {Name: strings.Repeat("x", 61)},
	} {
		if _, err := f.sla.CreateTier(ctx, f.workspaceID, req); !errors.Is(err, ErrSupportSLAPolicyInvalid) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if renamed, err := f.sla.UpdateTier(ctx, f.workspaceID, gold.ID, model.SupportSLATierRequest{Name: "Gold", Description: "Enterprise"}); err != nil || renamed.Name != "Gold" {
		t.Fatalf("rename: %+v %v", renamed, err)
	}
	if _, err := f.sla.UpdateTier(ctx, f.workspaceID, silver.ID, model.SupportSLATierRequest{Name: "gold"}); !errors.Is(err, ErrSupportSLAPolicyInvalid) {
		t.Fatalf("rename onto another tier: %v", err)
	}
	if tiers, err := f.sla.ReorderTiers(ctx, f.workspaceID, []string{silver.ID, gold.ID}); err != nil || tiers[0].ID != silver.ID {
		t.Fatalf("reorder: %+v %v", tiers, err)
	}
	if _, err := f.sla.ReorderTiers(ctx, f.workspaceID, []string{silver.ID}); !errors.Is(err, repository.ErrSupportSLATierNotFound) {
		t.Fatalf("partial reorder: %v", err)
	}

	// Policies validate tiers and companies.
	acme := f.company(t, "Acme")
	for name, req := range map[string]model.SupportSLAPolicyRequest{
		"unknown tier":    {Name: "X", FirstResponseMinutes: intPtr(10), CompanyTierIDs: []string{uuid.NewString()}},
		"unknown company": {Name: "X", FirstResponseMinutes: intPtr(10), CompanyIDs: []string{uuid.NewString()}},
	} {
		if _, err := f.sla.CreatePolicy(ctx, f.workspaceID, f.userID, req); !errors.Is(err, ErrSupportSLAPolicyInvalid) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	policy := f.policy(t, model.SupportSLAPolicyRequest{Name: "Gold", FirstResponseMinutes: intPtr(30),
		CompanyTierIDs: []string{gold.ID, model.SupportSLANoTier}, CompanyIDs: []string{acme}})
	if len(policy.Companies) != 1 || policy.Companies[0].Name != "Acme" {
		t.Fatalf("policy companies %+v", policy.Companies)
	}

	f.setTier(t, f.start, &gold.ID, acme)
	tiers, err := f.sla.ListTiers(ctx, f.workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	for _, tier := range tiers {
		if tier.ID == gold.ID && (tier.CompanyCount != 1 || tier.PolicyCount != 1) {
			t.Fatalf("gold usage %+v", tier)
		}
	}

	// Deleting a tier clears it from companies and policy conditions.
	if err := f.sla.DeleteTier(ctx, f.workspaceID, gold.ID); err != nil {
		t.Fatal(err)
	}
	company, err := f.sla.repo.GetCompany(ctx, f.workspaceID, acme)
	if err != nil || company.TierID != nil || company.TierChangedAt == nil {
		t.Fatalf("company after tier delete: %+v %v", company, err)
	}
	policies, err := f.sla.ListPolicies(ctx, f.workspaceID)
	if err != nil || len(policies) != 1 || strings.Join(policies[0].CompanyTierIDs, ",") != model.SupportSLANoTier || policies[0].ConditionsChangedAt == nil {
		t.Fatalf("policy after tier delete: %+v %v", policies, err)
	}
	if err := f.sla.DeleteTier(ctx, f.workspaceID, gold.ID); !errors.Is(err, repository.ErrSupportSLATierNotFound) {
		t.Fatalf("second delete: %v", err)
	}
}

func TestSupportSLASetCompanyTierValidates(t *testing.T) {
	f := newSLATierFixture(t)
	ctx := context.Background()
	acme := f.company(t, "Acme")
	gold := f.tier(t, "Gold")
	tooMany := make([]string, model.SupportSLASetTierLimit+1)
	for i := range tooMany {
		tooMany[i] = uuid.NewString()
	}
	unknownTier := uuid.NewString()
	for name, tc := range map[string]struct {
		req  model.SetSupportSLATierRequest
		want error
	}{
		"no companies":    {model.SetSupportSLATierRequest{TierID: &gold.ID}, ErrSupportSLAPolicyInvalid},
		"too many":        {model.SetSupportSLATierRequest{CompanyIDs: tooMany, TierID: &gold.ID}, ErrSupportSLAPolicyInvalid},
		"unknown company": {model.SetSupportSLATierRequest{CompanyIDs: []string{acme, uuid.NewString()}, TierID: &gold.ID}, ErrSupportSLACompanyNotFound},
		"unknown tier":    {model.SetSupportSLATierRequest{CompanyIDs: []string{acme}, TierID: &unknownTier}, repository.ErrSupportSLATierNotFound},
	} {
		if _, err := f.sla.SetCompanyTier(ctx, f.workspaceID, tc.req); !errors.Is(err, tc.want) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if updated, err := f.sla.SetCompanyTier(ctx, f.workspaceID, model.SetSupportSLATierRequest{CompanyIDs: []string{acme, acme}, TierID: &gold.ID}); err != nil || updated != 1 {
		t.Fatalf("set: %d %v", updated, err)
	}
	// Setting the same tier again changes nothing.
	if updated, err := f.sla.SetCompanyTier(ctx, f.workspaceID, model.SetSupportSLATierRequest{CompanyIDs: []string{acme}, TierID: &gold.ID}); err != nil || updated != 0 {
		t.Fatalf("set again: %d %v", updated, err)
	}
	if updated, err := f.sla.SetCompanyTier(ctx, f.workspaceID, model.SetSupportSLATierRequest{CompanyIDs: []string{acme}}); err != nil || updated != 1 {
		t.Fatalf("clear: %d %v", updated, err)
	}
	f.sla.SetEntitlementService(deniedSLAEntitlements{})
	var entitlementErr *EntitlementError
	if _, err := f.sla.SetCompanyTier(ctx, f.workspaceID, model.SetSupportSLATierRequest{CompanyIDs: []string{acme}, TierID: &gold.ID}); !errors.As(err, &entitlementErr) {
		t.Fatalf("entitlement: %v", err)
	}
}

func TestSupportSLATierChangeCountsFromTheChange(t *testing.T) {
	f := newSLATierFixture(t)
	gold := f.tier(t, "Gold")
	f.policy(t, model.SupportSLAPolicyRequest{Name: "Gold", FirstResponseMinutes: intPtr(30), CompanyTierIDs: []string{gold.ID}})
	f.policy(t, model.SupportSLAPolicyRequest{Name: "Standard", FirstResponseMinutes: intPtr(480)})
	acme := f.company(t, "Acme")
	created := f.start.Add(time.Minute)
	conversation := f.conversation(t, acme, created)
	processSLAAt(t, f.sla, f.workspaceID, conversation, created)
	if record := slaRecord(t, f.db, conversation); record.PolicyName != "Standard" || !record.FirstResponseDueAt.Equal(created.Add(8*time.Hour)) {
		t.Fatalf("standard record %+v", record)
	}

	// Three hours in, Acme moves to Gold: its 30 minutes count from the change.
	upgraded := created.Add(3 * time.Hour)
	f.setTier(t, upgraded, &gold.ID, acme)
	if !f.queued(t, conversation) {
		t.Fatal("tier change did not queue the company's conversation")
	}
	f.runQueue(t, upgraded)
	record := slaRecord(t, f.db, conversation)
	if record.PolicyName != "Gold" || record.Breached || !record.FirstResponseDueAt.Equal(upgraded.Add(30*time.Minute)) {
		t.Fatalf("gold record %+v", record)
	}

	// Raising the priority keeps that start: same company, same tier.
	if err := f.db.Model(&model.SupportConversation{}).Where("id = ?", conversation).Update("priority", "urgent").Error; err != nil {
		t.Fatal(err)
	}
	processSLAAt(t, f.sla, f.workspaceID, conversation, upgraded.Add(10*time.Minute))
	if record := slaRecord(t, f.db, conversation); !record.FirstResponseDueAt.Equal(upgraded.Add(30 * time.Minute)) {
		t.Fatalf("priority change moved the start: %+v", record)
	}

	// A conversation of another company without a tier stays on Standard.
	other := f.conversation(t, f.company(t, "Beta"), created)
	processSLAAt(t, f.sla, f.workspaceID, other, upgraded)
	if record := slaRecord(t, f.db, other); record.PolicyName != "Standard" {
		t.Fatalf("other company %+v", record)
	}
}

func TestSupportSLAFirstTierPolicyCountsFromTierChange(t *testing.T) {
	f := newSLATierFixture(t)
	gold := f.tier(t, "Gold")
	f.policy(t, model.SupportSLAPolicyRequest{Name: "Gold", FirstResponseMinutes: intPtr(30), CompanyTierIDs: []string{gold.ID}})
	acme := f.company(t, "Acme")
	created := f.start.Add(time.Minute)
	conversation := f.conversation(t, acme, created)
	processSLAAt(t, f.sla, f.workspaceID, conversation, created)
	var count int64
	f.db.Model(&model.SupportConversationSLARecord{}).Where("conversation_id = ?", conversation).Count(&count)
	if count != 0 {
		t.Fatal("no policy applies yet, but a record was written")
	}
	upgraded := created.Add(2 * time.Hour)
	f.setTier(t, upgraded, &gold.ID, acme)
	f.runQueue(t, upgraded.Add(time.Minute))
	if record := slaRecord(t, f.db, conversation); record.Breached || !record.FirstResponseDueAt.Equal(upgraded.Add(30*time.Minute)) {
		t.Fatalf("first tier policy %+v", record)
	}
}

func TestSupportSLACompanyChangeCountsFromTheChange(t *testing.T) {
	f := newSLATierFixture(t)
	gold := f.tier(t, "Gold")
	f.policy(t, model.SupportSLAPolicyRequest{Name: "Gold", FirstResponseMinutes: intPtr(30), CompanyTierIDs: []string{gold.ID}})
	f.policy(t, model.SupportSLAPolicyRequest{Name: "Standard", FirstResponseMinutes: intPtr(480)})
	acme := f.company(t, "Acme")
	f.setTier(t, f.start, &gold.ID, acme)
	created := f.start.Add(time.Minute)
	conversation := f.conversation(t, "", created)
	processSLAAt(t, f.sla, f.workspaceID, conversation, created)
	if record := slaRecord(t, f.db, conversation); record.PolicyName != "Standard" {
		t.Fatalf("without company %+v", record)
	}
	linked := created.Add(2 * time.Hour)
	if err := f.db.Model(&model.SupportConversation{}).Where("id = ?", conversation).Update("crm_company_id", acme).Error; err != nil {
		t.Fatal(err)
	}
	processSLAAt(t, f.sla, f.workspaceID, conversation, linked)
	if record := slaRecord(t, f.db, conversation); record.PolicyName != "Gold" || record.Breached || !record.FirstResponseDueAt.Equal(linked.Add(30*time.Minute)) {
		t.Fatalf("after linking the company %+v", record)
	}
}

func TestSupportSLACustomCompanySLA(t *testing.T) {
	f := newSLATierFixture(t)
	ctx := context.Background()
	gold := f.tier(t, "Gold")
	f.policy(t, model.SupportSLAPolicyRequest{Name: "Gold urgent", Priorities: []string{"urgent"}, FirstResponseMinutes: intPtr(15), CompanyTierIDs: []string{gold.ID}})
	f.policy(t, model.SupportSLAPolicyRequest{Name: "Silver", FirstResponseMinutes: intPtr(60), CompanyTierIDs: []string{model.SupportSLANoTier}})
	f.policy(t, model.SupportSLAPolicyRequest{Name: "Standard", FirstResponseMinutes: intPtr(480), ResolutionMinutes: intPtr(2880)})
	disabled := false
	off := f.policy(t, model.SupportSLAPolicyRequest{Name: "Off", FirstResponseMinutes: intPtr(5)})
	if _, err := f.sla.UpdatePolicy(ctx, f.workspaceID, off.ID, model.SupportSLAPolicyRequest{Name: "Off", Enabled: &disabled, FirstResponseMinutes: intPtr(5)}); err != nil {
		t.Fatal(err)
	}
	acme := f.company(t, "Acme")
	f.setTier(t, f.start, &gold.ID, acme)

	view, err := f.sla.CompanySLA(ctx, f.workspaceID, acme)
	if err != nil || view.TierName == nil || *view.TierName != "Gold" || view.HasCustom || len(view.Policies) != 2 ||
		view.Policies[0].Name != "Gold urgent" || view.Policies[1].Name != "Standard" {
		t.Fatalf("company SLA %+v %v", view, err)
	}

	later := f.start.Add(time.Hour)
	f.sla.now = func() time.Time { return later }
	policies, err := f.sla.CustomizeCompanySLA(ctx, f.workspaceID, f.userID, acme)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, policy := range policies {
		names = append(names, policy.Name)
	}
	if strings.Join(names, ",") != "Acme · Gold urgent,Acme · Standard,Gold urgent,Silver,Standard,Off" {
		t.Fatalf("policies after customize: %v", names)
	}
	custom := policies[0]
	if len(custom.CompanyTierIDs) != 0 || strings.Join(custom.CompanyIDs, ",") != acme || !custom.ActiveSince.Equal(later) ||
		strings.Join(custom.Priorities, ",") != "urgent" || *custom.FirstResponseMinutes != 15 || custom.Companies[0].Name != "Acme" {
		t.Fatalf("custom policy %+v", custom)
	}
	if _, err := f.sla.CustomizeCompanySLA(ctx, f.workspaceID, f.userID, acme); !errors.Is(err, ErrSupportSLACustomExists) {
		t.Fatalf("second customize: %v", err)
	}
	if view, err := f.sla.CompanySLA(ctx, f.workspaceID, acme); err != nil || !view.HasCustom || !view.Policies[0].Custom || len(view.Policies) != 4 {
		t.Fatalf("company SLA after customize %+v %v", view, err)
	}

	// New conversations of Acme get the custom policy.
	conversation := f.conversation(t, acme, later.Add(time.Minute))
	processSLAAt(t, f.sla, f.workspaceID, conversation, later.Add(time.Minute))
	if record := slaRecord(t, f.db, conversation); record.PolicyName != "Acme · Standard" {
		t.Fatalf("custom policy not applied: %+v", record)
	}

	if err := f.sla.RemoveCompanySLA(ctx, f.workspaceID, acme); err != nil {
		t.Fatal(err)
	}
	if err := f.sla.RemoveCompanySLA(ctx, f.workspaceID, acme); !errors.Is(err, repository.ErrSupportSLAPolicyNotFound) {
		t.Fatalf("second remove: %v", err)
	}
	f.runQueue(t, later.Add(2*time.Minute))
	if record := slaRecord(t, f.db, conversation); record.PolicyName != "Standard" {
		t.Fatalf("after removing the custom SLA: %+v", record)
	}
	if _, err := f.sla.CompanySLA(ctx, f.workspaceID, uuid.NewString()); !errors.Is(err, ErrSupportSLACompanyNotFound) {
		t.Fatalf("unknown company: %v", err)
	}
	empty := newSLATierFixture(t)
	if _, err := empty.sla.CustomizeCompanySLA(ctx, empty.workspaceID, empty.userID, empty.company(t, "Solo")); !errors.Is(err, ErrSupportSLAPolicyInvalid) {
		t.Fatalf("customize without policies: %v", err)
	}
}

func TestSupportSLATierCommands(t *testing.T) {
	f := newSLATierFixture(t)
	ctx := context.Background()
	gold := f.tier(t, "Gold")
	acme := f.company(t, "Acme")
	commands := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	commands.SetSupportSLAService(f.sla)
	def, ok := commands.Definition("support.set_company_sla_tier")
	if !ok || !def.Mutating || len(def.RequiredPermissionsAll) != 1 || def.RequiredPermissionsAll[0] != authorization.PermSupportAdmin {
		t.Fatalf("set tier definition %+v", def)
	}
	member := model.InternalCommandContext{WorkspaceID: f.workspaceID, ActorID: f.userID, ActorRole: model.RoleAdmin}
	run := func(meta model.InternalCommandContext, input string) (map[string]any, error) {
		raw, err := commands.executeSupportSetCompanySLATier(ctx, meta, json.RawMessage(input))
		if err != nil {
			return nil, err
		}
		var out map[string]any
		return out, json.Unmarshal(raw, &out)
	}

	out, err := run(member, `{"company_id":"`+acme+`","tier":"gold"}`)
	if err != nil || out["changed"] != true || out["tier"].(map[string]any)["id"] != gold.ID {
		t.Fatalf("set by name: %+v %v", out, err)
	}
	if out, err := run(member, `{"company_id":"`+acme+`","tier":"`+gold.ID+`"}`); err != nil || out["changed"] != false {
		t.Fatalf("set again by id: %+v %v", out, err)
	}
	if _, err := run(member, `{"company_id":"`+acme+`","tier":"Platinum"}`); err == nil || !strings.Contains(err.Error(), "tiers are: Gold") {
		t.Fatalf("unknown tier: %v", err)
	}
	if _, err := run(member, `{"company_id":"`+uuid.NewString()+`","tier":"Gold"}`); err == nil || !strings.Contains(err.Error(), "company") {
		t.Fatalf("unknown company: %v", err)
	}
	if _, err := run(model.InternalCommandContext{WorkspaceID: f.workspaceID}, `{"company_id":"`+acme+`"}`); err == nil {
		t.Fatal("set without a workspace member")
	}
	if out, err := run(member, `{"company_id":"`+acme+`","tier":null}`); err != nil || out["changed"] != true || out["tier"] != nil {
		t.Fatalf("clear: %+v %v", out, err)
	}
	raw, err := commands.executeSupportListSLATiers(ctx, member, nil)
	if err != nil || !strings.Contains(string(raw), `"name":"Gold"`) {
		t.Fatalf("list: %s %v", raw, err)
	}
}

func TestSupportSLALateCompanyLinkCountsFromTheLink(t *testing.T) {
	f := newSLATierFixture(t)
	gold := f.tier(t, "Gold")
	f.policy(t, model.SupportSLAPolicyRequest{Name: "Gold", FirstResponseMinutes: intPtr(30), CompanyTierIDs: []string{gold.ID}})
	acme := f.company(t, "Acme")
	f.setTier(t, f.start, &gold.ID, acme)
	created := f.start.Add(time.Minute)
	conversation := f.conversation(t, "", created)
	processSLAAt(t, f.sla, f.workspaceID, conversation, created)

	// Hours later a teammate links Acme, Gold for months: no instant breach.
	linked := created.Add(3 * time.Hour)
	// SQLite has no triggers: record the change as Postgres would.
	mustExec(t, f.db, `UPDATE support_conversations SET crm_company_id = ?, crm_company_changed_at = ? WHERE id = ?`, acme, linked, conversation)
	processSLAAt(t, f.sla, f.workspaceID, conversation, linked)
	if record := slaRecord(t, f.db, conversation); record.Breached || !record.FirstResponseDueAt.Equal(linked.Add(30*time.Minute)) {
		t.Fatalf("late link %+v", record)
	}
}

func TestSupportSLADeletingAPolicysOnlyTierTurnsItOff(t *testing.T) {
	f := newSLATierFixture(t)
	ctx := context.Background()
	gold := f.tier(t, "Gold")
	only := f.policy(t, model.SupportSLAPolicyRequest{Name: "Gold only", FirstResponseMinutes: intPtr(30), CompanyTierIDs: []string{gold.ID}})
	if err := f.sla.DeleteTier(ctx, f.workspaceID, gold.ID); err != nil {
		t.Fatal(err)
	}
	policies, err := f.sla.ListPolicies(ctx, f.workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if policies[0].ID != only.ID || policies[0].Enabled || len(policies[0].CompanyTierIDs) != 0 {
		t.Fatalf("policy left matching everyone: %+v", policies[0])
	}
}

func TestSupportSLADisabledCustomSLAStillCounts(t *testing.T) {
	f := newSLATierFixture(t)
	ctx := context.Background()
	acme := f.company(t, "Acme")
	f.policy(t, model.SupportSLAPolicyRequest{Name: "Standard", FirstResponseMinutes: intPtr(480)})
	policies, err := f.sla.CustomizeCompanySLA(ctx, f.workspaceID, f.userID, acme)
	if err != nil {
		t.Fatal(err)
	}
	disabled := false
	custom := policies[0]
	if _, err := f.sla.UpdatePolicy(ctx, f.workspaceID, custom.ID, model.SupportSLAPolicyRequest{Name: custom.Name, Enabled: &disabled,
		FirstResponseMinutes: custom.FirstResponseMinutes, CompanyIDs: []string{acme}}); err != nil {
		t.Fatal(err)
	}
	view, err := f.sla.CompanySLA(ctx, f.workspaceID, acme)
	if err != nil || !view.HasCustom || len(view.Policies) != 1 || view.Policies[0].Custom {
		t.Fatalf("company SLA with a disabled custom policy %+v %v", view, err)
	}
	if err := f.sla.RemoveCompanySLA(ctx, f.workspaceID, acme); err != nil {
		t.Fatal(err)
	}
}

func TestSupportSLARejectsMalformedIDs(t *testing.T) {
	f := newSLATierFixture(t)
	ctx := context.Background()
	acme := f.company(t, "Acme")
	notATier := "gold"
	if _, err := f.sla.CompanySLA(ctx, f.workspaceID, "acme"); !errors.Is(err, ErrSupportSLACompanyNotFound) {
		t.Fatalf("company SLA: %v", err)
	}
	if _, err := f.sla.SetCompanyTier(ctx, f.workspaceID, model.SetSupportSLATierRequest{CompanyIDs: []string{"acme"}}); !errors.Is(err, ErrSupportSLACompanyNotFound) {
		t.Fatalf("set tier company: %v", err)
	}
	if _, err := f.sla.SetCompanyTier(ctx, f.workspaceID, model.SetSupportSLATierRequest{CompanyIDs: []string{acme}, TierID: &notATier}); !errors.Is(err, repository.ErrSupportSLATierNotFound) {
		t.Fatalf("set tier tier: %v", err)
	}
	if err := f.sla.DeleteTier(ctx, f.workspaceID, notATier); !errors.Is(err, repository.ErrSupportSLATierNotFound) {
		t.Fatalf("delete tier: %v", err)
	}
	if _, err := f.sla.UpdateTier(ctx, f.workspaceID, notATier, model.SupportSLATierRequest{Name: "Gold"}); !errors.Is(err, repository.ErrSupportSLATierNotFound) {
		t.Fatalf("update tier: %v", err)
	}
	if _, err := f.sla.CreatePolicy(ctx, f.workspaceID, f.userID, model.SupportSLAPolicyRequest{Name: "X", FirstResponseMinutes: intPtr(5), CompanyIDs: []string{"acme"}}); !errors.Is(err, ErrSupportSLAPolicyInvalid) {
		t.Fatalf("policy company: %v", err)
	}
}

func TestApplySupportSLAWarnings(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	at := func(minutes int) *time.Time { return timePtr(now.Add(time.Duration(minutes) * time.Minute)) }
	fifteen := 15
	for _, tc := range []struct {
		name     string
		record   model.SupportConversationSLARecord
		previous *model.SupportConversationSLARecord
		warn     *int
		want     []string
		wake     *time.Time
	}{
		{name: "before the warning, wake at it", record: model.SupportConversationSLARecord{FirstResponseState: model.SupportSLAStatePending, FirstResponseDueAt: at(40), WakeAt: at(40)}, warn: &fifteen, wake: at(25)},
		{name: "inside the window, warn", record: model.SupportConversationSLARecord{FirstResponseState: model.SupportSLAStatePending, FirstResponseDueAt: at(10), WakeAt: at(10)}, warn: &fifteen, want: []string{model.SupportSLAMetricFirstResponse}, wake: at(10)},
		{name: "already warned for this due time", record: model.SupportConversationSLARecord{FirstResponseState: model.SupportSLAStatePending, FirstResponseDueAt: at(10)}, previous: &model.SupportConversationSLARecord{FirstResponseWarnedFor: at(10)}, warn: &fifteen},
		{name: "a new next-response cycle warns again", record: model.SupportConversationSLARecord{NextResponseState: model.SupportSLAStatePending, NextResponseDueAt: at(10)}, previous: &model.SupportConversationSLARecord{NextResponseWarnedFor: at(-60)}, warn: &fifteen, want: []string{model.SupportSLAMetricNextResponse}},
		{name: "too late to be useful, record silently", record: model.SupportConversationSLARecord{ResolutionState: model.SupportSLAStatePending, ResolutionDueAt: at(2)}, warn: &fifteen},
		{name: "paused targets never warn", record: model.SupportConversationSLARecord{ResolutionState: model.SupportSLAStatePaused, ResolutionDueAt: at(5)}, warn: &fifteen},
		{name: "overdue targets never warn", record: model.SupportConversationSLARecord{FirstResponseState: model.SupportSLAStatePending, FirstResponseDueAt: at(-1)}, warn: &fifteen},
		{name: "warnings off", record: model.SupportConversationSLARecord{FirstResponseState: model.SupportSLAStatePending, FirstResponseDueAt: at(10)}},
	} {
		record := tc.record
		got := applySupportSLAWarnings(&record, tc.previous, tc.warn, newSupportSLAClock(model.SupportInboxSettings{}, false), now, false)
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("%s: warned %v, want %v", tc.name, got, tc.want)
		}
		if tc.wake != nil && (record.WakeAt == nil || !record.WakeAt.Equal(*tc.wake)) {
			t.Errorf("%s: wake %v, want %v", tc.name, record.WakeAt, tc.wake)
		}
	}
	// Recording a late warning keeps it from firing later.
	record := model.SupportConversationSLARecord{ResolutionState: model.SupportSLAStatePending, ResolutionDueAt: at(2)}
	calendar := newSupportSLAClock(model.SupportInboxSettings{}, false)
	applySupportSLAWarnings(&record, nil, &fifteen, calendar, now, false)
	if record.ResolutionWarnedFor == nil || !record.ResolutionWarnedFor.Equal(*at(2)) {
		t.Fatalf("late warning not recorded: %+v", record.ResolutionWarnedFor)
	}

	// A pause moves the resolution due time; it does not warn twice.
	moved := model.SupportConversationSLARecord{ResolutionState: model.SupportSLAStatePending, ResolutionDueAt: at(10)}
	if got := applySupportSLAWarnings(&moved, &model.SupportConversationSLARecord{ResolutionWarnedFor: at(7)}, &fifteen, calendar, now, false); len(got) != 0 {
		t.Fatalf("warned again after a pause: %v", got)
	}
	// New targets after a switch may warn again.
	moved = model.SupportConversationSLARecord{ResolutionState: model.SupportSLAStatePending, ResolutionDueAt: at(10)}
	if got := applySupportSLAWarnings(&moved, &model.SupportConversationSLARecord{ResolutionWarnedFor: at(7)}, &fifteen, calendar, now, true); strings.Join(got, ",") != model.SupportSLAMetricResolution {
		t.Fatalf("switched targets did not warn: %v", got)
	}
	// A lead time as long as the conversation's own target never warns.
	short := model.SupportConversationSLARecord{NextResponseState: model.SupportSLAStatePending, NextResponseDueAt: at(10), TargetNextResponseMinutes: &fifteen}
	if got := applySupportSLAWarnings(&short, nil, &fifteen, calendar, now, false); len(got) != 0 || short.NextResponseWarnedFor != nil {
		t.Fatalf("warned on a target shorter than the lead: %v", got)
	}
}

func TestSupportSLAWarningsCountBusinessHours(t *testing.T) {
	week := map[string]model.BusinessHoursDay{}
	for _, day := range []string{"mon", "tue", "wed", "thu", "fri"} {
		week[day] = model.BusinessHoursDay{Enabled: true, Start: "09:00", End: "17:00"}
	}
	clock := newSupportSLAClock(model.SupportInboxSettings{BusinessHoursEnabled: true, BusinessHoursTimezone: "UTC", BusinessHoursSchedule: week}, true)
	monday := time.Date(2026, 10, 5, 9, 10, 0, 0, time.UTC)
	if got := clock.Sub(monday, 30*time.Minute); !got.Equal(time.Date(2026, 10, 2, 16, 40, 0, 0, time.UTC)) {
		t.Fatalf("Sub across the weekend = %v", got)
	}
	if got := clock.Sub(monday, 5*time.Minute); !got.Equal(monday.Add(-5 * time.Minute)) {
		t.Fatalf("Sub inside a day = %v", got)
	}
	thirty := 30
	// The worker wakes on Friday at 16:40, half an hour of business time
	// before Monday 09:10.
	record := model.SupportConversationSLARecord{FirstResponseState: model.SupportSLAStatePending, FirstResponseDueAt: &monday}
	friday := time.Date(2026, 10, 2, 16, 41, 0, 0, time.UTC)
	if got := applySupportSLAWarnings(&record, nil, &thirty, clock, time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC), false); len(got) != 0 ||
		!record.WakeAt.Equal(time.Date(2026, 10, 2, 16, 40, 0, 0, time.UTC)) {
		t.Fatalf("business-hours wake %v %v", got, record.WakeAt)
	}
	record.WakeAt = nil
	if got := applySupportSLAWarnings(&record, nil, &thirty, clock, friday, false); strings.Join(got, ",") != model.SupportSLAMetricFirstResponse {
		t.Fatalf("business-hours warning %v", got)
	}
}

func TestSupportSLAWarningPolicyValidation(t *testing.T) {
	f := newSLATierFixture(t)
	ctx := context.Background()
	for name, req := range map[string]model.SupportSLAPolicyRequest{
		"warning not before the target": {Name: "X", FirstResponseMinutes: intPtr(30), ResolutionMinutes: intPtr(600), WarnBeforeMinutes: intPtr(30)},
		"negative warning":              {Name: "X", FirstResponseMinutes: intPtr(30), WarnBeforeMinutes: intPtr(-5)},
		"unknown teammate":              {Name: "X", FirstResponseMinutes: intPtr(30), NotifyUserIDs: []string{uuid.NewString()}},
		"unknown assignee":              {Name: "X", FirstResponseMinutes: intPtr(30), AssignUnassignedUserID: strPtr(uuid.NewString())},
	} {
		if _, err := f.sla.CreatePolicy(ctx, f.workspaceID, f.userID, req); !errors.Is(err, ErrSupportSLAPolicyInvalid) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	policy := f.policy(t, model.SupportSLAPolicyRequest{Name: "Warn", FirstResponseMinutes: intPtr(60), WarnBeforeMinutes: intPtr(15),
		NotifyUserIDs: []string{f.userID}, NotifyCSOwner: true, AssignUnassignedUserID: &f.userID})
	if *policy.WarnBeforeMinutes != 15 || strings.Join(policy.NotifyUserIDs, ",") != f.userID || !policy.NotifyCSOwner || *policy.AssignUnassignedUserID != f.userID {
		t.Fatalf("policy alerts %+v", policy)
	}
	// Zero turns warnings off.
	updated, err := f.sla.UpdatePolicy(ctx, f.workspaceID, policy.ID, model.SupportSLAPolicyRequest{Name: "Warn", FirstResponseMinutes: intPtr(60), WarnBeforeMinutes: intPtr(0)})
	if err != nil || updated.WarnBeforeMinutes != nil || len(updated.NotifyUserIDs) != 0 || updated.AssignUnassignedUserID != nil {
		t.Fatalf("cleared alerts %+v %v", updated, err)
	}
}

func TestSupportSLAWarnsOnceAndAssigns(t *testing.T) {
	f := newSLATierFixture(t)
	f.policy(t, model.SupportSLAPolicyRequest{Name: "Standard", FirstResponseMinutes: intPtr(60), WarnBeforeMinutes: intPtr(15), AssignUnassignedUserID: &f.userID})
	created := f.start.Add(time.Minute)
	conversation := f.conversation(t, "", created)
	processSLAAt(t, f.sla, f.workspaceID, conversation, created)
	record := slaRecord(t, f.db, conversation)
	if !record.WakeAt.Equal(created.Add(45*time.Minute)) || record.FirstResponseWarnedFor != nil {
		t.Fatalf("before the warning %+v", record)
	}

	// The worker wakes at the warning time.
	f.runQueue(t, created.Add(46*time.Minute))
	record = slaRecord(t, f.db, conversation)
	if record.FirstResponseWarnedFor == nil || !record.FirstResponseWarnedFor.Equal(created.Add(time.Hour)) || !record.WakeAt.Equal(created.Add(time.Hour)) {
		t.Fatalf("after the warning %+v", record)
	}
	var assigned model.SupportConversation
	if err := f.db.Where("id = ?", conversation).Take(&assigned).Error; err != nil || assigned.AssignedUserID == nil || *assigned.AssignedUserID != f.userID {
		t.Fatalf("unassigned conversation not taken: %+v %v", assigned.AssignedUserID, err)
	}

	// Later evaluations before the breach do not warn again.
	processSLAAt(t, f.sla, f.workspaceID, conversation, created.Add(50*time.Minute))
	if again := slaRecord(t, f.db, conversation); !again.FirstResponseWarnedFor.Equal(*record.FirstResponseWarnedFor) {
		t.Fatalf("warned again %+v", again)
	}
}

func TestSupportSLAAlertRecipients(t *testing.T) {
	f := newSLATierFixture(t)
	ctx := context.Background()
	other, ownerMember := uuid.NewString(), uuid.NewString()
	seedUser(t, f.db, other, "cs@example.com", "Cass Success", "hash")
	seedWorkspaceMember(t, f.db, ownerMember, f.workspaceID, other, "cs@example.com", "Cass Success", model.RoleMember)
	assignee := f.userID
	conversation := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: f.workspaceID, AssignedUserID: &assignee}
	policy := &model.SupportSLAPolicy{NotifyUserIDs: model.DocsStringArray{f.userID}, NotifyCSOwner: true}
	company := &repository.SupportSLACompany{ID: uuid.NewString(), CSOwnerMemberID: &ownerMember}
	got := f.sla.alertRecipients(ctx, conversation, policy, company)
	if strings.Join(got, ",") != assignee+","+other {
		t.Fatalf("recipients %v", got)
	}
	if got := f.sla.alertRecipients(ctx, conversation, nil, company); strings.Join(got, ",") != assignee {
		t.Fatalf("without a policy %v", got)
	}
}

func TestSupportSLAPolicyDropsTeammatesWhoLostAccess(t *testing.T) {
	f := newSLATierFixture(t)
	ctx := context.Background()
	other, member := uuid.NewString(), uuid.NewString()
	seedUser(t, f.db, other, "leaver@example.com", "Lee Leaver", "hash")
	seedWorkspaceMember(t, f.db, member, f.workspaceID, other, "leaver@example.com", "Lee Leaver", model.RoleAdmin)
	policy := f.policy(t, model.SupportSLAPolicyRequest{Name: "Warn", FirstResponseMinutes: intPtr(60), WarnBeforeMinutes: intPtr(10),
		NotifyUserIDs: []string{f.userID, other}, AssignUnassignedUserID: &other})
	mustExec(t, f.db, `UPDATE workspace_members SET status = 'inactive' WHERE id = ?`, member)

	// Saving the policy again keeps working and drops the former teammate.
	updated, err := f.sla.UpdatePolicy(ctx, f.workspaceID, policy.ID, model.SupportSLAPolicyRequest{Name: "Warn", FirstResponseMinutes: intPtr(60), WarnBeforeMinutes: intPtr(10),
		NotifyUserIDs: []string{f.userID, other}, AssignUnassignedUserID: &other})
	if err != nil || strings.Join(updated.NotifyUserIDs, ",") != f.userID || updated.AssignUnassignedUserID != nil {
		t.Fatalf("stale teammates kept: %+v %v", updated, err)
	}
	teammates, err := f.sla.SLATeammates(ctx, f.workspaceID)
	if err != nil || len(teammates) != 1 || *teammates[0].UserID != f.userID {
		t.Fatalf("teammates %+v %v", teammates, err)
	}
}

func TestSupportSLAWarningKeepsAFreshAssignment(t *testing.T) {
	f := newSLATierFixture(t)
	other, member := uuid.NewString(), uuid.NewString()
	seedUser(t, f.db, other, "taker@example.com", "Tia Taker", "hash")
	seedWorkspaceMember(t, f.db, member, f.workspaceID, other, "taker@example.com", "Tia Taker", model.RoleAdmin)
	policy := f.policy(t, model.SupportSLAPolicyRequest{Name: "Warn", FirstResponseMinutes: intPtr(60), WarnBeforeMinutes: intPtr(10), AssignUnassignedUserID: &f.userID})
	conversation := f.conversation(t, "", f.start.Add(time.Minute))
	// Loaded as unassigned, then a teammate takes it before the warning assigns.
	loaded := &model.SupportConversation{ID: conversation, WorkspaceID: f.workspaceID}
	mustExec(t, f.db, `UPDATE support_conversations SET assigned_user_id = ? WHERE id = ?`, other, conversation)
	f.sla.assignBeforeBreach(context.Background(), loaded, policy)
	var current model.SupportConversation
	if err := f.db.Where("id = ?", conversation).Take(&current).Error; err != nil || *current.AssignedUserID != other || *loaded.AssignedUserID != other {
		t.Fatalf("fresh assignment overwritten: %v %v", current.AssignedUserID, err)
	}
}

// importedConversation creates an Intercom conversation from long ago with
// a customer message, and a teammate reply when answered.
func (f *slaTierFixture) importedConversation(t *testing.T, status string, created time.Time, answered bool) string {
	t.Helper()
	f.displayID++
	conversation := model.SupportConversation{ID: uuid.NewString(), WorkspaceID: f.workspaceID, DisplayID: f.displayID, Subject: "Imported",
		Status: status, Priority: "medium", Channel: "widget", Source: model.SupportImportSourceIntercom, CreatedAt: created}
	if status == model.SupportConversationStatusResolved {
		conversation.ResolvedAt = timePtr(created.Add(time.Hour))
	}
	if err := f.db.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	add := func(sender string, at time.Time) {
		if err := f.db.Create(&model.SupportMessage{ID: uuid.NewString(), WorkspaceID: f.workspaceID, ConversationID: conversation.ID, SenderType: sender,
			MessageType: "reply", Content: "imported", CreatedAt: at, UpdatedAt: at}).Error; err != nil {
			t.Fatal(err)
		}
	}
	add("customer", created)
	if answered {
		add("user", created.Add(30*time.Minute))
		add("customer", created.Add(45*time.Minute))
	}
	return conversation.ID
}

func TestSupportSLAImportedConversationsCountFromCutover(t *testing.T) {
	f := newSLATierFixture(t)
	f.policy(t, model.SupportSLAPolicyRequest{Name: "Standard", FirstResponseMinutes: intPtr(60), NextResponseMinutes: intPtr(120), ResolutionMinutes: intPtr(2880)})
	longAgo := f.start.Add(-90 * 24 * time.Hour)
	cutover := f.start.Add(time.Hour)

	// Open and never answered in Intercom: every target counts from cutover.
	waiting := f.importedConversation(t, model.SupportConversationStatusOpen, longAgo, false)
	processSLAAt(t, f.sla, f.workspaceID, waiting, cutover)
	record := slaRecord(t, f.db, waiting)
	if record.Breached || record.FirstResponseState != model.SupportSLAStatePending || !record.FirstResponseDueAt.Equal(cutover.Add(time.Hour)) ||
		!record.ResolutionDueAt.Equal(cutover.Add(48*time.Hour)) || !record.TargetsFrom.Equal(cutover) {
		t.Fatalf("unanswered import %+v", record)
	}

	// Answered in Intercom, customer waiting again: no first-response target,
	// and the open next-response cycle counts from cutover.
	answered := f.importedConversation(t, model.SupportConversationStatusOpen, longAgo, true)
	processSLAAt(t, f.sla, f.workspaceID, answered, cutover)
	record = slaRecord(t, f.db, answered)
	if record.FirstResponseState != "" || record.NextResponseState != model.SupportSLAStatePending || record.NextResponseBreaches != 0 ||
		!record.NextResponseDueAt.Equal(cutover.Add(2*time.Hour)) || record.Breached {
		t.Fatalf("answered import %+v", record)
	}
	// Later evaluations keep the cutover.
	processSLAAt(t, f.sla, f.workspaceID, answered, cutover.Add(time.Minute))
	if again := slaRecord(t, f.db, answered); !again.TargetsFrom.Equal(cutover) || again.Breached {
		t.Fatalf("cutover moved %+v", again)
	}

	// Closed imports stay out of SLA entirely.
	resolved := f.importedConversation(t, model.SupportConversationStatusResolved, longAgo, true)
	processSLAAt(t, f.sla, f.workspaceID, resolved, cutover)
	var count int64
	f.db.Model(&model.SupportConversationSLARecord{}).Where("conversation_id = ?", resolved).Count(&count)
	if count != 0 {
		t.Fatal("a closed import got an SLA record")
	}

	// The customer writes again and reopens it: targets count from then.
	reopened := cutover.Add(24 * time.Hour)
	if err := f.db.Create(&model.SupportMessage{ID: uuid.NewString(), WorkspaceID: f.workspaceID, ConversationID: resolved, SenderType: "customer",
		MessageType: "reply", Content: "Back again", CreatedAt: reopened, UpdatedAt: reopened}).Error; err != nil {
		t.Fatal(err)
	}
	mustExec(t, f.db, `UPDATE support_conversations SET status = 'open' WHERE id = ?`, resolved)
	processSLAAt(t, f.sla, f.workspaceID, resolved, reopened.Add(time.Minute))
	record = slaRecord(t, f.db, resolved)
	if record.FirstResponseState != "" || record.NextResponseState != model.SupportSLAStatePending ||
		!record.NextResponseDueAt.Equal(reopened.Add(time.Minute+2*time.Hour)) || record.Breached {
		t.Fatalf("reopened import %+v", record)
	}
}
