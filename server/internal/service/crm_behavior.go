package service

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// SetBehaviorProfiles attaches synced product-usage data to contact reads.
func (s *CRMContactService) SetBehaviorProfiles(repo *repository.CRMBehaviorProfileRepository) *CRMContactService {
	s.behavior = repo
	return s
}

// SetBehaviorProfiles attaches synced product-usage data to company reads.
func (s *CRMCompanyService) SetBehaviorProfiles(repo *repository.CRMBehaviorProfileRepository) *CRMCompanyService {
	s.behavior = repo
	return s
}

func (s *CRMContactService) attachContactBehavior(ctx context.Context, workspaceID string, contacts []model.CRMContact) error {
	if s.behavior == nil || len(contacts) == 0 {
		return nil
	}
	ids := make([]string, 0, len(contacts))
	for i := range contacts {
		ids = append(ids, contacts[i].ID)
	}
	summaries, err := s.behavior.SummariesByID(ctx, workspaceID, model.CRMFieldObjectContact, ids)
	if err != nil {
		return err
	}
	for i := range contacts {
		if summary, ok := summaries[contacts[i].ID]; ok {
			contacts[i].Behavior = &summary
		}
	}
	return nil
}

func (s *CRMCompanyService) attachCompanyBehavior(ctx context.Context, workspaceID string, companies []model.CRMCompany) error {
	if s.behavior == nil || len(companies) == 0 {
		return nil
	}
	ids := make([]string, 0, len(companies))
	for i := range companies {
		ids = append(ids, companies[i].ID)
	}
	summaries, err := s.behavior.SummariesByID(ctx, workspaceID, model.CRMFieldObjectCompany, ids)
	if err != nil {
		return err
	}
	for i := range companies {
		if summary, ok := summaries[companies[i].ID]; ok {
			companies[i].Behavior = &summary
		}
	}
	return nil
}
