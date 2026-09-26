package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestInboundEmailSelectsOnlyUnambiguousContactCompany(t *testing.T) {
	for _, tt := range []struct {
		name                        string
		companies                   int
		reverse, duplicate, foreign bool
		wantSelected                bool
	}{
		{name: "one company", companies: 1, wantSelected: true},
		{name: "reverse association", companies: 1, reverse: true, wantSelected: true},
		{name: "duplicate association", companies: 1, duplicate: true, wantSelected: true},
		{name: "no company"},
		{name: "multiple companies including primary", companies: 2},
		{name: "foreign workspace company", companies: 1, foreign: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
			db := env.convRepo.DB()
			const ws = "11111111-1111-1111-1111-111111111111"
			contactRepo := repository.NewCRMContactRepository(db)
			companyRepo := repository.NewCRMCompanyRepository(db)
			assocRepo := repository.NewCRMAssociationRepository(db)
			env.service.supportInboxService.SetCRMCompanyRepository(companyRepo)
			env.service.supportInboxService.assocRepo = assocRepo
			email := "customer@example.com"
			contact := &model.CRMContact{WorkspaceID: ws, DisplayID: "CON-1", FirstName: "Taylor", Email: &email, LifecycleStage: model.CRMLifecycleLead, LeadStatus: model.CRMLeadStatusNew}
			if err := contactRepo.Create(ctx, contact); err != nil {
				t.Fatal(err)
			}
			companyID := ""
			for i := 0; i < tt.companies; i++ {
				companyWS := ws
				if tt.foreign {
					companyWS = "foreign-workspace"
					seedWorkspace(t, db, companyWS, "Foreign", "foreign", "22222222-2222-2222-2222-222222222222")
				}
				company := &model.CRMCompany{WorkspaceID: companyWS, DisplayID: fmt.Sprintf("COM-%d", i+1), Name: fmt.Sprintf("Company %d", i+1)}
				if err := companyRepo.Create(ctx, company); err != nil {
					t.Fatal(err)
				}
				companyID = company.ID
				assoc := &model.CRMAssociation{WorkspaceID: ws, FromObjectType: model.CRMObjectContact, FromObjectID: contact.ID, ToObjectType: model.CRMObjectCompany, ToObjectID: company.ID}
				if i == 0 {
					assoc.AssociationLabel = strPtr("primary")
				}
				if tt.reverse {
					assoc.FromObjectType, assoc.ToObjectType = assoc.ToObjectType, assoc.FromObjectType
					assoc.FromObjectID, assoc.ToObjectID = assoc.ToObjectID, assoc.FromObjectID
				}
				if err := assocRepo.Create(ctx, assoc); err != nil {
					t.Fatal(err)
				}
				if tt.duplicate {
					reverse := &model.CRMAssociation{WorkspaceID: ws, FromObjectType: model.CRMObjectCompany, FromObjectID: company.ID, ToObjectType: model.CRMObjectContact, ToObjectID: contact.ID}
					if err := assocRepo.Create(ctx, reverse); err != nil {
						t.Fatal(err)
					}
				}
			}
			route := &model.SupportEmailRoute{WorkspaceID: ws, RouteKey: "company-route", InboundAddress: "inbox@acme.on.helpin.email", ProviderType: "forwarding", Active: true, CreatedByID: "22222222-2222-2222-2222-222222222222"}
			if err := env.routeRepo.Create(ctx, route); err != nil {
				t.Fatal(err)
			}
			payload := model.PostmarkInboundPayload{FromFull: model.PostmarkAddress{Email: email, Name: "Taylor"}, To: route.InboundAddress, OriginalRecipient: route.InboundAddress, Subject: "Billing help", MessageID: "company-test", StrippedTextReply: "Please help", Headers: []model.PostmarkHeader{{Name: "Message-ID", Value: "<company-test@example.com>"}}}
			if err := env.service.ProcessInboundEmail(ctx, payload, `{"MessageID":"company-test"}`); err != nil {
				t.Fatal(err)
			}
			conversations, total, err := env.convRepo.List(ctx, supportConversationListParams(ws, "", "", model.PMPagination{Page: 1, PerPage: 10}, "", model.RoleOwner, nil, "", ""))
			if err != nil || total != 1 {
				t.Fatalf("conversation count=%d, error=%v", total, err)
			}
			conversation := conversations[0]
			if conversation.CRMContactID == nil || *conversation.CRMContactID != contact.ID {
				t.Fatal("did not match existing contact")
			}
			if tt.wantSelected {
				if conversation.CRMCompanyID == nil || *conversation.CRMCompanyID != companyID {
					t.Fatalf("company = %v, want %s", conversation.CRMCompanyID, companyID)
				}
				visitor, err := env.service.supportInboxService.GetVisitorContext(ctx, ws, conversation.ID)
				if err != nil {
					t.Fatal(err)
				}
				if visitor.Company == nil || visitor.Company.ID != companyID {
					t.Fatal("selected company missing from sidebar response")
				}
			} else if conversation.CRMCompanyID != nil {
				t.Fatalf("ambiguous or foreign company selected: %s", *conversation.CRMCompanyID)
			}

			// A teammate can explicitly clear the default. A subsequent
			// customer reply must not infer the same company again.
			if _, err := env.service.supportInboxService.UpdateConversationCRMCompany(ctx, ws, conversation.ID, nil, "22222222-2222-2222-2222-222222222222"); err != nil {
				t.Fatal(err)
			}
			payload.MessageID = "company-reply"
			payload.Headers = []model.PostmarkHeader{{Name: "Message-ID", Value: "<company-reply@example.com>"}, {Name: "In-Reply-To", Value: "<company-test@example.com>"}}
			if err := env.service.ProcessInboundEmail(ctx, payload, `{"MessageID":"company-reply"}`); err != nil {
				t.Fatal(err)
			}
			updated, err := env.convRepo.GetByID(ctx, ws, conversation.ID, "", model.RoleOwner)
			if err != nil {
				t.Fatal(err)
			}
			if updated.CRMCompanyID != nil {
				t.Fatal("reply restored a manually cleared company")
			}
			messages, err := env.messageRepo.ListByConversation(ctx, ws, conversation.ID, false)
			if err != nil {
				t.Fatal(err)
			}
			customerReplies := 0
			for _, message := range messages {
				if message.SenderType == "customer" {
					customerReplies++
				}
			}
			if customerReplies != 2 {
				t.Fatalf("reply did not join original conversation: %d customer replies", customerReplies)
			}
		})
	}
}
