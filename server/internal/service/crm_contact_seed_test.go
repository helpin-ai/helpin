package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCRMContactServiceSeedCreatesRequestedContacts(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	const (
		workspaceID = "ws-seed"
		ownerUserID = "user-owner"
	)

	seedUser(t, db, ownerUserID, "owner@example.com", "Owner User", "hash")
	seedWorkspace(t, db, workspaceID, "Workspace", "workspace", ownerUserID)

	repo := repository.NewCRMContactRepository(db)
	svc := NewCRMContactService(repo)

	result, err := svc.Seed(ctx, model.SeedCRMContactsRequest{
		WorkspaceID: workspaceID,
		Count:       500,
	})
	if err != nil {
		t.Fatalf("Seed error = %v", err)
	}
	if result.Created != 500 {
		t.Fatalf("result.Created = %d, want 500", result.Created)
	}

	contacts, total, err := repo.List(ctx, workspaceID, model.CRMContactListFilters{}, model.PMPagination{Page: 1, PerPage: 3})
	if err != nil {
		t.Fatalf("List error = %v", err)
	}
	if total != 500 {
		t.Fatalf("total = %d, want 500", total)
	}
	if len(contacts) != 3 {
		t.Fatalf("len(contacts) = %d, want 3", len(contacts))
	}
	if contacts[0].DisplayID != "CON-1" {
		t.Fatalf("first display_id = %q, want %q", contacts[0].DisplayID, "CON-1")
	}
	if contacts[1].Source == nil || *contacts[1].Source != "seed" {
		t.Fatalf("second source = %v, want seed", contacts[1].Source)
	}
}

func TestCRMContactServiceSeedDefaultsAndAppends(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	const (
		workspaceID = "ws-seed-default"
		ownerUserID = "user-owner"
	)

	seedUser(t, db, ownerUserID, "owner@example.com", "Owner User", "hash")
	seedWorkspace(t, db, workspaceID, "Workspace", "workspace", ownerUserID)

	repo := repository.NewCRMContactRepository(db)
	svc := NewCRMContactService(repo)

	existing := &model.CRMContact{
		WorkspaceID:    workspaceID,
		DisplayID:      "CON-1",
		FirstName:      "Existing",
		LifecycleStage: model.CRMLifecycleLead,
		LeadStatus:     model.CRMLeadStatusOpen,
	}
	if err := repo.Create(ctx, existing); err != nil {
		t.Fatalf("seed existing contact: %v", err)
	}

	result, err := svc.Seed(ctx, model.SeedCRMContactsRequest{WorkspaceID: workspaceID})
	if err != nil {
		t.Fatalf("Seed error = %v", err)
	}
	if result.Created != 500 {
		t.Fatalf("result.Created = %d, want 500", result.Created)
	}

	contacts, total, err := repo.List(ctx, workspaceID, model.CRMContactListFilters{}, model.PMPagination{Page: 1, PerPage: 500})
	if err != nil {
		t.Fatalf("List error = %v", err)
	}
	if total != 501 {
		t.Fatalf("total = %d, want 501", total)
	}

	found := false
	for _, contact := range contacts {
		if contact.DisplayID == "CON-2" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected seeded contact with display_id CON-2")
	}
}

func TestCRMContactServiceSeedRejectsOversizedBatch(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	svc := NewCRMContactService(repository.NewCRMContactRepository(db))
	if _, err := svc.Seed(ctx, model.SeedCRMContactsRequest{WorkspaceID: "ws-seed", Count: 2001}); err == nil {
		t.Fatal("expected oversize seed request to fail")
	}
}
