package service

import (
	"context"
	"errors"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type situationSourceStore interface {
	SourceSignals(context.Context, string, string, int) ([]model.CRMSignal, error)
	SourceSuggestions(context.Context, string, string, int) ([]model.CRMSuggestion, error)
	SourceMember(context.Context, string, string) (*string, error)
	SourceMotion(context.Context, model.CRMSuggestion) (string, error)
	ResolveOwner(context.Context, string, model.CreateCRMSituationRequest) (*string, error)
	ImportSource(context.Context, model.CRMSituationSourceInput) error
}

type situationSignalQualifier interface {
	QualifySituationSignals(context.Context, string, []model.CRMSignal) ([]model.CRMSignal, error)
}

// CRMSituationSourceService reconciles canonical CRM sources into customer work.
// It never sends, creates PM tasks, invokes agents, or decides approvals.
type CRMSituationSourceService struct {
	store   situationSourceStore
	signals situationSignalQualifier
}

// NewCRMSituationSourceService binds persistence and the existing signal policy.
func NewCRMSituationSourceService(store situationSourceStore, signals situationSignalQualifier) *CRMSituationSourceService {
	return &CRMSituationSourceService{store: store, signals: signals}
}

// ReconcileWorkspace is a restart-safe, keyset-paged sweep. Every source is its
// own durable transaction; a malformed record cannot hide the remainder of a page.
func (s *CRMSituationSourceService) ReconcileWorkspace(ctx context.Context, ws string) error {
	if !validSituationID(ws) {
		return ErrCRMSituationInput
	}
	var failures []error
	for _, kind := range []string{"suggestion", "signal"} {
		after := ""
		for {
			if err := ctx.Err(); err != nil {
				return errors.Join(append(failures, err)...)
			}
			const size = 200
			count := 0
			if kind == "suggestion" {
				rows, err := s.store.SourceSuggestions(ctx, ws, after, size)
				if err != nil {
					return errors.Join(append(failures, err)...)
				}
				count = len(rows)
				for _, row := range rows {
					after = row.ID
					if err := s.ImportSuggestion(ctx, row); err != nil {
						failures = append(failures, err)
					}
				}
			} else {
				rows, err := s.store.SourceSignals(ctx, ws, after, size)
				if err != nil {
					return errors.Join(append(failures, err)...)
				}
				count = len(rows)
				if count > 0 {
					after = rows[count-1].ID
				}
				qualified, err := s.signals.QualifySituationSignals(ctx, ws, rows)
				if err != nil {
					return errors.Join(append(failures, err)...)
				}
				for _, signal := range qualified {
					if err := s.importSignal(ctx, signal); err != nil {
						failures = append(failures, err)
					}
				}
			}
			if count < size {
				break
			}
		}
	}
	return errors.Join(failures...)
}

// ImportSuggestion preserves standalone recommendations and explicit source links.
func (s *CRMSituationSourceService) ImportSuggestion(ctx context.Context, suggestion model.CRMSuggestion) error {
	// Bound proposals are linked atomically when created; reconciliation cannot
	// turn an execution artifact into another customer objective.
	if suggestion.SuggestionType == "playbook_action" {
		return nil
	}
	if suggestion.Status != model.CRMSuggestionStatusPending &&
		!(suggestion.Status == model.CRMSuggestionStatusAccepted && suggestion.ExecutionStatus != model.CRMSuggestionExecutionSucceeded) {
		return nil
	}
	motion, _ := suggestion.Context["commercial_motion"].(string)
	if model.CRMSituationCategoryForMotion(motion) == "" {
		resolved, err := s.store.SourceMotion(ctx, suggestion)
		if err != nil {
			return err
		}
		motion = resolved
	}
	if model.CRMSituationCategoryForMotion(motion) == "" {
		switch suggestion.SuggestionType {
		case model.CRMSuggestionDealCreate, model.CRMSuggestionDealAdvance:
			motion = "conversion"
		case model.CRMSuggestionRiskAlert:
			motion = "retention"
		default:
			motion = model.CRMCommercialMotionNeedsContext
		}
	}
	input := model.CRMSituationSourceInput{Kind: "suggestion", SourceID: suggestion.ID}
	input.Situation = model.CRMSituation{WorkspaceID: suggestion.WorkspaceID, CommercialMotion: motion,
		Title:     sourceText(suggestion.Title, 240, "Review customer recommendation"),
		Objective: "Assess the recommendation and agree the customer's next step", Lifecycle: "open", Attention: "needs_context"}
	if suggestion.ObjectType != nil {
		switch *suggestion.ObjectType {
		case "company":
			input.Situation.CompanyID = suggestion.ObjectID
		case "contact":
			input.Situation.ContactID = suggestion.ObjectID
		case "deal":
			input.Situation.DealID = suggestion.ObjectID
		}
	}
	var err error
	if suggestion.UserID != nil {
		input.Situation.OwnerMemberID, err = s.store.SourceMember(ctx, suggestion.WorkspaceID, *suggestion.UserID)
	} else if input.Situation.CompanyID != nil || input.Situation.ContactID != nil || input.Situation.DealID != nil {
		input.Situation.OwnerMemberID, err = s.store.ResolveOwner(ctx, suggestion.WorkspaceID, model.CreateCRMSituationRequest{
			CommercialMotion: motion, CompanyID: input.Situation.CompanyID, ContactID: input.Situation.ContactID, DealID: input.Situation.DealID})
	}
	if err != nil {
		return err
	}
	input.Situation.NextActionOwnerMemberID = input.Situation.OwnerMemberID
	if id, ok := suggestion.Context["situation_id"].(string); ok && validSituationID(id) {
		input.ExistingSituationID = &id
	} else if suggestion.Context["situation_id"] != nil {
		return ErrCRMSituationInput
	}
	for _, id := range suggestion.SignalIDs {
		input.References = append(input.References, model.CRMSituationReference{Kind: "signal", SourceID: id})
	}
	return s.store.ImportSource(ctx, input)
}

func (s *CRMSituationSourceService) importSignal(ctx context.Context, signal model.CRMSignal) error {
	if !signal.ActivationEligible || model.CRMSituationCategoryForMotion(signal.CommercialMotion) == "" {
		return nil
	}
	input := model.CRMSituationSourceInput{Kind: "signal", SourceID: signal.ID, Situation: model.CRMSituation{
		WorkspaceID: signal.WorkspaceID, Title: sourceText(signal.Summary, 240, "Review customer signal"),
		Objective: "Assess the customer evidence and agree the next step", CommercialMotion: signal.CommercialMotion,
		CompanyID: signal.CompanyID, ContactID: signal.ContactID, DealID: signal.DealID,
		OwnerMemberID: signal.OwnerMemberID, NextActionOwnerMemberID: signal.OwnerMemberID,
		Lifecycle: "open", Attention: "needs_context", Priority: signal.BusinessPriority}}
	if signal.RecommendedActionLabel != nil {
		input.Situation.NextStep = sourceText(*signal.RecommendedActionLabel, 1000, "Review customer evidence")
	}
	// Evidence metadata is not authority to merge customer work. An explicit
	// situation link is accepted only from the CRM suggestion creation contract.
	return s.store.ImportSource(ctx, input)
}

func sourceText(value string, max int, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	runes := []rune(value)
	if len(runes) > max {
		return string(runes[:max])
	}
	return value
}
