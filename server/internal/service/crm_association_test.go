package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

func TestCRMAssociationServiceScopedDeleteRejectsOtherWorkspace(t *testing.T) {
	db := newAssociationsTestDB(t)
	svc := NewCRMAssociationService(repository.NewCRMAssociationRepository(db))
	seedCRMAssociation(t, db, "assoc-scoped", "ws-2", model.CRMObjectContact, "contact-2", model.CRMObjectCompany, "company-2")
	if err := svc.DeleteScoped(context.Background(), "ws-1", "assoc-scoped"); err == nil || !strings.Contains(err.Error(), "association not found") {
		t.Fatalf("expected scoped not-found error, got %v", err)
	}
	var count int64
	if err := db.Model(&model.CRMAssociation{}).Where("id = ?", "assoc-scoped").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("cross-workspace association was deleted")
	}
}

func TestCRMAssociationServiceProtectsDealCustomerAndRejectsGenericCompanyLink(t *testing.T) {
	db := newAssociationsTestDB(t)
	svc := NewCRMAssociationService(repository.NewCRMAssociationRepository(db))
	customerLabel := model.CRMAssociationLabelDealCustomer
	seedCRMAssociation(t, db, "assoc-customer", "ws-1", model.CRMObjectDeal, "deal-1", model.CRMObjectCompany, "company-1")
	if err := db.Model(&model.CRMAssociation{}).Where("id = ?", "assoc-customer").Update("association_label", customerLabel).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteScoped(context.Background(), "ws-1", "assoc-customer"); !errors.Is(err, ErrDealCustomerAssociationProtected) {
		t.Fatalf("expected protected customer error, got %v", err)
	}
	_, err := svc.Create(context.Background(), model.CreateCRMAssociationRequest{WorkspaceID: "ws-1", FromObjectType: model.CRMObjectDeal, FromObjectID: "deal-1", ToObjectType: model.CRMObjectCompany, ToObjectID: "company-2"})
	if !errors.Is(err, ErrDealCustomerEndpointRequired) {
		t.Fatalf("expected dedicated customer endpoint error, got %v", err)
	}
}

func TestCRMAssociationServiceCanonicalizesDealContactParticipant(t *testing.T) {
	db := newAssociationsTestDB(t)
	svc := NewCRMAssociationService(repository.NewCRMAssociationRepository(db))
	association, err := svc.Create(context.Background(), model.CreateCRMAssociationRequest{WorkspaceID: "ws-1", FromObjectType: model.CRMObjectContact, FromObjectID: "contact-1", ToObjectType: model.CRMObjectDeal, ToObjectID: "deal-1"})
	if err != nil {
		t.Fatal(err)
	}
	if association.FromObjectType != model.CRMObjectDeal || association.FromObjectID != "deal-1" || association.ToObjectType != model.CRMObjectContact || association.ToObjectID != "contact-1" {
		t.Fatalf("association was not canonicalized: %#v", association)
	}
}

func TestCRMAssociationServiceSetsPrimaryContactCompany(t *testing.T) {
	db := newAssociationsTestDB(t)
	svc := NewCRMAssociationService(repository.NewCRMAssociationRepository(db))
	seedCRMAssociation(t, db, "assoc-old", "ws-1", model.CRMObjectContact, "contact-1", model.CRMObjectCompany, "company-1")
	seedCRMAssociation(t, db, "assoc-new", "ws-1", model.CRMObjectContact, "contact-1", model.CRMObjectCompany, "company-2")
	primary := "primary"
	if err := db.Model(&model.CRMAssociation{}).Where("id = ?", "assoc-old").Update("association_label", &primary).Error; err != nil {
		t.Fatal(err)
	}

	assoc, err := svc.SetPrimaryContactCompany(context.Background(), "ws-1", "contact-1", "company-2")
	if err != nil {
		t.Fatalf("set primary company: %v", err)
	}
	if assoc.ID != "assoc-new" || !isPrimaryCompanyAssociationLabel(assoc.AssociationLabel) {
		t.Fatalf("unexpected primary association: %#v", assoc)
	}
	old, err := svc.GetScoped(context.Background(), "ws-1", "assoc-old")
	if err != nil {
		t.Fatal(err)
	}
	if old.AssociationLabel != nil {
		t.Fatalf("old primary label was not cleared: %#v", old)
	}
}

func TestListByObjectEnrichedDoesNotInferCompanyDealsThroughContacts(t *testing.T) {
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
	if len(assocs) != 1 {
		t.Fatalf("expected only the direct contact, got %d rows", len(assocs))
	}

	var directContact *model.CRMAssociationEnriched
	for i := range assocs {
		assoc := &assocs[i]
		otherType, otherID := otherAssociationSide(assoc.CRMAssociation, model.CRMObjectCompany, "company-1")
		if otherType == model.CRMObjectContact && otherID == "contact-1" {
			directContact = assoc
		}
	}

	if directContact == nil || directContact.Inferred {
		t.Fatalf("expected direct contact association, got %+v", assocs)
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

func TestListByObjectEnrichedDoesNotInferDealCompaniesViaContacts(t *testing.T) {
	db := newAssociationsTestDB(t)
	svc := NewCRMAssociationService(repository.NewCRMAssociationRepository(db))

	seedCRMCompany(t, db, "company-1", "ws-1", "Acme Corp", "CO-1")
	seedCRMContact(t, db, "contact-1", "ws-1", "Jane Doe", "C-1")
	seedCRMDeal(t, db, "deal-1", "ws-1", "Expansion", "D-1")
	seedCRMAssociation(t, db, "assoc-deal-contact", "ws-1", model.CRMObjectDeal, "deal-1", model.CRMObjectContact, "contact-1")
	seedCRMAssociation(t, db, "assoc-contact-company", "ws-1", model.CRMObjectContact, "contact-1", model.CRMObjectCompany, "company-1")

	assocs, err := svc.ListByObjectEnriched(context.Background(), "ws-1", model.CRMObjectDeal, "deal-1")
	if err != nil {
		t.Fatalf("ListByObjectEnriched returned error: %v", err)
	}
	if len(assocs) != 1 {
		t.Fatalf("expected only the direct contact, got %d rows", len(assocs))
	}
	if assocs[0].Inferred {
		t.Fatalf("expected direct contact association, got %+v", assocs[0])
	}
}

func TestListByObjectEnrichedPrefersDirectDealCompanyAssociation(t *testing.T) {
	db := newAssociationsTestDB(t)
	svc := NewCRMAssociationService(repository.NewCRMAssociationRepository(db))

	seedCRMCompany(t, db, "company-1", "ws-1", "Acme Corp", "CO-1")
	seedCRMContact(t, db, "contact-1", "ws-1", "Jane Doe", "C-1")
	seedCRMDeal(t, db, "deal-1", "ws-1", "Expansion", "D-1")
	seedCRMAssociation(t, db, "assoc-deal-contact", "ws-1", model.CRMObjectDeal, "deal-1", model.CRMObjectContact, "contact-1")
	seedCRMAssociation(t, db, "assoc-contact-company", "ws-1", model.CRMObjectContact, "contact-1", model.CRMObjectCompany, "company-1")
	seedCRMAssociation(t, db, "assoc-deal-company", "ws-1", model.CRMObjectDeal, "deal-1", model.CRMObjectCompany, "company-1")

	assocs, err := svc.ListByObjectEnriched(context.Background(), "ws-1", model.CRMObjectDeal, "deal-1")
	if err != nil {
		t.Fatalf("ListByObjectEnriched returned error: %v", err)
	}
	if len(assocs) != 2 {
		t.Fatalf("expected direct contact and direct company only, got %d rows", len(assocs))
	}
	for i := range assocs {
		assoc := &assocs[i]
		otherType, otherID := otherAssociationSide(assoc.CRMAssociation, model.CRMObjectDeal, "deal-1")
		if otherType == model.CRMObjectCompany && otherID == "company-1" {
			if assoc.Inferred || assoc.ID != "assoc-deal-company" {
				t.Fatalf("expected direct company association to win, got %+v", assoc)
			}
			return
		}
	}
	t.Fatal("expected company association")
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
