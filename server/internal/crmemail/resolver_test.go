package crmemail

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestResolver_UsesExactCaseInsensitiveLookupAndAvoidsDuplicateCreate(t *testing.T) {
	db := setupCRMEmailTestDB(t)
	contactRepo := repository.NewCRMContactRepository(db)
	resolver := NewResolver(contactRepo)
	ctx := context.Background()

	email := "Alice@Example.com"
	if err := contactRepo.Create(ctx, &model.CRMContact{
		WorkspaceID:    "ws-1",
		DisplayID:      "CON-1",
		FirstName:      "Alice",
		Email:          &email,
		LifecycleStage: model.CRMLifecycleSubscriber,
		LeadStatus:     model.CRMLeadStatusNew,
	}); err != nil {
		t.Fatalf("seed contact: %v", err)
	}

	settings := model.DefaultEmailSyncSettings()
	settings.RecordCreationMode = "always"

	result, err := resolver.Resolve(ctx, ResolveInput{
		WorkspaceID: "ws-1",
		Direction:   model.CRMEmailDirectionOutbound,
		Settings:    &settings,
		SelfEmails:  []string{"owner@example.com"},
		From:        Participant{Email: "owner@example.com", Role: model.CRMEmailParticipantRoleFrom},
		To:          []Participant{{Email: "alice@example.com", Role: model.CRMEmailParticipantRoleTo}},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(result.ContactIDs) != 1 {
		t.Fatalf("contact_ids = %v, want single existing contact", result.ContactIDs)
	}

	var count int64
	if err := db.Table("crm_contacts").Where("workspace_id = ?", "ws-1").Count(&count).Error; err != nil {
		t.Fatalf("count contacts: %v", err)
	}
	if count != 1 {
		t.Fatalf("contact count = %d, want 1", count)
	}
}

func TestResolver_RecordCreationModesAndBlockedPrefixes(t *testing.T) {
	db := setupCRMEmailTestDB(t)
	contactRepo := repository.NewCRMContactRepository(db)
	resolver := NewResolver(contactRepo)
	ctx := context.Background()

	blockedPrefixes, _ := json.Marshal([]string{"support"})
	settings := model.DefaultEmailSyncSettings()
	settings.RecordCreationMode = "selective"
	settings.BlockedRecordPrefixes = blockedPrefixes

	inbound, err := resolver.Resolve(ctx, ResolveInput{
		WorkspaceID: "ws-1",
		Direction:   model.CRMEmailDirectionInbound,
		Settings:    &settings,
		SelfEmails:  []string{"owner@example.com"},
		From:        Participant{Email: "buyer@example.com", Role: model.CRMEmailParticipantRoleFrom},
		To:          []Participant{{Email: "owner@example.com", Role: model.CRMEmailParticipantRoleTo}},
	})
	if err != nil {
		t.Fatalf("Resolve inbound: %v", err)
	}
	if len(inbound.ContactIDs) != 0 {
		t.Fatalf("inbound contact_ids = %v, want none in selective mode", inbound.ContactIDs)
	}

	outbound, err := resolver.Resolve(ctx, ResolveInput{
		WorkspaceID: "ws-1",
		Direction:   model.CRMEmailDirectionOutbound,
		Settings:    &settings,
		SelfEmails:  []string{"owner@example.com"},
		From:        Participant{Email: "owner@example.com", Role: model.CRMEmailParticipantRoleFrom},
		To:          []Participant{{Email: "jane.doe+sales@example.com", Role: model.CRMEmailParticipantRoleTo}},
		CC:          []Participant{{Email: "support@example.com", Role: model.CRMEmailParticipantRoleCC}},
	})
	if err != nil {
		t.Fatalf("Resolve outbound: %v", err)
	}
	if len(outbound.ContactIDs) != 1 {
		t.Fatalf("outbound contact_ids = %v, want one created contact", outbound.ContactIDs)
	}
	if outbound.PrimaryContactID == nil {
		t.Fatal("expected primary contact for single outbound participant")
	}

	var created model.CRMContact
	if err := db.Table("crm_contacts").Where("id = ?", outbound.ContactIDs[0]).First(&created).Error; err != nil {
		t.Fatalf("load created contact: %v", err)
	}
	if created.FirstName != "Jane" {
		t.Fatalf("first_name = %q, want Jane", created.FirstName)
	}
	if created.LastName == nil || *created.LastName != "Doe Sales" {
		t.Fatalf("last_name = %v, want Doe Sales", created.LastName)
	}

	var total int64
	if err := db.Table("crm_contacts").Where("workspace_id = ?", "ws-1").Count(&total).Error; err != nil {
		t.Fatalf("count contacts: %v", err)
	}
	if total != 1 {
		t.Fatalf("contact count = %d, want 1 (blocked prefix should not create)", total)
	}
}
