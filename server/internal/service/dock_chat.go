package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// ErrDockChatNotFound is returned when a chat does not exist or is not owned
// by the requesting user.
var ErrDockChatNotFound = errors.New("dock chat not found")

// ErrDockChatInvalidCursor is returned for malformed list pagination cursors.
var ErrDockChatInvalidCursor = errors.New("invalid dock chat cursor")

// ErrDockChatInvalidVisibility is returned for an unsupported visibility or
// for module visibility without a valid module context.
var ErrDockChatInvalidVisibility = errors.New("invalid dock chat visibility")

const (
	dockChatTriggerType               = "dock_chat"
	dockChatTitleMaxRunes             = 60
	dockChatCarryForwardTurns         = 20
	dockChatCarryForwardChars         = 500
	dockChatCarryForwardTotal         = 6000
	dockChatPageContextOpenTag        = "<page_context>"
	dockChatReferencesOpenTag         = "<references>"
	dockChatAttachmentsOpenTag        = "<attachments>"
	dockChatSourceAttachmentsOpenTag  = "<source_attachments>"
	dockChatAttachmentAnalysisOpenTag = "<attachment_analysis>"
	dockChatReferencesMax             = 10
	dockChatListDefaultLimit          = 30
	dockChatListMaxLimit              = 50
)

type dockChatCursor struct {
	ActivityAt time.Time `json:"activity_at"`
	ID         string    `json:"id"`
}

// DockChatService owns private and shared dock conversations whose turns are
// executed by an agent-runtime chat-mode run of the ask_agent preset.
type DockChatService struct {
	chatRepo            *repository.DockChatRepository
	runRepo             *repository.AgentRunRepository
	runMessageRepo      *repository.AgentRunMessageRepository
	planRepo            *repository.CommandBarPlanRepository
	agentService        *AgentService
	commandService      *InternalCommandService
	authz               *authorization.AuthzService
	titleLLM            dockChatTitleLLM
	pmAttachmentRepo    *repository.PMAttachmentRepository
	pmAttachmentService *PMAttachmentService
	supportInboxService *SupportInboxService
	mediaLLM            dockChatMediaLLM
	externalMediaClient *http.Client
}

// SetPMAttachmentRepository enables first-class Ask media attachments.
func (s *DockChatService) SetPMAttachmentRepository(repo *repository.PMAttachmentRepository) *DockChatService {
	if s != nil {
		s.pmAttachmentRepo = repo
	}
	return s
}

// SetMediaSourceService lets Ask discover media already attached to a support
// conversation selected as page context.
func (s *DockChatService) SetMediaSourceService(support *SupportInboxService) *DockChatService {
	if s != nil {
		s.supportInboxService = support
	}
	return s
}

// NewDockChatService creates a DockChatService.
func NewDockChatService(
	chatRepo *repository.DockChatRepository,
	runRepo *repository.AgentRunRepository,
	runMessageRepo *repository.AgentRunMessageRepository,
	planRepo *repository.CommandBarPlanRepository,
	agentService *AgentService,
	commandService *InternalCommandService,
	authz *authorization.AuthzService,
) *DockChatService {
	return &DockChatService{
		chatRepo:            chatRepo,
		runRepo:             runRepo,
		runMessageRepo:      runMessageRepo,
		planRepo:            planRepo,
		agentService:        agentService,
		commandService:      commandService,
		authz:               authz,
		externalMediaClient: newDockChatExternalMediaClient(),
	}
}

// ListChats returns one stable cursor page of the user's unarchived chats.
func (s *DockChatService) ListChats(ctx context.Context, workspaceID, userID string, limit int, encodedCursor string) (*model.DockChatListResponse, error) {
	if limit <= 0 {
		limit = dockChatListDefaultLimit
	}
	if limit > dockChatListMaxLimit {
		limit = dockChatListMaxLimit
	}
	var before *time.Time
	var beforeID string
	if encodedCursor != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(encodedCursor)
		if err != nil {
			return nil, ErrDockChatInvalidCursor
		}
		var cursor dockChatCursor
		if err := json.Unmarshal(decoded, &cursor); err != nil || cursor.ActivityAt.IsZero() || cursor.ID == "" {
			return nil, ErrDockChatInvalidCursor
		}
		before = &cursor.ActivityAt
		beforeID = cursor.ID
	}

	modules, err := s.accessibleDockChatModules(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	chats, err := s.chatRepo.ListVisible(ctx, workspaceID, userID, modules, limit+1, before, beforeID)
	if err != nil {
		return nil, err
	}
	if err := s.hydrateActiveRunStatuses(ctx, workspaceID, chats); err != nil {
		return nil, err
	}
	response := &model.DockChatListResponse{Chats: chats}
	if len(chats) > limit {
		response.Chats = chats[:limit]
		last := response.Chats[len(response.Chats)-1]
		activityAt := last.CreatedAt
		if last.LastMessageAt != nil {
			activityAt = *last.LastMessageAt
		}
		payload, err := json.Marshal(dockChatCursor{ActivityAt: activityAt, ID: last.ID})
		if err != nil {
			return nil, fmt.Errorf("encode dock chat cursor: %w", err)
		}
		next := base64.RawURLEncoding.EncodeToString(payload)
		response.NextCursor = &next
	}
	return response, nil
}

// FindSupportConversationChat returns an existing visible chat for a support
// conversation without creating one. The support sidebar uses this to keep an
// untouched conversation as a local draft until the first message is sent.
func (s *DockChatService) FindSupportConversationChat(ctx context.Context, workspaceID, userID, conversationID string) (*model.DockChat, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, errors.New("support conversation id is required")
	}
	existingChats, err := s.chatRepo.ListBySupportConversation(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, fmt.Errorf("find support conversation chat: %w", err)
	}
	for index := range existingChats {
		allowed, accessErr := s.canAccessChat(ctx, &existingChats[index], userID)
		if accessErr != nil {
			return nil, accessErr
		}
		if allowed {
			return &existingChats[index], nil
		}
	}
	return nil, nil
}

// CreateChat creates an empty chat; its backing run starts lazily on the
// first message.
func (s *DockChatService) CreateChat(ctx context.Context, workspaceID, userID string, req model.CreateDockChatRequest) (*model.DockChat, error) {
	visibility, moduleID, err := s.resolveCreateVisibility(ctx, workspaceID, userID, req)
	if err != nil {
		return nil, err
	}
	if req.SupportConversationID != nil {
		conversationID := strings.TrimSpace(*req.SupportConversationID)
		if conversationID == "" {
			return nil, errors.New("support conversation id is required")
		}
		req.SupportConversationID = &conversationID
		existingChat, err := s.FindSupportConversationChat(ctx, workspaceID, userID, conversationID)
		if err != nil {
			return nil, err
		}
		if existingChat != nil {
			return existingChat, nil
		}
	}
	chat := &model.DockChat{
		WorkspaceID:           workspaceID,
		UserID:                userID,
		Title:                 strings.TrimSpace(req.Title),
		Visibility:            visibility,
		ModuleID:              moduleID,
		SupportConversationID: req.SupportConversationID,
	}
	if err := s.chatRepo.Create(ctx, chat); err != nil {
		if req.SupportConversationID != nil {
			existingChats, lookupErr := s.chatRepo.ListBySupportConversation(ctx, workspaceID, *req.SupportConversationID)
			if lookupErr == nil {
				for index := range existingChats {
					allowed, accessErr := s.canAccessChat(ctx, &existingChats[index], userID)
					if accessErr == nil && allowed {
						return &existingChats[index], nil
					}
				}
			}
		}
		return nil, fmt.Errorf("create dock chat: %w", err)
	}
	return chat, nil
}

// UpdateChat renames or archives/unarchives a chat.
func (s *DockChatService) UpdateChat(ctx context.Context, workspaceID, userID, chatID string, req model.UpdateDockChatRequest) (*model.DockChat, error) {
	var result *model.DockChat
	err := s.chatRepo.WithTurnLock(ctx, workspaceID, chatID, func() error {
		var err error
		result, err = s.updateChatLocked(ctx, workspaceID, userID, chatID, req)
		return err
	})
	return result, err
}

func (s *DockChatService) updateChatLocked(ctx context.Context, workspaceID, userID, chatID string, req model.UpdateDockChatRequest) (*model.DockChat, error) {
	chat, err := s.ownedChat(ctx, workspaceID, userID, chatID)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	if req.ExecutionEnabled != nil && *req.ExecutionEnabled != chat.ExecutionEnabled {
		if *req.ExecutionEnabled {
			if err := s.authorizeChatExecution(ctx, workspaceID, userID); err != nil {
				return nil, err
			}
		}
		if chat.ActiveRunID != nil {
			run, err := s.runRepo.GetByID(ctx, workspaceID, *chat.ActiveRunID)
			if err != nil {
				return nil, err
			}
			if run != nil && model.IsAgentRunActiveStatus(run.Status) {
				if *req.ExecutionEnabled && !model.IsAgentRunPausedStatus(run.Status) {
					return nil, fmt.Errorf("wait for the current turn to finish before enabling execution")
				}
				if _, err := s.agentService.CancelRun(ctx, workspaceID, run.ID, userID); err != nil {
					return nil, err
				}
			}
		}
		updates["execution_enabled"] = *req.ExecutionEnabled
	}
	if req.Title != nil {
		updates["title"] = strings.TrimSpace(*req.Title)
	}
	if req.Archived != nil {
		if *req.Archived {
			updates["archived_at"] = time.Now().UTC()
		} else {
			updates["archived_at"] = nil
		}
	}
	if req.Visibility != nil {
		visibility := *req.Visibility
		if !validDockChatVisibility(visibility) || (visibility == model.DockChatVisibilityModule && chat.ModuleID == nil) {
			return nil, ErrDockChatInvalidVisibility
		}
		if visibility == model.DockChatVisibilityModule {
			allowed, accessErr := s.canAccessDockChatModule(ctx, workspaceID, userID, *chat.ModuleID)
			if accessErr != nil {
				return nil, accessErr
			}
			if !allowed {
				return nil, ErrDockChatNotFound
			}
		}
		updates["visibility"] = visibility
	}
	if len(updates) == 0 {
		return chat, nil
	}
	if err := s.chatRepo.Update(ctx, workspaceID, chat.ID, updates); err != nil {
		return nil, fmt.Errorf("update dock chat: %w", err)
	}
	updated, err := s.chatRepo.GetByID(ctx, workspaceID, chat.ID)
	if err != nil || updated == nil {
		return updated, err
	}
	chats := []model.DockChat{*updated}
	if err := s.hydrateActiveRunStatuses(ctx, workspaceID, chats); err != nil {
		return nil, err
	}
	return &chats[0], nil
}

// GetChat returns the chat with a summary of its current backing run.
func (s *DockChatService) GetChat(ctx context.Context, workspaceID, userID, chatID string) (*model.DockChatDetail, error) {
	chat, err := s.accessibleChat(ctx, workspaceID, userID, chatID)
	if err != nil {
		return nil, err
	}
	return s.chatDetail(ctx, chat)
}

// ActiveRunForChat resolves the chat's current backing run after a visibility
// check. Handlers use it to proxy read-only run data without granting access
// to arbitrary runs.
func (s *DockChatService) ActiveRunForChat(ctx context.Context, workspaceID, userID, chatID string) (*model.AgentRun, error) {
	chat, err := s.accessibleChat(ctx, workspaceID, userID, chatID)
	if err != nil {
		return nil, err
	}
	if chat.ActiveRunID == nil {
		return nil, nil
	}
	return s.runRepo.GetByID(ctx, workspaceID, *chat.ActiveRunID)
}

// OwnedActiveRunForChat resolves a run for a mutating operation. Shared
// teammates can inspect a chat, but only its creator can approve or cancel
// work until collaborative message attribution is introduced.
func (s *DockChatService) OwnedActiveRunForChat(ctx context.Context, workspaceID, userID, chatID string) (*model.AgentRun, error) {
	chat, err := s.ownedChat(ctx, workspaceID, userID, chatID)
	if err != nil {
		return nil, err
	}
	if chat.ActiveRunID == nil {
		return nil, nil
	}
	return s.runRepo.GetByID(ctx, workspaceID, *chat.ActiveRunID)
}

// SendMessage delivers one user chat turn: it lazily starts the backing
// chat-mode run, resumes a paused run with the reply, or starts a successor
// run carrying forward context when the previous run ended (idle expiry,
// completion, failure).
func (s *DockChatService) SendMessage(ctx context.Context, workspaceID, userID, chatID string, req model.SendDockChatMessageRequest) (*model.DockChatDetail, error) {
	var result *model.DockChatDetail
	err := s.chatRepo.WithTurnLock(ctx, workspaceID, chatID, func() error {
		var err error
		result, err = s.sendMessageLocked(ctx, workspaceID, userID, chatID, req)
		return err
	})
	return result, err
}

func (s *DockChatService) sendMessageLocked(ctx context.Context, workspaceID, userID, chatID string, req model.SendDockChatMessageRequest) (*model.DockChatDetail, error) {
	if req.AIProfileID != "" && (req.ModelConnectionID != "" || req.ModelName != "") {
		return nil, fmt.Errorf("select a profile or legacy connection, not both")
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}
	chat, err := s.accessibleChat(ctx, workspaceID, userID, chatID)
	if err != nil {
		return nil, err
	}
	clientMessageID := strings.TrimSpace(req.ClientMessageID)
	if clientMessageID == "" {
		clientMessageID = uuid.NewString()
	} else if _, err := uuid.Parse(clientMessageID); err != nil {
		return nil, fmt.Errorf("client_message_id must be a UUID")
	}
	if s.runMessageRepo != nil {
		existing, err := s.runMessageRepo.GetByClientMessageID(ctx, workspaceID, clientMessageID)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			if existing.DockChatID == nil || strings.TrimSpace(*existing.DockChatID) != chat.ID {
				return nil, fmt.Errorf("client_message_id is already used by another chat")
			}
			if existing.DeliveryStatus != "failed" {
				detail, err := s.chatDetail(ctx, chat)
				if detail != nil {
					detail.AcceptedMessage = existing
				}
				return detail, err
			}
			// A confirmed failed attempt was never part of the AI conversation.
			// Give an explicit retry a fresh delivery identity while retaining the
			// failed row for operations/audit history.
			clientMessageID = uuid.NewString()
		}
	}

	references, err := normalizeDockChatReferences(req.References)
	if err != nil {
		return nil, err
	}
	attachedContexts := dockChatAttachedContexts(chat, req.PageContext, references)
	attachments, err := s.resolveDockChatMediaAttachments(ctx, workspaceID, userID, req.AttachmentIDs)
	if err != nil {
		return nil, err
	}
	sourceAttachments, err := s.resolveDockChatSourceMediaAttachments(ctx, workspaceID, req.PageContext, references)
	if err != nil {
		return nil, err
	}
	analysis, err := s.analyzeDockChatMedia(ctx, workspaceID, userID, content, append(append([]dockChatMediaAttachment{}, attachments...), sourceAttachments...))
	if err != nil {
		return nil, err
	}
	composed := composeDockChatTurn(content, req.PageContext, references, attachments, sourceAttachments, analysis)

	var currentRun *model.AgentRun
	if chat.ActiveRunID != nil {
		currentRun, err = s.runRepo.GetByID(ctx, workspaceID, *chat.ActiveRunID)
		if err != nil {
			return nil, fmt.Errorf("get chat run: %w", err)
		}
	}

	if currentRun != nil {
		if err := requireAIConnectionRunOwner(currentRun, userID); err != nil {
			return nil, err
		}
		var input model.AgentRunInputPayload
		if err := decodeAIConnectionRunInput(currentRun.Input, &input); err != nil {
			return nil, err
		}
		if (req.AIProfileID != "" && (input.AISelection == nil || input.AISelection.ProfileID != req.AIProfileID)) || (req.ModelConnectionID != "" && req.ModelConnectionID != input.ModelConnectionID) || (req.ModelName != "" && req.ModelName != input.ModelName) {
			return nil, fmt.Errorf("start a new chat to change AI connection or model")
		}
	}

	switch {
	case currentRun == nil || !model.IsAgentRunActiveStatus(currentRun.Status):
		// First message, or the previous backing run ended.
		if err := s.startChatRun(ctx, chat, userID, composed, attachedContexts, currentRun, clientMessageID, req.ModelConnectionID, req.ModelName, req.AIProfileID); err != nil {
			return nil, err
		}
	case model.IsAgentRunPausedStatus(currentRun.Status):
		usesCurrentTools, err := s.runUsesCurrentScopedTools(ctx, chat, userID, currentRun)
		if err != nil {
			return nil, err
		}
		if !usesCurrentTools {
			// Tool grants are immutable runtime-start input. Rotate a paused Dock
			// run when the managed Ask Agent contract or the user's scoped
			// permissions changed so old chats gain new reads and lose revoked
			// capabilities without waiting for the 72-hour idle timeout.
			if _, err := s.agentService.CancelRun(ctx, workspaceID, currentRun.ID, userID); err != nil {
				return nil, fmt.Errorf("rotate stale chat run: %w", err)
			}
			if err := s.startChatRun(ctx, chat, userID, composed, attachedContexts, currentRun, clientMessageID, req.ModelConnectionID, req.ModelName, req.AIProfileID); err != nil {
				return nil, err
			}
		} else if err := s.setRunAttachedContexts(ctx, currentRun, attachedContexts); err != nil {
			return nil, err
		} else if _, err := s.agentService.SendRunMessage(ctx, workspaceID, currentRun.ID, userID, model.SendAgentRunMessageRequest{Content: composed, ClientMessageID: clientMessageID}); err != nil {
			if !isChatRunExpiredError(err) {
				return nil, err
			}
			// The runtime idle-expired the run; it is completed on its side.
			// Continue the conversation through a successor run.
			clientMessageID = uuid.NewString()
			if err := s.startChatRun(ctx, chat, userID, composed, attachedContexts, currentRun, clientMessageID, req.ModelConnectionID, req.ModelName, req.AIProfileID); err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("the chat agent is still working on the previous message")
	}

	now := time.Now().UTC()
	updates := map[string]interface{}{"last_message_at": now}
	if err := s.chatRepo.Update(ctx, workspaceID, chat.ID, updates); err != nil {
		return nil, fmt.Errorf("touch dock chat: %w", err)
	}

	chat, err = s.chatRepo.GetByID(ctx, workspaceID, chat.ID)
	if err != nil || chat == nil {
		return nil, fmt.Errorf("reload dock chat: %w", err)
	}
	detail, err := s.chatDetail(ctx, chat)
	if err != nil {
		return nil, err
	}
	if s.runMessageRepo != nil {
		detail.AcceptedMessage, err = s.runMessageRepo.GetByClientMessageID(ctx, workspaceID, clientMessageID)
		if err != nil {
			return nil, err
		}
	}
	return detail, nil
}

// ListMessages returns one stable page from the chat's persisted transcript,
// including messages belonging to predecessor backing runs.
func (s *DockChatService) ListMessages(ctx context.Context, workspaceID, userID, chatID string, before *int64, limit int) (*model.DockChatMessageListResponse, error) {
	chat, err := s.accessibleChat(ctx, workspaceID, userID, chatID)
	if err != nil {
		return nil, err
	}
	if s.runMessageRepo == nil {
		return &model.DockChatMessageListResponse{Messages: []model.AgentRunMessage{}}, nil
	}
	messages, nextBefore, err := s.runMessageRepo.ListByDockChat(ctx, workspaceID, chat.ID, before, limit)
	if err != nil {
		return nil, err
	}
	return &model.DockChatMessageListResponse{Messages: compactDockChatMessagePage(messages), NextBefore: nextBefore}, nil
}

// GetMessageWorkDetail returns the full persisted assistant interval omitted
// from compact Dock chat history.
func (s *DockChatService) GetMessageWorkDetail(ctx context.Context, workspaceID, userID, chatID, messageID string) (*model.DockChatWorkDetailResponse, error) {
	chat, err := s.accessibleChat(ctx, workspaceID, userID, chatID)
	if err != nil {
		return nil, err
	}
	if s.runMessageRepo == nil {
		return &model.DockChatWorkDetailResponse{Messages: []model.AgentRunMessage{}}, nil
	}
	messages, err := s.runMessageRepo.ListDockChatTurnThroughMessage(ctx, workspaceID, chat.ID, messageID)
	if err != nil {
		return nil, err
	}
	if len(messages) == 0 || messages[len(messages)-1].ID != messageID || messages[len(messages)-1].Role != "assistant" {
		return nil, ErrDockChatNotFound
	}
	return &model.DockChatWorkDetailResponse{Messages: messages}, nil
}

func (s *DockChatService) runUsesCurrentScopedTools(ctx context.Context, chat *model.DockChat, userID string, run *model.AgentRun) (bool, error) {
	if chat == nil || run == nil {
		return false, nil
	}
	agent, err := s.agentService.ensureBuiltInAgent(ctx, chat.WorkspaceID, userID, model.AgentPresetAskAgent)
	if err != nil {
		return false, fmt.Errorf("ensure ask agent for tool contract: %w", err)
	}
	current, err := s.scopedChatExecutionTools(ctx, chat, userID, agent)
	if err != nil {
		return false, err
	}
	var input model.AgentRunInputPayload
	if err := json.Unmarshal(run.Input, &input); err != nil {
		return false, nil
	}
	return input.ExecutionEnabled == chat.ExecutionEnabled && sameNormalizedToolSet(input.AllowedTools, current), nil
}

func sameNormalizedToolSet(left, right []string) bool {
	leftSet := make(map[string]struct{}, len(left))
	for _, tool := range left {
		if tool = strings.TrimSpace(tool); tool != "" {
			leftSet[tool] = struct{}{}
		}
	}
	rightSet := make(map[string]struct{}, len(right))
	for _, tool := range right {
		if tool = strings.TrimSpace(tool); tool != "" {
			rightSet[tool] = struct{}{}
		}
	}
	if len(leftSet) != len(rightSet) {
		return false
	}
	for tool := range leftSet {
		if _, ok := rightSet[tool]; !ok {
			return false
		}
	}
	return true
}

func (s *DockChatService) chatDetail(ctx context.Context, chat *model.DockChat) (*model.DockChatDetail, error) {
	detail := &model.DockChatDetail{Chat: *chat, PlanIDs: []string{}}
	if chat.ActiveRunID != nil {
		run, err := s.runRepo.GetByID(ctx, chat.WorkspaceID, *chat.ActiveRunID)
		if err != nil {
			return nil, fmt.Errorf("get chat run: %w", err)
		}
		if run != nil {
			model.NormalizeAgentRunPauseState(run)
			detail.Chat.ActiveRunStatus = run.Status
		}
		detail.Run = run
	}
	if s.planRepo != nil {
		plans, err := s.planRepo.ListByDockChat(ctx, chat.WorkspaceID, chat.ID, 20)
		if err == nil {
			for _, plan := range plans {
				detail.PlanIDs = append(detail.PlanIDs, plan.ID)
			}
		}
	}
	return detail, nil
}

// hydrateActiveRunStatuses attaches the current backing-run lifecycle to a
// page of chats in one query so roster indicators do not require N+1 reads.
func (s *DockChatService) hydrateActiveRunStatuses(ctx context.Context, workspaceID string, chats []model.DockChat) error {
	if s.runRepo == nil || len(chats) == 0 {
		return nil
	}
	runIDs := make([]string, 0, len(chats))
	seen := make(map[string]struct{}, len(chats))
	for i := range chats {
		if chats[i].ActiveRunID == nil {
			continue
		}
		runID := strings.TrimSpace(*chats[i].ActiveRunID)
		if runID == "" {
			continue
		}
		if _, exists := seen[runID]; exists {
			continue
		}
		seen[runID] = struct{}{}
		runIDs = append(runIDs, runID)
	}
	if len(runIDs) == 0 {
		return nil
	}

	runs, err := s.runRepo.ListByIDs(ctx, workspaceID, runIDs)
	if err != nil {
		return fmt.Errorf("list dock chat run statuses: %w", err)
	}
	statusByID := make(map[string]string, len(runs))
	for i := range runs {
		model.NormalizeAgentRunPauseState(&runs[i])
		statusByID[runs[i].ID] = runs[i].Status
	}
	for i := range chats {
		if chats[i].ActiveRunID != nil {
			chats[i].ActiveRunStatus = statusByID[strings.TrimSpace(*chats[i].ActiveRunID)]
		}
	}
	return nil
}

func (s *DockChatService) ownedChat(ctx context.Context, workspaceID, userID, chatID string) (*model.DockChat, error) {
	chat, err := s.chatRepo.GetByID(ctx, workspaceID, chatID)
	if err != nil {
		return nil, fmt.Errorf("get dock chat: %w", err)
	}
	if chat == nil || chat.UserID != strings.TrimSpace(userID) {
		return nil, ErrDockChatNotFound
	}
	return chat, nil
}

func (s *DockChatService) accessibleChat(ctx context.Context, workspaceID, userID, chatID string) (*model.DockChat, error) {
	chat, err := s.chatRepo.GetByID(ctx, workspaceID, chatID)
	if err != nil {
		return nil, fmt.Errorf("get dock chat: %w", err)
	}
	allowed, err := s.canAccessChat(ctx, chat, userID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrDockChatNotFound
	}
	return chat, nil
}

func (s *DockChatService) canAccessChat(ctx context.Context, chat *model.DockChat, userID string) (bool, error) {
	if chat == nil {
		return false, nil
	}
	if strings.TrimSpace(chat.UserID) == strings.TrimSpace(userID) {
		return true, nil
	}
	switch chat.Visibility {
	case model.DockChatVisibilityWorkspace:
		if s.authz == nil {
			return false, nil
		}
		_, err := s.authz.ResolveActor(ctx, chat.WorkspaceID, userID)
		return err == nil, err
	case model.DockChatVisibilityModule:
		if chat.ModuleID == nil {
			return false, nil
		}
		return s.canAccessDockChatModule(ctx, chat.WorkspaceID, userID, *chat.ModuleID)
	default:
		return false, nil
	}
}

func (s *DockChatService) accessibleDockChatModules(ctx context.Context, workspaceID, userID string) ([]model.ModuleID, error) {
	if s.authz == nil {
		return nil, nil
	}
	actor, err := s.authz.ResolveActor(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	modules, err := s.authz.AccessibleModules(ctx, actor)
	if err != nil {
		return nil, err
	}
	result := make([]model.ModuleID, 0, len(modules))
	for _, moduleID := range modules {
		if validDockChatModule(moduleID) {
			result = append(result, moduleID)
		}
	}
	return result, nil
}

func (s *DockChatService) canAccessDockChatModule(ctx context.Context, workspaceID, userID string, moduleID model.ModuleID) (bool, error) {
	if !validDockChatModule(moduleID) || s.authz == nil {
		return false, nil
	}
	actor, err := s.authz.ResolveActor(ctx, workspaceID, userID)
	if err != nil {
		return false, err
	}
	return s.authz.CanAccessModule(ctx, actor, moduleID)
}

func (s *DockChatService) resolveCreateVisibility(ctx context.Context, workspaceID, userID string, req model.CreateDockChatRequest) (model.DockChatVisibility, *model.ModuleID, error) {
	moduleID := req.ModuleID
	if req.SupportConversationID != nil {
		support := model.ModuleSupport
		moduleID = &support
	}
	if moduleID != nil && !validDockChatModule(*moduleID) {
		return "", nil, ErrDockChatInvalidVisibility
	}
	visibility := model.DockChatVisibilityPrivate
	if moduleID != nil {
		visibility = model.DockChatVisibilityModule
	}
	if req.Visibility != nil {
		visibility = *req.Visibility
	}
	if !validDockChatVisibility(visibility) || (visibility == model.DockChatVisibilityModule && moduleID == nil) {
		return "", nil, ErrDockChatInvalidVisibility
	}
	if moduleID != nil {
		allowed, err := s.canAccessDockChatModule(ctx, workspaceID, userID, *moduleID)
		if err != nil {
			return "", nil, err
		}
		// Services created without authz are used by focused repository tests;
		// production construction always supplies authz.
		if s.authz != nil && !allowed {
			return "", nil, ErrDockChatNotFound
		}
	}
	return visibility, moduleID, nil
}

func validDockChatVisibility(visibility model.DockChatVisibility) bool {
	switch visibility {
	case model.DockChatVisibilityPrivate, model.DockChatVisibilityModule, model.DockChatVisibilityWorkspace:
		return true
	default:
		return false
	}
}

func validDockChatModule(moduleID model.ModuleID) bool {
	switch moduleID {
	case model.ModuleSupport, model.ModuleCRM, model.ModulePM, model.ModuleDocs:
		return true
	default:
		return false
	}
}

// startChatRun starts a (possibly successor) backing run for the chat and
// repoints the chat at it.
func (s *DockChatService) startChatRun(ctx context.Context, chat *model.DockChat, userID, composedTurn string, attachedContexts []model.AgentRunContextReference, previousRun *model.AgentRun, clientMessageID string, selection ...string) error {
	agent, err := s.agentService.ensureBuiltInAgent(ctx, chat.WorkspaceID, userID, model.AgentPresetAskAgent)
	if err != nil {
		return fmt.Errorf("ensure ask agent: %w", err)
	}
	allowedTools, err := s.scopedChatExecutionTools(ctx, chat, userID, agent)
	if err != nil {
		return err
	}

	var contextBlocks []string
	var parentRunID *string
	if previousRun != nil {
		if carry := s.buildCarryForward(ctx, previousRun); carry != "" {
			contextBlocks = append(contextBlocks, carry)
		}
		parentRunID = &previousRun.ID
	}
	// Deliver any settled child-run results that could not be resumed into the
	// previous run (it had already ended) through the successor's context.
	pendingBlocks, pendingPlanIDs := s.unnotifiedPlanResultBlocks(ctx, chat)
	contextBlocks = append(contextBlocks, pendingBlocks...)
	additional := composedTurn
	if len(contextBlocks) > 0 {
		additional = strings.Join(contextBlocks, "\n\n") + "\n\n" + composedTurn
	}

	now := time.Now().UTC()
	triggerContext, err := json.Marshal(struct {
		DockChatID       string                           `json:"dock_chat_id"`
		AttachedContexts []model.AgentRunContextReference `json:"attached_contexts,omitempty"`
	}{DockChatID: chat.ID, AttachedContexts: attachedContexts})
	if err != nil {
		return fmt.Errorf("encode dock chat context: %w", err)
	}
	trigger := &model.AgentRunTriggerContext{
		Source:      model.AgentRunTriggerSourceManual,
		TriggerType: dockChatTriggerType,
		ActorID:     &userID,
		FiredAt:     &now,
		Context:     triggerContext,
	}

	connectionID, modelName, profileID := "", "", ""
	if len(selection) >= 2 {
		connectionID, modelName = selection[0], selection[1]
	}
	if len(selection) >= 3 {
		profileID = selection[2]
	}
	run, err := s.agentService.startTargetRunWithOptions(
		ctx,
		chat.WorkspaceID,
		"workspace",
		chat.WorkspaceID,
		model.StartAgentRunRequest{
			AgentID:           agent.ID,
			ModelConnectionID: connectionID, ModelName: modelName, AIProfileID: profileID,
			AdditionalContext: &additional,
			AllowedTools:      allowedTools,
		},
		&userID,
		trigger,
		nil,
		parentRunID,
		startTargetRunOptions{dockChatID: &chat.ID, clientMessageID: clientMessageID, executionEnabled: chat.ExecutionEnabled},
	)
	if err != nil {
		return err
	}
	if err := s.chatRepo.SetActiveRun(ctx, chat.WorkspaceID, chat.ID, run.ID); err != nil {
		return fmt.Errorf("set chat active run: %w", err)
	}
	chat.ActiveRunID = &run.ID
	for _, planID := range pendingPlanIDs {
		if err := s.planRepo.MarkParentNotified(ctx, chat.WorkspaceID, planID); err != nil {
			slog.WarnContext(ctx, "dock chat: mark plan notified after carry-forward failed",
				"workspace_id", chat.WorkspaceID, "plan_id", planID, "error", err)
		}
	}
	return nil
}

func dockChatAttachedContexts(chat *model.DockChat, pageContext map[string]interface{}, references []model.DockEntityReference) []model.AgentRunContextReference {
	contexts := make([]model.AgentRunContextReference, 0, len(references)+2)
	if chat != nil && chat.SupportConversationID != nil && strings.TrimSpace(*chat.SupportConversationID) != "" {
		contexts = append(contexts, model.AgentRunContextReference{EntityType: "support_conversation", EntityID: strings.TrimSpace(*chat.SupportConversationID)})
	}
	if pageContext != nil {
		entityType, _ := pageContext["entity_type"].(string)
		entityID, _ := pageContext["entity_id"].(string)
		if entityType, entityID = strings.TrimSpace(entityType), strings.TrimSpace(entityID); entityType != "" && entityID != "" {
			contexts = append(contexts, model.AgentRunContextReference{EntityType: entityType, EntityID: entityID})
		}
	}
	for _, reference := range references {
		contexts = append(contexts, model.AgentRunContextReference{EntityType: reference.EntityType, EntityID: reference.EntityID})
	}
	seen := make(map[string]struct{}, len(contexts))
	unique := contexts[:0]
	for _, context := range contexts {
		key := context.EntityType + ":" + context.EntityID
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, context)
	}
	return unique
}

func (s *DockChatService) setRunAttachedContexts(ctx context.Context, run *model.AgentRun, contexts []model.AgentRunContextReference) error {
	if s == nil || s.runRepo == nil || run == nil {
		return nil
	}
	var input model.AgentRunInputPayload
	if err := json.Unmarshal(run.Input, &input); err != nil {
		return fmt.Errorf("decode chat run input: %w", err)
	}
	input.AttachedContexts = contexts
	updated, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("encode chat run input: %w", err)
	}
	if err := s.runRepo.UpdateInput(ctx, run.WorkspaceID, run.ID, updated); err != nil {
		return err
	}
	run.Input = updated
	return nil
}

// scopedChatTools narrows the ask_agent preset tools to the requesting user's
// module access: command-backed tools are kept only when the user's role
// grants the command's module permission. Non-command tools (web research,
// interaction tools) are always kept.
func (s *DockChatService) scopedChatTools(ctx context.Context, workspaceID, userID string, agent *model.Agent) ([]string, error) {
	agentTools := parseJSONStringSlice(agent.AllowedTools)
	if s.authz == nil || s.commandService == nil {
		return agentTools, nil
	}
	actor, err := s.authz.ResolveActor(ctx, workspaceID, userID)
	if err != nil {
		return nil, fmt.Errorf("resolve chat actor: %w", err)
	}

	defsByAlias := make(map[string]InternalCommandDefinition)
	for _, def := range s.commandService.ToolDefinitions() {
		if def.Tool != nil && strings.TrimSpace(def.Tool.Alias) != "" {
			defsByAlias[strings.TrimSpace(def.Tool.Alias)] = def
		}
	}

	scoped := make([]string, 0, len(agentTools))
	for _, tool := range agentTools {
		def, isCommand := defsByAlias[strings.TrimSpace(tool)]
		if !isCommand {
			scoped = append(scoped, tool)
			continue
		}
		perms := commandPermissionsForDefinition(def)
		if len(perms) == 0 || s.authz.CanAny(actor, perms...) {
			scoped = append(scoped, tool)
		}
	}
	return scoped, nil
}

// buildCarryForward renders a compact context block from the previous backing
// run so a successor run keeps the thread of the conversation.
func (s *DockChatService) buildCarryForward(ctx context.Context, previousRun *model.AgentRun) string {
	if previousRun == nil {
		return ""
	}
	var messages []model.AgentRunMessage
	var err error
	if s.runMessageRepo != nil {
		messages, err = s.runMessageRepo.ListByRun(ctx, previousRun.WorkspaceID, previousRun.ID)
	}
	if err != nil {
		messages = nil
	}
	if len(messages) > dockChatCarryForwardTurns {
		messages = messages[len(messages)-dockChatCarryForwardTurns:]
	}

	var b strings.Builder
	b.WriteString("<previous_conversation>\n")
	b.WriteString("The previous execution workspace is unavailable in this successor run. Files and packages do not transfer. Published artifacts remain available. Never repeat an external mutation to reconstruct missing files or results. An interrupted command, push or PR may have completed externally; verify remote state and report uncertainty before retrying.\n")
	var summary map[string]json.RawMessage
	if json.Unmarshal(previousRun.OutputSummary, &summary) == nil && len(summary["interrupted_external_effects"]) > 0 {
		b.WriteString("Interrupted operations (started, outcome unknown): ")
		b.Write(summary["interrupted_external_effects"])
		b.WriteString("\n")
	}
	b.WriteString("This chat continues an earlier conversation whose run ended (")
	b.WriteString(strings.TrimSpace(previousRun.Status))
	b.WriteString("). Recent transcript:\n")
	total := 0
	for _, message := range messages {
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		if len(content) > dockChatCarryForwardChars {
			content = content[:dockChatCarryForwardChars] + "…"
		}
		line := message.Role + ": " + content + "\n"
		if total+len(line) > dockChatCarryForwardTotal {
			break
		}
		b.WriteString(line)
		total += len(line)
	}
	b.WriteString("</previous_conversation>")
	return b.String()
}

type dockChatMediaAttachment struct {
	ID       string `json:"id"`
	FileName string `json:"file_name"`
	FileType string `json:"file_type"`
	FileSize int64  `json:"file_size"`
	Source   string `json:"source,omitempty"`
	URL      string `json:"-"`
}

func composeDockChatTurn(content string, pageContext map[string]interface{}, references []model.DockEntityReference, attachments, sourceAttachments []dockChatMediaAttachment, analysis string) string {
	blocks := []string{content}
	if len(pageContext) > 0 {
		if encoded, err := json.Marshal(pageContext); err == nil {
			blocks = append(blocks, dockChatPageContextOpenTag+string(encoded)+"</page_context>")
		}
	}
	if len(references) > 0 {
		if encoded, err := json.Marshal(references); err == nil {
			blocks = append(blocks, dockChatReferencesOpenTag+string(encoded)+"</references>")
		}
	}
	if len(attachments) > 0 {
		if encoded, err := json.Marshal(attachments); err == nil {
			blocks = append(blocks, dockChatAttachmentsOpenTag+string(encoded)+"</attachments>")
		}
	}
	if len(sourceAttachments) > 0 {
		if encoded, err := json.Marshal(sourceAttachments); err == nil {
			blocks = append(blocks, dockChatSourceAttachmentsOpenTag+string(encoded)+"</source_attachments>")
		}
	}
	if analysis = strings.TrimSpace(analysis); analysis != "" {
		blocks = append(blocks, dockChatAttachmentAnalysisOpenTag+analysis+"</attachment_analysis>")
	}
	return strings.Join(blocks, "\n\n")
}

func (s *DockChatService) resolveDockChatSourceMediaAttachments(ctx context.Context, workspaceID string, pageContext map[string]interface{}, references []model.DockEntityReference) ([]dockChatMediaAttachment, error) {
	type source struct{ EntityType, EntityID string }
	sources := make([]source, 0, len(references)+1)
	if pageContext != nil {
		entityType, _ := pageContext["entity_type"].(string)
		entityID, _ := pageContext["entity_id"].(string)
		if strings.TrimSpace(entityType) != "" && strings.TrimSpace(entityID) != "" {
			sources = append(sources, source{entityType, entityID})
		}
	}
	for _, ref := range references {
		sources = append(sources, source{ref.EntityType, ref.EntityID})
	}

	seen := map[string]struct{}{}
	result := make([]dockChatMediaAttachment, 0)
	for _, item := range sources {
		key := item.EntityType + ":" + item.EntityID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if item.EntityType == "support_conversation" && s.supportInboxService != nil {
			messages, err := s.supportInboxService.ListConversationMessages(ctx, workspaceID, item.EntityID, false)
			if err != nil {
				return nil, err
			}
			seenExternalURLs := map[string]struct{}{}
			for _, message := range messages {
				for _, attachment := range message.Attachments {
					if !isDockChatMediaType(attachment.FileType) || isDecorativeDockChatAsset(attachment.FileName, attachment.FileSize) || strings.TrimSpace(attachment.URL) == "" {
						continue
					}
					result = append(result, dockChatMediaAttachment{ID: attachment.ID, FileName: attachment.FileName, FileType: attachment.FileType, FileSize: attachment.FileSize, Source: "support_conversation", URL: attachment.URL})
				}
				for _, externalURL := range dockChatHostedImageURLs(message) {
					if _, ok := seenExternalURLs[externalURL]; ok {
						continue
					}
					seenExternalURLs[externalURL] = struct{}{}
					digest := sha256.Sum256([]byte(externalURL))
					result = append(result, dockChatMediaAttachment{ID: fmt.Sprintf("external-%x", digest[:8]), FileName: "Hosted image", Source: "hosted_link", URL: externalURL})
				}
			}
			continue
		}
		if s.pmAttachmentRepo == nil || s.pmAttachmentService == nil {
			continue
		}
		entityType := item.EntityType
		if entityType == "document" {
			entityType = entityTypeEditorUpload
		}
		attachments, err := s.pmAttachmentRepo.List(ctx, entityType, item.EntityID)
		if err != nil {
			return nil, err
		}
		for _, attachment := range attachments {
			if attachment.WorkspaceID != workspaceID || !isDockChatMediaType(attachment.ContentType) || isDecorativeDockChatAsset(attachment.FileName, attachment.FileSize) {
				continue
			}
			url, err := s.pmAttachmentService.ContentURL(ctx, attachment.ID)
			if err != nil {
				continue
			}
			result = append(result, dockChatMediaAttachment{ID: attachment.ID, FileName: attachment.FileName, FileType: attachment.ContentType, FileSize: attachment.FileSize, Source: item.EntityType, URL: url})
		}
	}
	return s.hydrateDockChatExternalImages(ctx, result)
}

func dockChatHostedImageURLs(message model.SupportMessage) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0)
	add := func(raw string) {
		raw = strings.TrimSpace(raw)
		if !isDockChatExternalMediaURL(raw) {
			return
		}
		if _, ok := seen[raw]; ok {
			return
		}
		seen[raw] = struct{}{}
		result = append(result, raw)
	}
	for _, raw := range supportPreviewURLPattern.FindAllString(message.Content, -1) {
		add(trimSupportPreviewURL(raw))
	}
	var metadata struct {
		LinkPreviews []model.SupportLinkPreview `json:"link_previews"`
	}
	if json.Unmarshal([]byte(message.Metadata), &metadata) == nil {
		for _, preview := range metadata.LinkPreviews {
			if preview.ImageURL != nil {
				add(*preview.ImageURL)
			}
		}
	}
	return result
}

func isDecorativeDockChatAsset(fileName string, size int64) bool {
	if size > 15*1024 {
		return false
	}
	name := strings.ToLower(strings.TrimSpace(fileName))
	for _, marker := range []string{"logo", "signature", "spacer", "tracking", "pixel", "icon"} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func (s *DockChatService) resolveDockChatMediaAttachments(ctx context.Context, workspaceID, userID string, ids []string) ([]dockChatMediaAttachment, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if s.pmAttachmentRepo == nil {
		return nil, fmt.Errorf("Ask media attachments are not configured")
	}
	result := make([]dockChatMediaAttachment, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		attachment, err := s.pmAttachmentRepo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if attachment == nil || attachment.WorkspaceID != workspaceID || attachment.UploadedByID != userID || !attachment.IsUploaded || attachment.EntityType != entityTypeEditorUpload {
			return nil, fmt.Errorf("Ask media attachment is unavailable")
		}
		if !isDockChatAttachmentType(attachment.ContentType) {
			return nil, fmt.Errorf("Ask does not support this file type")
		}
		if attachment.FileSize > 20*1024*1024 {
			return nil, fmt.Errorf("Ask media attachments must be 20 MB or smaller")
		}
		result = append(result, dockChatMediaAttachment{ID: attachment.ID, FileName: attachment.FileName, FileType: attachment.ContentType, FileSize: attachment.FileSize})
	}
	return result, nil
}

func isDockChatAttachmentType(contentType string) bool {
	if isDockChatMediaType(contentType) {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "application/pdf", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "application/json", "text/plain", "text/markdown", "text/x-markdown", "text/csv":
		return true
	default:
		return false
	}
}

func isDockChatMediaType(contentType string) bool {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "video/mp4", "video/quicktime", "video/webm", "video/mpeg":
		return true
	default:
		return false
	}
}

func normalizeDockChatReferences(references []model.DockEntityReference) ([]model.DockEntityReference, error) {
	if len(references) > dockChatReferencesMax {
		return nil, fmt.Errorf("at most %d references are allowed", dockChatReferencesMax)
	}
	allowedTypes := map[string]struct{}{
		"task": {}, "epic": {}, "document": {}, "crm_contact": {}, "crm_deal": {},
	}
	seen := make(map[string]struct{}, len(references))
	normalized := make([]model.DockEntityReference, 0, len(references))
	for _, reference := range references {
		reference.EntityType = strings.TrimSpace(reference.EntityType)
		reference.EntityID = strings.TrimSpace(reference.EntityID)
		reference.DisplayTitle = strings.TrimSpace(reference.DisplayTitle)
		if _, ok := allowedTypes[reference.EntityType]; !ok {
			return nil, fmt.Errorf("unsupported reference type %q", reference.EntityType)
		}
		if reference.EntityID == "" || reference.DisplayTitle == "" {
			return nil, fmt.Errorf("reference entity_id and display_title are required")
		}
		key := reference.EntityType + ":" + reference.EntityID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, reference)
	}
	return normalized, nil
}

func dockChatTitleFromContent(content string) string {
	title := strings.TrimSpace(content)
	if idx := strings.IndexAny(title, "\r\n"); idx >= 0 {
		title = strings.TrimSpace(title[:idx])
	}
	runes := []rune(title)
	if len(runes) > dockChatTitleMaxRunes {
		title = string(runes[:dockChatTitleMaxRunes]) + "…"
	}
	return title
}

// isChatRunExpiredError matches the runtime's idle-expiry resume error
// ("run idle timeout expired"), after which the runtime completes the run.
func isChatRunExpiredError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "idle timeout")
}
