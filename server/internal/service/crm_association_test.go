package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

func TestListByObjectEnrichedIncludesInferredCompanyContactAssociations(t *testing.T) {
	db := newAssociationsTestDB(t)
	svc := NewCRMAssociationService(repository.NewCRMAssociationRepository(db))

	seedCRMCompany(t, db, "company-1", "ws-1", "Acme Corp", "CO-1")
	seedCRMContact(t, db, "contact-1", "ws-1", "Jane Doe", "C-1")
	seedCRMDeal(t, db, "deal-1", "ws-1", "Expansion", "D-1")
	seedCRMAssociation(t, db, "assoc-company-contact", "ws-1", model.CRMObjectCompany, "company-1", model.CRMObjectContact, "contact-1")
	seedCRMAssociation(t, db, "assoc-contact-deal", "ws-1", model.CRMObjectContact, "contact-1", model.CRMObjectDeal, "deal-1")

	assocs, err := svc.ListByObjectEnriched(context.Background(), "ws-1", model.CRMObjectCompany, "company-1")
	if err != nil {
		t.Fatalf("ListByObjectEnriched returned error: %v", err)
	}
	if len(assocs) != 2 {
		t.Fatalf("expected direct contact plus inferred deal, got %d rows", len(assocs))
	}

	var directContact, inferredDeal *model.CRMAssociationEnriched
	for i := range assocs {
		assoc := &assocs[i]
		otherType, otherID := otherAssociationSide(assoc.CRMAssociation, model.CRMObjectCompany, "company-1")
		switch {
		case otherType == model.CRMObjectContact && otherID == "contact-1":
			directContact = assoc
		case otherType == model.CRMObjectDeal && otherID == "deal-1":
			inferredDeal = assoc
		}
	}

	if directContact == nil || directContact.Inferred {
		t.Fatalf("expected direct contact association, got %+v", assocs)
	}
	if inferredDeal == nil || !inferredDeal.Inferred {
		t.Fatalf("expected inferred deal association, got %+v", assocs)
	}
	if inferredDeal.ID != "" {
		t.Fatalf("expected inferred association to have no id, got %q", inferredDeal.ID)
	}
	if inferredDeal.ContextLabel == nil || *inferredDeal.ContextLabel != "via Jane Doe" {
		t.Fatalf("expected inferred context label 'via Jane Doe', got %+v", inferredDeal.ContextLabel)
	}
}

func TestListByObjectEnrichedSkipsInferredDuplicatesForCompanyContactAssociations(t *testing.T) {
	db := newAssociationsTestDB(t)
	svc := NewCRMAssociationService(repository.NewCRMAssociationRepository(db))

	seedCRMCompany(t, db, "company-1", "ws-1", "Acme Corp", "CO-1")
	seedCRMContact(t, db, "contact-1", "ws-1", "Jane Doe", "C-1")
	seedAssociationTask(t, db, "task-1", "ws-1", 101, "Follow up", false)
	seedCRMAssociation(t, db, "assoc-company-contact", "ws-1", model.CRMObjectCompany, "company-1", model.CRMObjectContact, "contact-1")
	seedCRMAssociation(t, db, "assoc-company-task", "ws-1", model.CRMObjectCompany, "company-1", model.CRMObjectTask, "task-1")
	seedCRMAssociation(t, db, "assoc-contact-task", "ws-1", model.CRMObjectContact, "contact-1", model.CRMObjectTask, "task-1")

	assocs, err := svc.ListByObjectEnriched(context.Background(), "ws-1", model.CRMObjectCompany, "company-1")
	if err != nil {
		t.Fatalf("ListByObjectEnriched returned error: %v", err)
	}
	if len(assocs) != 2 {
		t.Fatalf("expected direct contact and direct task only, got %d rows", len(assocs))
	}

	var taskAssoc *model.CRMAssociationEnriched
	for i := range assocs {
		assoc := &assocs[i]
		otherType, otherID := otherAssociationSide(assoc.CRMAssociation, model.CRMObjectCompany, "company-1")
		if otherType == model.CRMObjectTask && otherID == "task-1" {
			taskAssoc = assoc
			break
		}
	}

	if taskAssoc == nil {
		t.Fatalf("expected task association in response, got %+v", assocs)
	}
	if taskAssoc.Inferred {
		t.Fatalf("expected direct task association to win over inferred duplicate, got %+v", taskAssoc)
	}
	if taskAssoc.ID != "assoc-company-task" {
		t.Fatalf("expected direct task association id %q, got %q", "assoc-company-task", taskAssoc.ID)
	}
}

func seedCRMDeal(t *testing.T, db *gorm.DB, id, workspaceID, name, displayID string) {
	t.Helper()
	if err := db.Exec(
		`INSERT INTO crm_deals (id, workspace_id, name, display_id)
		 VALUES (?, ?, ?, ?)`,
		id, workspaceID, name, displayID,
	).Error; err != nil {
		t.Fatalf("seed deal: %v", err)
	}
}
