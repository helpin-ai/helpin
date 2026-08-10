package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// ErrDockChatNotFound is returned when a chat does not exist or is not owned
// by the requesting user.
var ErrDockChatNotFound = errors.New("dock chat not found")

// ErrDockChatInvalidCursor is returned for malformed list pagination cursors.
var ErrDockChatInvalidCursor = errors.New("invalid dock chat cursor")

const (
	dockChatTriggerType        = "dock_chat"
	dockChatTitleMaxRunes      = 60
	dockChatCarryForwardTurns  = 20
	dockChatCarryForwardChars  = 500
	dockChatCarryForwardTotal  = 6000
	dockChatPageContextOpenTag = "<page_context>"
	dockChatReferencesOpenTag  = "<references>"
	dockChatReferencesMax      = 10
	dockChatListDefaultLimit   = 30
	dockChatListMaxLimit       = 50
)

type dockChatCursor struct {
	ActivityAt time.Time `json:"activity_at"`
	ID         string    `json:"id"`
}

// DockChatService owns dock chats: user-scoped conversations whose turns are
// executed by an agent-runtime chat-mode run of the ask_agent preset.
type DockChatService struct {
	chatRepo       *repository.DockChatRepository
	runRepo        *repository.AgentRunRepository
	runMessageRepo *repository.AgentRunMessageRepository
	planRepo       *repository.CommandBarPlanRepository
	agentService   *AgentService
	commandService *InternalCommandService
	authz          *authorization.AuthzService
	titleLLM       dockChatTitleLLM
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
		chatRepo:       chatRepo,
		runRepo:        runRepo,
		runMessageRepo: runMessageRepo,
		planRepo:       planRepo,
		agentService:   agentService,
		commandService: commandService,
		authz:          authz,
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

	chats, err := s.chatRepo.ListByWorkspaceUser(ctx, workspaceID, userID, limit+1, before, beforeID)
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

// CreateChat creates an empty chat; its backing run starts lazily on the
// first message.
func (s *DockChatService) CreateChat(ctx context.Context, workspaceID, userID string, req model.CreateDockChatRequest) (*model.DockChat, error) {
	chat := &model.DockChat{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Title:       strings.TrimSpace(req.Title),
	}
	if err := s.chatRepo.Create(ctx, chat); err != nil {
		return nil, fmt.Errorf("create dock chat: %w", err)
	}
	return chat, nil
}

// UpdateChat renames or archives/unarchives a chat.
func (s *DockChatService) UpdateChat(ctx context.Context, workspaceID, userID, chatID string, req model.UpdateDockChatRequest) (*model.DockChat, error) {
	chat, err := s.ownedChat(ctx, workspaceID, userID, chatID)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
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
	chat, err := s.ownedChat(ctx, workspaceID, userID, chatID)
	if err != nil {
		return nil, err
	}
	return s.chatDetail(ctx, chat)
}

// ActiveRunForChat resolves the chat's current backing run after an ownership
// check. Handlers use it to proxy run-scoped reads (snapshot, events, cancel)
// without granting the caller access to arbitrary runs.
func (s *DockChatService) ActiveRunForChat(ctx context.Context, workspaceID, userID, chatID string) (*model.AgentRun, error) {
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
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}
	chat, err := s.ownedChat(ctx, workspaceID, userID, chatID)
	if err != nil {
		return nil, err
	}

	references, err := normalizeDockChatReferences(req.References)
	if err != nil {
		return nil, err
	}
	composed := composeDockChatTurn(content, req.PageContext, references)

	var currentRun *model.AgentRun
	if chat.ActiveRunID != nil {
		currentRun, err = s.runRepo.GetByID(ctx, workspaceID, *chat.ActiveRunID)
		if err != nil {
			return nil, fmt.Errorf("get chat run: %w", err)
		}
	}

	switch {
	case currentRun == nil || !model.IsAgentRunActiveStatus(currentRun.Status):
		// First message, or the previous backing run ended.
		if err := s.startChatRun(ctx, chat, userID, composed, currentRun); err != nil {
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
			if err := s.startChatRun(ctx, chat, userID, composed, currentRun); err != nil {
				return nil, err
			}
		} else if _, err := s.agentService.SendRunMessage(ctx, workspaceID, currentRun.ID, userID, model.SendAgentRunMessageRequest{Content: composed}); err != nil {
			if !isChatRunExpiredError(err) {
				return nil, err
			}
			// The runtime idle-expired the run; it is completed on its side.
			// Continue the conversation through a successor run.
			if err := s.startChatRun(ctx, chat, userID, composed, currentRun); err != nil {
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
	return s.chatDetail(ctx, chat)
}

func (s *DockChatService) runUsesCurrentScopedTools(ctx context.Context, chat *model.DockChat, userID string, run *model.AgentRun) (bool, error) {
	if chat == nil || run == nil {
		return false, nil
	}
	agent, err := s.agentService.ensureBuiltInAgent(ctx, chat.WorkspaceID, userID, model.AgentPresetAskAgent)
	if err != nil {
		return false, fmt.Errorf("ensure ask agent for tool contract: %w", err)
	}
	current, err := s.scopedChatTools(ctx, chat.WorkspaceID, userID, agent)
	if err != nil {
		return false, err
	}
	var input model.AgentRunInputPayload
	if err := json.Unmarshal(run.Input, &input); err != nil {
		return false, nil
	}
	return sameNormalizedToolSet(input.AllowedTools, current), nil
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

// startChatRun starts a (possibly successor) backing run for the chat and
// repoints the chat at it.
func (s *DockChatService) startChatRun(ctx context.Context, chat *model.DockChat, userID, composedTurn string, previousRun *model.AgentRun) error {
	agent, err := s.agentService.ensureBuiltInAgent(ctx, chat.WorkspaceID, userID, model.AgentPresetAskAgent)
	if err != nil {
		return fmt.Errorf("ensure ask agent: %w", err)
	}
	allowedTools, err := s.scopedChatTools(ctx, chat.WorkspaceID, userID, agent)
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
	trigger := &model.AgentRunTriggerContext{
		Source:      model.AgentRunTriggerSourceManual,
		TriggerType: dockChatTriggerType,
		ActorID:     &userID,
		FiredAt:     &now,
		Context:     json.RawMessage(fmt.Sprintf(`{"dock_chat_id":%q}`, chat.ID)),
	}

	run, err := s.agentService.startTargetRunWithOptions(
		ctx,
		chat.WorkspaceID,
		"workspace",
		chat.WorkspaceID,
		model.StartAgentRunRequest{
			AgentID:           agent.ID,
			AdditionalContext: &additional,
			AllowedTools:      allowedTools,
		},
		&userID,
		trigger,
		nil,
		parentRunID,
		startTargetRunOptions{dockChatID: &chat.ID},
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
	messages, err := s.runMessageRepo.ListByRun(ctx, previousRun.WorkspaceID, previousRun.ID)
	if err != nil {
		messages = nil
	}
	if len(messages) > dockChatCarryForwardTurns {
		messages = messages[len(messages)-dockChatCarryForwardTurns:]
	}

	var b strings.Builder
	b.WriteString("<previous_conversation>\n")
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

func composeDockChatTurn(content string, pageContext map[string]interface{}, references []model.DockEntityReference) string {
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
	return strings.Join(blocks, "\n\n")
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
