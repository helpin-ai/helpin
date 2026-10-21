package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func storedPortalAccess(t *testing.T, db *gorm.DB, contactID string) *string {
	t.Helper()
	var row struct{ PortalAccess *string }
	if err := db.Table("crm_contacts").Select("portal_access").Where("id = ?", contactID).Take(&row).Error; err != nil {
		t.Fatalf("load portal access: %v", err)
	}
	return row.PortalAccess
}

func TestCRMContactPortalGuardProtectsEmailChanges(t *testing.T) {
	for _, access := range []string{model.PortalAccessAllowed, model.PortalAccessBlocked} {
		t.Run(access, func(t *testing.T) {
			db := newTestDB(t)
			ctx := context.Background()
			workspaceID := uuid.NewString()
			contactID := setPortalContact(t, db, workspaceID, "customer@example.com", portalAccess(access))
			contacts := NewCRMContactService(repository.NewCRMContactRepository(db))

			newEmail := "someone-else@example.com"
			if _, err := contacts.UpdateWithActor(ctx, contactID, model.UpdateCRMContactRequest{Email: &newEmail}, "", ""); !errors.Is(err, repository.ErrPortalProtectedContact) {
				t.Fatalf("CRM editor changed a protected email: %v", err)
			}
			sameEmail := " Customer@Example.com"
			name := "Renamed"
			if _, err := contacts.UpdateWithActor(ctx, contactID, model.UpdateCRMContactRequest{FirstName: &name, Email: &sameEmail}, "", ""); err != nil {
				t.Fatalf("non-email edit of a protected contact: %v", err)
			}
			if _, err := contacts.UpdateWithActor(repository.WithPortalAccessAuthority(ctx), contactID, model.UpdateCRMContactRequest{Email: &newEmail}, "", ""); err != nil {
				t.Fatalf("support admin email change: %v", err)
			}
			if got := derefString(storedPortalAccess(t, db, contactID)); got != access {
				t.Fatalf("portal access changed by a contact update: %q", got)
			}
		})
	}
}

func TestCRMContactPortalGuardProtectsDeletion(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	workspaceID := uuid.NewString()
	blocked := setPortalContact(t, db, workspaceID, "blocked@example.com", portalAccess(model.PortalAccessBlocked))
	contacts := NewCRMContactService(repository.NewCRMContactRepository(db))
	if err := contacts.Delete(ctx, workspaceID, blocked); !errors.Is(err, repository.ErrPortalProtectedContact) {
		t.Fatalf("CRM editor deleted a blocked contact: %v", err)
	}
	// The full anonymizing delete needs the PostgreSQL support schema
	// (support_contact_privacy_postgres_test.go); here it must pass the guard.
	if err := contacts.Delete(repository.WithPortalAccessAuthority(ctx), workspaceID, blocked); errors.Is(err, repository.ErrPortalProtectedContact) {
		t.Fatalf("support admin delete refused: %v", err)
	}
}

// Generic writes never set or overwrite the support admin's decision.
func TestCRMContactGenericWritesLeavePortalAccessAlone(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	workspaceID := uuid.NewString()
	contactRepo := repository.NewCRMContactRepository(db)

	created := &model.CRMContact{ID: uuid.NewString(), WorkspaceID: workspaceID, DisplayID: "C-1", FirstName: "Imported", Email: strPtr("imported@example.com"), PortalAccess: portalAccess(model.PortalAccessAllowed)}
	if err := contactRepo.Create(ctx, created); err != nil {
		t.Fatal(err)
	}
	if got := storedPortalAccess(t, db, created.ID); got != nil {
		t.Fatalf("contact creation set portal access: %q", *got)
	}

	stale, err := contactRepo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.NewCustomerPortalRepository(db).SetContactPortalAccess(ctx, workspaceID, created.ID, portalAccess(model.PortalAccessAllowed)); err != nil {
		t.Fatal(err)
	}
	stale.FirstName = "Updated elsewhere"
	if err := contactRepo.Update(ctx, stale); err != nil {
		t.Fatal(err)
	}
	if got := derefString(storedPortalAccess(t, db, created.ID)); got != model.PortalAccessAllowed {
		t.Fatalf("stale contact update overwrote portal access: %q", got)
	}
}
