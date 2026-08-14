package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCRMContactUpdateRefreshesLinkedSupportIdentityAndRecordsEmailChange(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	if err := db.Exec(`CREATE TABLE crm_activities (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
		workspace_id TEXT NOT NULL,
		activity_type TEXT NOT NULL,
		contact_id TEXT,
		company_id TEXT,
		deal_id TEXT,
		owner_member_id TEXT,
		subject TEXT,
		body TEXT,
		occurred_at DATETIME NOT NULL,
		metadata TEXT NOT NULL DEFAULT '{}',
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create crm activities table: %v", err)
	}

	const (
		workspaceID = "ws-contact-sync"
		userID      = "user-contact-sync"
		memberID    = "member-contact-sync"
	)
	seedUser(t, db, userID, "owner@example.com", "Owner User", "hash")
	seedWorkspace(t, db, workspaceID, "Workspace", "workspace-contact-sync", userID)

	oldEmail := "old@example.com"
	oldPhone := "+1 555 0100"
	contact := &model.CRMContact{
		WorkspaceID:    workspaceID,
		DisplayID:      "CON-1",
		FirstName:      "Old",
		LastName:       strPtr("Name"),
		Email:          &oldEmail,
		Phone:          &oldPhone,
		LifecycleStage: model.CRMLifecycleLead,
		LeadStatus:     model.CRMLeadStatusOpen,
	}
	contactRepo := repository.NewCRMContactRepository(db)
	if err := contactRepo.Create(ctx, contact); err != nil {
		t.Fatalf("create contact: %v", err)
	}

	conversationRepo := repository.NewSupportConversationRepository(db)
	conversation := &model.SupportConversation{
		WorkspaceID:   workspaceID,
		Subject:       "Linked conversation",
		Status:        model.SupportConversationStatusOpen,
		CustomerName:  strPtr("Stale Name"),
		CustomerEmail: strPtr("stale@example.com"),
		CustomerPhone: strPtr("stale phone"),
		CRMContactID:  &contact.ID,
	}
	if err := conversationRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	activityRepo := repository.NewCRMActivityRepository(db)
	svc := NewCRMContactService(contactRepo).
		SetIdentitySync(activityRepo, nil)
	firstName := "Current"
	lastName := "Customer"
	newEmail := "new@example.com"
	newPhone := "+1 555 0199"
	if _, err := svc.UpdateWithActor(ctx, contact.ID, model.UpdateCRMContactRequest{
		FirstName: &firstName,
		LastName:  &lastName,
		Email:     &newEmail,
		Phone:     &newPhone,
	}, userID, memberID); err != nil {
		t.Fatalf("update contact: %v", err)
	}

	updatedConversation, err := conversationRepo.GetByID(ctx, workspaceID, conversation.ID, "", "owner")
	if err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	if updatedConversation == nil || derefString(updatedConversation.CustomerName) != "Current Customer" {
		t.Fatalf("customer_name = %v, want Current Customer", updatedConversation)
	}
	if derefString(updatedConversation.CustomerEmail) != newEmail {
		t.Fatalf("customer_email = %q, want %q", derefString(updatedConversation.CustomerEmail), newEmail)
	}
	if derefString(updatedConversation.CustomerPhone) != newPhone {
		t.Fatalf("customer_phone = %q, want %q", derefString(updatedConversation.CustomerPhone), newPhone)
	}

	activities, total, err := activityRepo.List(ctx, workspaceID, model.CRMActivityListFilters{
		ContactID: &contact.ID,
	}, model.PMPagination{Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("list activities: %v", err)
	}
	if total != 1 || len(activities) != 1 {
		t.Fatalf("activities = %d/%d, want 1", len(activities), total)
	}
	activity := activities[0]
	if derefString(activity.Subject) != "Email changed" || derefString(activity.Body) != "old@example.com → new@example.com" {
		t.Fatalf("activity = %q / %q", derefString(activity.Subject), derefString(activity.Body))
	}
	if activity.OwnerMemberID == nil || *activity.OwnerMemberID != memberID {
		t.Fatalf("owner_member_id = %v, want %q", activity.OwnerMemberID, memberID)
	}
	if activity.Metadata["actor_user_id"] != userID || activity.Metadata["immutable"] != true {
		t.Fatalf("metadata = %#v", activity.Metadata)
	}
	if activity.OccurredAt.After(time.Now().UTC().Add(time.Second)) {
		t.Fatalf("occurred_at is in the future: %v", activity.OccurredAt)
	}
	if err := activityRepo.Delete(ctx, activity.ID); err == nil {
		t.Fatal("expected immutable email-change activity deletion to fail")
	}
}

func TestLinkingCRMContactImmediatelyUsesCRMIdentityInSupport(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	const workspaceID = "ws-contact-link"

	email := "linked@example.com"
	phone := "+1 555 0142"
	contact := &model.CRMContact{
		WorkspaceID:    workspaceID,
		DisplayID:      "CON-1",
		FirstName:      "Linked",
		LastName:       strPtr("Customer"),
		Email:          &email,
		Phone:          &phone,
		LifecycleStage: model.CRMLifecycleLead,
		LeadStatus:     model.CRMLeadStatusOpen,
	}
	contactRepo := repository.NewCRMContactRepository(db)
	if err := contactRepo.Create(ctx, contact); err != nil {
		t.Fatalf("create contact: %v", err)
	}

	conversationRepo := repository.NewSupportConversationRepository(db)
	conversation := &model.SupportConversation{
		WorkspaceID:   workspaceID,
		Subject:       "Unlinked lead",
		Status:        model.SupportConversationStatusOpen,
		CustomerName:  strPtr("Anonymous visitor"),
		CustomerEmail: strPtr("temporary@example.com"),
	}
	if err := conversationRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	svc := &SupportInboxService{
		conversationRepo: conversationRepo,
		assocRepo:        repository.NewCRMAssociationRepository(db),
		contactRepo:      contactRepo,
	}
	linked, err := svc.UpdateConversationCRMContact(
		ctx, workspaceID, conversation.ID, &contact.ID, "user-linking-contact",
	)
	if err != nil {
		t.Fatalf("link contact: %v", err)
	}
	if derefString(linked.CustomerName) != "Linked Customer" {
		t.Fatalf("customer_name = %q, want Linked Customer", derefString(linked.CustomerName))
	}
	if derefString(linked.CustomerEmail) != email {
		t.Fatalf("customer_email = %q, want %q", derefString(linked.CustomerEmail), email)
	}
	if derefString(linked.CustomerPhone) != phone {
		t.Fatalf("customer_phone = %q, want %q", derefString(linked.CustomerPhone), phone)
	}

	persisted, err := conversationRepo.GetByID(ctx, workspaceID, conversation.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("get linked conversation: %v", err)
	}
	if persisted == nil || persisted.CRMContactID == nil || *persisted.CRMContactID != contact.ID {
		t.Fatalf("crm_contact_id = %v, want %q", persisted, contact.ID)
	}
	if derefString(persisted.CustomerEmail) != email {
		t.Fatalf("persisted customer_email = %q, want %q", derefString(persisted.CustomerEmail), email)
	}
}
