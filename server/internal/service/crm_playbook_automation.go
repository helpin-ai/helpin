package service

import (
	"context"
	"errors"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// AutomationPreview reads product-owned skill metadata for an exact policy revision.
// It deliberately has no Flow/Agent lookup, mutation, launch or scheduling dependency.
// A preview fingerprint is not a published connection or execution authority.
func (s *CRMPlaybookService) AutomationPreview(ctx context.Context, ws, id, versionID string, revision int64) (*model.CRMPlaybookAutomationPreview, error) {
	item, err := s.Get(ctx, ws, id)
	if err != nil {
		return nil, err
	}
	if revision < 1 || (versionID != "" && !validSituationID(versionID)) {
		return nil, ErrCRMPlaybookInput
	}
	if item.Playbook.Revision != revision {
		return nil, repository.ErrCRMPlaybookStale
	}
	definition := item.Playbook.Draft
	result := &model.CRMPlaybookAutomationPreview{
		PlaybookID: id, PlaybookRevision: revision, Scope: "draft",
		PresetKey: model.AgentPresetCRMOperator, Skills: []model.CRMPlaybookSkillSummary{},
		Status: "not_connected", ExecutionEnabled: false,
		Explanation: "These skills match this Playbook. This preview does not connect or run automation.",
	}
	if versionID != "" {
		if item.Playbook.PublishedVersionID == nil || *item.Playbook.PublishedVersionID != versionID ||
			item.PublishedVersion == nil || item.PublishedVersion.ID != versionID ||
			item.PublishedVersion.PlaybookID != id || item.PublishedVersion.WorkspaceID != ws {
			return nil, repository.ErrCRMPlaybookStale
		}
		definition = item.PublishedVersion.Definition
		result.PlaybookVersionID, result.Scope = &versionID, "published"
	}
	result.Journey = definition.Journey
	snapshot, err := agentcontract.CaptureCRMPlaybookSpecialization(definition.Journey)
	if errors.Is(err, agentcontract.ErrCRMPlaybookJourneyUnsupported) {
		result.Status = "unsupported_journey"
		result.Explanation = "No Beacon skill matches this Playbook. You can still manage it manually."
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	result.SpecializationVersion = snapshot.Version
	for _, skill := range snapshot.Skills {
		result.Skills = append(result.Skills, model.CRMPlaybookSkillSummary{
			Key: skill.Key, Title: skill.Title, Role: skill.Role, Version: skill.Version,
		})
	}
	return result, nil
}
