package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type coverageDockContextReader interface {
	GetGapDetail(context.Context, string, string) (*model.SupportCoverageGapDetail, error)
}

func (s *DockChatService) coverageGapDetail(ctx context.Context, workspaceID, gapID string) (*model.SupportCoverageGapDetail, error) {
	reader := s.coverageContextReader
	if reader == nil && s.agentService != nil && s.agentService.supportCoverageSvc != nil {
		reader = s.agentService.supportCoverageSvc
	}
	if reader == nil {
		return nil, fmt.Errorf("coverage context is unavailable")
	}
	detail, err := reader.GetGapDetail(ctx, workspaceID, gapID)
	if err != nil || detail == nil {
		return nil, ErrDockChatNotFound
	}
	return detail, nil
}

// FindCoverageGapChat reads the shared gap thread without starting a run or creating a chat.
func (s *DockChatService) FindCoverageGapChat(ctx context.Context, workspaceID, userID, gapID string) (*model.DockChat, error) {
	if strings.TrimSpace(gapID) == "" {
		return nil, fmt.Errorf("coverage gap id is required")
	}
	if _, err := s.coverageGapDetail(ctx, workspaceID, gapID); err != nil {
		return nil, err
	}
	if s.authz != nil {
		allowed, err := s.canAccessDockChatModule(ctx, workspaceID, userID, model.ModuleSupport)
		if err != nil || !allowed {
			return nil, ErrDockChatNotFound
		}
	}
	return s.chatRepo.FindByCoverageGap(ctx, workspaceID, gapID)
}

func (s *DockChatService) createCoverageChat(ctx context.Context, workspaceID, userID string, req model.CreateDockChatRequest) (*model.DockChat, error) {
	id := strings.TrimSpace(*req.CoverageGapID)
	if id == "" || req.SupportConversationID != nil {
		return nil, fmt.Errorf("select one coverage gap")
	}
	if req.Visibility != nil && *req.Visibility != model.DockChatVisibilityModule {
		return nil, ErrDockChatInvalidVisibility
	}
	detail, err := s.coverageGapDetail(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	support := model.ModuleSupport
	chat := &model.DockChat{ID: uuid.NewString(), WorkspaceID: workspaceID, UserID: userID, Title: detail.Title, CoverageGapID: &id, Visibility: model.DockChatVisibilityModule, ModuleID: &support, InitialContext: coverageDockFindings(detail)}
	if err := s.authorizeCoverageChatTurn(ctx, chat, userID); err != nil {
		return nil, err
	}
	existing, err := s.FindCoverageGapChat(ctx, workspaceID, userID, id)
	if err != nil || existing != nil {
		return existing, err
	}
	if req.ExecutionEnabled {
		if err := s.authorizeChatExecution(ctx, workspaceID, userID); err != nil {
			return nil, err
		}
		chat.ExecutionEnabled = true
	}
	if err := s.chatRepo.Create(ctx, chat); err != nil {
		// The database uniqueness constraint wins concurrent first sends.
		existing, lookupErr := s.FindCoverageGapChat(ctx, workspaceID, userID, id)
		if lookupErr == nil && existing != nil {
			return existing, nil
		}
		return nil, fmt.Errorf("create coverage conversation: %w", err)
	}
	return chat, nil
}

func (s *DockChatService) authorizeCoverageChatTurn(ctx context.Context, chat *model.DockChat, userID string) error {
	if chat.UserID != userID {
		return fmt.Errorf("this conversation is managed by its creator")
	}
	detail, err := s.coverageGapDetail(ctx, chat.WorkspaceID, *chat.CoverageGapID)
	if err != nil {
		return err
	}
	if detail.Status != model.SupportCoverageGapStatusOpen {
		return fmt.Errorf("reopen this coverage gap to continue")
	}
	if s.authz != nil {
		actor, err := s.authz.ResolveActor(ctx, chat.WorkspaceID, userID)
		if err != nil || !s.authz.Can(actor, authorization.PermSupportEdit) {
			return ErrDockChatNotFound
		}
	}
	return nil
}

func coverageDockNextStep(detail *model.SupportCoverageGapDetail) string {
	for _, suggestion := range detail.Suggestions {
		if suggestion.Status == "draft" && suggestion.IsActive && suggestion.SupersededAt == nil {
			return "Review the proposed fix, then verify that the AI can use the approved guidance."
		}
	}
	if !detail.RecurrenceReopened {
		for _, suggestion := range detail.Suggestions {
			if suggestion.Status == "applied" && suggestion.ResultDocumentID != nil {
				return "Review and publish the saved draft, then verify that the AI can use the fix before marking this gap resolved."
			}
		}
	}
	for _, rec := range detail.Recommendations {
		if rec.Priority == "primary" && (rec.Status == "open" || rec.Status == "draft") && strings.TrimSpace(rec.SuggestedChange) != "" {
			return rec.SuggestedChange
		}
	}
	switch detail.GapKind {
	case "data":
		return "Identify the missing account data and how the AI can access it."
	case "action":
		return "Check the missing action and prepare the guidance or owner handoff needed to resolve it."
	case "policy":
		return "Agree on the policy or escalation rule, then prepare clear guidance."
	}
	if detail.FailureMode == "no_retrieval" {
		return "Check why the AI cannot find the existing guidance."
	}
	if detail.V1GapType == "weak_article" {
		return "Prepare the missing guidance for the existing article."
	}
	return "Review the sources and prepare the right fix for review."
}

func coverageDockTurnContext(detail *model.SupportCoverageGapDetail) string {
	context := strings.TrimSuffix(supportCoverageGapRunContext(detail, nil), "\n\n"+supportCoverageGapAgentInstructions(detail))
	return context + fmt.Sprintf(`
This Ask Agent conversation stays attached to coverage gap %s. Answer the operator's actual question; investigate fresh evidence only when useful. Ask only for missing facts. Cite source conversations and documents. Link the exact prepared document and proposal in your reply. Never invent policy or product behavior. For documentation work, delegate to the documentation agent targeting support_coverage_gap/%s and prepare a review-ready draft or proposal. Never publish content or mark the gap resolved automatically. Saving a draft is not resolution.`, detail.ID, detail.ID)
}
