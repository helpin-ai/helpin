package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCompanyDomainFromEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  string
	}{
		{name: "organization domain", email: "jane@acme.com", want: "acme.com"},
		{name: "uppercase is normalized", email: "Jane@ACME.com", want: "acme.com"},
		{name: "surrounding space is trimmed", email: "  jane@acme.com  ", want: "acme.com"},
		{name: "free provider is rejected", email: "jane@gmail.com", want: ""},
		{name: "other free provider is rejected", email: "jane@outlook.com", want: ""},
		{name: "disposable provider is rejected", email: "jane@mailinator.com", want: ""},
		{name: "malformed address", email: "not-an-email", want: ""},
		{name: "empty address", email: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := companyDomainFromEmail(tt.email); got != tt.want {
				t.Errorf("companyDomainFromEmail(%q) = %q, want %q", tt.email, got, tt.want)
			}
		})
	}
}

func TestMatchOrCreateCRMCompanyIdentityFallsBackToEmailDomain(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	const workspaceID = "ws-email-domain-company"
	seedWorkspace(t, db, workspaceID, "Email Domain Company", "email-domain-company", "user-123")

	companyRepo := repository.NewCRMCompanyRepository(db)
	domain := "acme.example"
	existing := &model.CRMCompany{
		WorkspaceID: workspaceID,
		DisplayID:   "COM-1",
		Name:        "Acme",
		Domain:      &domain,
	}
	if err := companyRepo.Create(ctx, existing); err != nil {
		t.Fatalf("create existing company: %v", err)
	}

	svc := &SupportInboxService{}

	t.Run("identified contact matches company by domain", func(t *testing.T) {
		companyID, method, err := svc.matchOrCreateCRMCompanyIdentityWithMethodTx(ctx, companyRepo, workspaceID, model.WidgetIdentityPayload{
			Email: "jane@acme.example",
		})
		if err != nil {
			t.Fatalf("matchOrCreateCRMCompanyIdentityWithMethodTx: %v", err)
		}
		if companyID == nil || *companyID != existing.ID {
			t.Fatalf("company id = %v, want %q", companyID, existing.ID)
		}
		if method != model.CompanyMatchMethodEmailDomain {
			t.Fatalf("match method = %q, want %q", method, model.CompanyMatchMethodEmailDomain)
		}
	})

	t.Run("free provider domain resolves nothing", func(t *testing.T) {
		companyID, method, err := svc.matchOrCreateCRMCompanyIdentityWithMethodTx(ctx, companyRepo, workspaceID, model.WidgetIdentityPayload{
			Email: "jane@gmail.com",
		})
		if err != nil {
			t.Fatalf("matchOrCreateCRMCompanyIdentityWithMethodTx: %v", err)
		}
		if companyID != nil {
			t.Fatalf("company id = %v, want nil", companyID)
		}
		if method != "" {
			t.Fatalf("match method = %q, want empty", method)
		}
	})

	t.Run("unknown domain never creates a company", func(t *testing.T) {
		companyID, _, err := svc.matchOrCreateCRMCompanyIdentityWithMethodTx(ctx, companyRepo, workspaceID, model.WidgetIdentityPayload{
			Email: "jane@unknown.example",
		})
		if err != nil {
			t.Fatalf("matchOrCreateCRMCompanyIdentityWithMethodTx: %v", err)
		}
		if companyID != nil {
			t.Fatalf("company id = %v, want nil", companyID)
		}
		companies, _, err := companyRepo.List(ctx, workspaceID, model.CRMCompanyListFilters{}, model.PMPagination{Page: 1, PerPage: 10})
		if err != nil {
			t.Fatalf("list companies: %v", err)
		}
		if len(companies) != 1 {
			t.Fatalf("len(companies) = %d, want 1 — domain inference must never create", len(companies))
		}
	})

	t.Run("declared company still reports declared provenance", func(t *testing.T) {
		companyID, method, err := svc.matchOrCreateCRMCompanyIdentityWithMethodTx(ctx, companyRepo, workspaceID, model.WidgetIdentityPayload{
			Email:   "jane@acme.example",
			Company: model.JSONB{"name": "Acme", "domain": domain},
		})
		if err != nil {
			t.Fatalf("matchOrCreateCRMCompanyIdentityWithMethodTx: %v", err)
		}
		if companyID == nil || *companyID != existing.ID {
			t.Fatalf("company id = %v, want %q", companyID, existing.ID)
		}
		if method != model.CompanyMatchMethodDeclared {
			t.Fatalf("match method = %q, want %q", method, model.CompanyMatchMethodDeclared)
		}
	})
}

func TestNormalizeVisitorAnonymousID(t *testing.T) {
	long := make([]byte, maxVisitorAnonymousIDLength+1)
	for i := range long {
		long[i] = 'a'
	}

	tests := []struct {
		name string
		raw  string
		want *string
	}{
		{name: "empty is dropped", raw: "", want: nil},
		{name: "whitespace only is dropped", raw: "   ", want: nil},
		{name: "over-long value is dropped", raw: string(long), want: nil},
		{name: "value is trimmed", raw: "  anon-1  ", want: ptrToStr("anon-1")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeVisitorAnonymousID(tt.raw)
			switch {
			case tt.want == nil && got != nil:
				t.Fatalf("NormalizeVisitorAnonymousID(%q) = %q, want nil", tt.raw, *got)
			case tt.want != nil && got == nil:
				t.Fatalf("NormalizeVisitorAnonymousID(%q) = nil, want %q", tt.raw, *tt.want)
			case tt.want != nil && *got != *tt.want:
				t.Fatalf("NormalizeVisitorAnonymousID(%q) = %q, want %q", tt.raw, *got, *tt.want)
			}
		})
	}
}
