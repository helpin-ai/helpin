package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

type SupportInboxViewService struct {
	viewRepo         *repository.SupportInboxViewRepository
	conversationRepo *repository.SupportConversationRepository
	wsPublisher      *websocket.Publisher
	logger           *slog.Logger
}

func NewSupportInboxViewService(viewRepo *repository.SupportInboxViewRepository, conversationRepo *repository.SupportConversationRepository, wsPublisher *websocket.Publisher) *SupportInboxViewService {
	return &SupportInboxViewService{
		viewRepo:         viewRepo,
		conversationRepo: conversationRepo,
		wsPublisher:      wsPublisher,
		logger:           slog.Default().With("service", "support_inbox_view"),
	}
}

func (s *SupportInboxViewService) List(ctx context.Context, workspaceID, userID string) ([]model.SupportInboxView, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.viewRepo.ListByWorkspace(ctx, workspaceID, userID)
}

func (s *SupportInboxViewService) ListBuiltinViews(ctx context.Context, workspaceID, userID string) ([]model.SupportInboxView, error) {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("workspace_id and user_id are required")
	}
	return s.viewRepo.ListBuiltinByUser(ctx, workspaceID, userID)
}

func (s *SupportInboxViewService) ListCounts(ctx context.Context, workspaceID, userID, workspaceMemberID, role string) ([]model.SupportInboxViewCount, error) {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("workspace_id and user_id are required")
	}
	if s.conversationRepo == nil {
		return nil, fmt.Errorf("conversation repository is required")
	}
	views, err := s.List(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	counts := make([]model.SupportInboxViewCount, 0, len(views))
	for _, view := range views {
		params := supportInboxViewConversationListParams(workspaceID, userID, view.Filters)
		total, unread, err := s.conversationRepo.CountByParams(ctx, repository.ConversationRepositoryListParams{
			ConversationListParams: params,
			WorkspaceMemberID:      workspaceMemberID,
			Role:                   role,
		})
		if err != nil {
			return nil, err
		}
		counts = append(counts, model.SupportInboxViewCount{
			ViewID:      view.ID,
			TotalCount:  total,
			UnreadCount: unread,
		})
	}
	return counts, nil
}

func (s *SupportInboxViewService) Create(ctx context.Context, workspaceID, userID, role string, req model.CreateSupportInboxViewRequest) (*model.SupportInboxView, error) {
	name := strings.TrimSpace(req.Name)
	if strings.TrimSpace(workspaceID) == "" || name == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}
	if req.IsShared && !canManageSharedSupportInboxViews(role) {
		return nil, fmt.Errorf("forbidden: only admins and owners can create shared views")
	}
	view := &model.SupportInboxView{
		WorkspaceID: workspaceID,
		Name:        name,
		Filters:     req.Filters,
		IsShared:    req.IsShared,
		ViewType:    model.SupportInboxViewTypeCustom,
		CreatedBy:   userID,
	}
	if view.Filters == nil {
		view.Filters = model.SupportInboxViewFilters{}
	}
	if err := s.viewRepo.Create(ctx, view); err != nil {
		s.logger.ErrorContext(ctx, "failed to create support inbox view", "error", err, "workspace_id", workspaceID)
		return nil, err
	}
	publishWorkspaceEvent(s.wsPublisher, "created", "support_inbox_view", view.ID, workspaceID, userID)
	return view, nil
}

func (s *SupportInboxViewService) UpsertBuiltinView(ctx context.Context, workspaceID, userID string, req model.UpdateSupportInboxBuiltinViewRequest) (*model.SupportInboxView, error) {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("workspace_id and user_id are required")
	}
	viewKey := strings.TrimSpace(req.ViewKey)
	viewType, err := supportInboxBuiltinViewTypeForKey(viewKey)
	if err != nil {
		return nil, err
	}
	filters := req.Filters
	if filters == nil {
		filters = model.SupportInboxViewFilters{}
	}
	view := &model.SupportInboxView{
		WorkspaceID: workspaceID,
		Name:        supportInboxBuiltinViewName(viewKey),
		Filters:     filters,
		IsShared:    false,
		ViewType:    viewType,
		ViewKey:     &viewKey,
		CreatedBy:   userID,
	}
	if err := s.viewRepo.UpsertBuiltin(ctx, view); err != nil {
		s.logger.ErrorContext(ctx, "failed to upsert support inbox builtin view", "error", err, "workspace_id", workspaceID, "user_id", userID, "view_key", viewKey)
		return nil, err
	}
	publishWorkspaceEvent(s.wsPublisher, "updated", "support_inbox_view", view.ID, view.WorkspaceID, userID)
	return view, nil
}

func (s *SupportInboxViewService) Update(ctx context.Context, workspaceID, id, userID, role string, req model.UpdateSupportInboxViewRequest) (*model.SupportInboxView, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	view, err := s.viewRepo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if view == nil {
		return nil, fmt.Errorf("view not found")
	}
	if view.ViewType != "" && view.ViewType != model.SupportInboxViewTypeCustom {
		return nil, fmt.Errorf("view not found")
	}
	if !canModifySupportInboxView(view, userID, role) {
		return nil, fmt.Errorf("forbidden: you cannot update this view")
	}
	if req.IsShared != nil && *req.IsShared && !canManageSharedSupportInboxViews(role) {
		return nil, fmt.Errorf("forbidden: only admins and owners can share views")
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		view.Name = name
	}
	if req.Filters != nil {
		view.Filters = *req.Filters
		if view.Filters == nil {
			view.Filters = model.SupportInboxViewFilters{}
		}
	}
	if req.IsShared != nil {
		view.IsShared = *req.IsShared
	}
	if err := s.viewRepo.Update(ctx, view); err != nil {
		s.logger.ErrorContext(ctx, "failed to update support inbox view", "error", err, "view_id", id)
		return nil, err
	}
	publishWorkspaceEvent(s.wsPublisher, "updated", "support_inbox_view", view.ID, view.WorkspaceID, userID)
	return view, nil
}

func (s *SupportInboxViewService) Delete(ctx context.Context, workspaceID, id, userID, role string) error {
	if strings.TrimSpace(workspaceID) == "" {
		return fmt.Errorf("workspace_id is required")
	}
	view, err := s.viewRepo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("view not found")
	}
	if view.ViewType != "" && view.ViewType != model.SupportInboxViewTypeCustom {
		return fmt.Errorf("view not found")
	}
	if !canModifySupportInboxView(view, userID, role) {
		return fmt.Errorf("forbidden: you cannot delete this view")
	}
	if err := s.viewRepo.Delete(ctx, workspaceID, id); err != nil {
		s.logger.ErrorContext(ctx, "failed to delete support inbox view", "error", err, "view_id", id)
		return err
	}
	publishWorkspaceEvent(s.wsPublisher, "deleted", "support_inbox_view", view.ID, view.WorkspaceID, userID)
	return nil
}

func canManageSharedSupportInboxViews(role string) bool {
	return role == model.RoleOwner || role == model.RoleAdmin
}

func canModifySupportInboxView(view *model.SupportInboxView, userID, role string) bool {
	if view == nil {
		return false
	}
	if view.ViewType != "" && view.ViewType != model.SupportInboxViewTypeCustom {
		return view.CreatedBy == userID
	}
	if !view.IsShared {
		return view.CreatedBy == userID
	}
	return view.CreatedBy == userID || canManageSharedSupportInboxViews(role)
}

func splitSupportViewCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		result = append(result, trimmed)
	}
	return result
}

func supportViewDefaultStates(navFilter string) []string {
	switch navFilter {
	case "mine", "inbox":
		return []string{model.SupportConversationStatusOpen, model.SupportConversationStatusWaitingOnCustomer}
	case "waiting":
		return []string{model.SupportConversationStatusWaitingOnCustomer}
	case "resolved", "resolved_by_ai":
		return []string{model.SupportConversationStatusResolved}
	case "spam":
		return []string{model.SupportConversationStatusSpam}
	case "ai_active":
		return []string{model.SupportConversationStatusOpen}
	default:
		return []string{model.SupportConversationStatusOpen}
	}
}

func supportViewStringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, value := range a {
		counts[value]++
	}
	for _, value := range b {
		counts[value]--
		if counts[value] < 0 {
			return false
		}
	}
	return true
}

func normalizeSupportViewNavFilter(value string) string {
	switch strings.TrimSpace(value) {
	case "mine", "waiting", "resolved", "spam", "ai_active", "resolved_by_ai":
		return strings.TrimSpace(value)
	default:
		return "inbox"
	}
}

func normalizeSupportViewStates(raw string, navFilter string) []string {
	states := splitSupportViewCSV(raw)
	normalized := make([]string, 0, len(states))
	seen := make(map[string]bool, len(states))
	for _, state := range states {
		value := model.NormalizeSupportConversationStatus(state)
		if value == "" || !model.IsValidSupportConversationStatus(value) || seen[value] {
			continue
		}
		seen[value] = true
		normalized = append(normalized, value)
	}
	if len(normalized) == 0 {
		return supportViewDefaultStates(navFilter)
	}
	return normalized
}

func supportInboxViewConversationListParams(workspaceID, userID string, filters model.SupportInboxViewFilters) repository.ConversationListParams {
	navFilter := normalizeSupportViewNavFilter(filters["nav_filter"])
	states := normalizeSupportViewStates(filters["states"], navFilter)
	hasExplicitStates := !supportViewStringSlicesEqual(states, supportViewDefaultStates(navFilter))

	params := repository.ConversationListParams{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Search:      strings.TrimSpace(filters["search"]),
		AssignedTo:  strings.TrimSpace(filters["assignment"]),
		TagIDs:      splitSupportViewCSV(filters["tag_ids"]),
		SystemTags:  splitSupportViewCSV(filters["system_tags"]),
		AIFilters:   splitSupportViewCSV(filters["ai"]),
		Sort:        strings.TrimSpace(filters["sort"]),
	}

	if mailboxIDs := splitSupportViewCSV(filters["mailbox_ids"]); len(mailboxIDs) > 0 {
		hasAll := false
		scoped := make([]string, 0, len(mailboxIDs))
		for _, mailboxID := range mailboxIDs {
			if mailboxID == "all" {
				hasAll = true
				continue
			}
			scoped = append(scoped, mailboxID)
		}
		if !hasAll {
			params.MailboxIDs = scoped
		}
	} else if mailboxID := strings.TrimSpace(filters["mailbox_id"]); mailboxID != "" && mailboxID != "all" {
		if mailboxID == "shared" {
			empty := ""
			params.MailboxID = &empty
		} else {
			params.MailboxID = &mailboxID
		}
	} else if navFilter == "inbox" {
		empty := ""
		params.MailboxID = &empty
	}

	if !hasExplicitStates {
		switch navFilter {
		case "inbox":
			params.Filter = model.SupportConversationListFilterInbox
		case "mine":
			params.Filter = model.SupportConversationListFilterMine
		case "waiting":
			params.Status = model.SupportConversationStatusWaitingOnCustomer
		case "resolved":
			params.Filter = model.SupportConversationListFilterResolved
		case "spam":
			params.Status = model.SupportConversationStatusSpam
		}
	} else {
		params.Statuses = states
		if navFilter == "mine" {
			params.Filter = model.SupportConversationListFilterMine
		}
	}

	return params
}

func supportInboxBuiltinViewTypeForKey(viewKey string) (string, error) {
	if strings.HasPrefix(viewKey, "team:") {
		if strings.TrimSpace(strings.TrimPrefix(viewKey, "team:")) == "" {
			return "", fmt.Errorf("view_key is invalid")
		}
		return model.SupportInboxViewTypeTeam, nil
	}
	switch viewKey {
	case "nav:inbox", "nav:mine", "nav:waiting", "nav:resolved", "nav:spam", "nav:ai_active", "nav:resolved_by_ai":
		return model.SupportInboxViewTypeDefault, nil
	default:
		return "", fmt.Errorf("view_key is invalid")
	}
}

func supportInboxBuiltinViewName(viewKey string) string {
	switch viewKey {
	case "nav:inbox":
		return "Inbox"
	case "nav:mine":
		return "Mine"
	case "nav:waiting":
		return "Waiting"
	case "nav:resolved":
		return "Resolved"
	case "nav:spam":
		return "Spam"
	case "nav:ai_active":
		return "AI Handling"
	case "nav:resolved_by_ai":
		return "AI Resolved"
	default:
		if strings.HasPrefix(viewKey, "team:") {
			return "Team inbox"
		}
		return "Built-in view"
	}
}
