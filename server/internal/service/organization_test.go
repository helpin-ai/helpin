package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestOrganizationServiceCreateUsesUniqueSlugWhenRequestedSlugExists(t *testing.T) {
	db := newTestDB(t)
	orgRepo := repository.NewOrganizationRepository(db)
	svc := NewOrganizationService(orgRepo)
	ctx := context.Background()

	first, err := svc.Create(ctx, model.CreateOrganizationRequest{
		Name: "Waqar's Organization",
		Slug: "waqar-s-organization",
	}, "user-1")
	if err != nil {
		t.Fatalf("first Create() error = %v", err)
	}
	if first.Slug != "waqar-s-organization" {
		t.Fatalf("first slug = %q, want waqar-s-organization", first.Slug)
	}

	second, err := svc.Create(ctx, model.CreateOrganizationRequest{
		Name: "Waqar's Organization",
		Slug: "waqar-s-organization",
	}, "user-2")
	if err != nil {
		t.Fatalf("second Create() error = %v", err)
	}
	if second.Slug != "waqar-s-organization-2" {
		t.Fatalf("second slug = %q, want waqar-s-organization-2", second.Slug)
	}
	if second.Role != model.RoleOwner {
		t.Fatalf("second role = %q, want owner", second.Role)
	}
}
