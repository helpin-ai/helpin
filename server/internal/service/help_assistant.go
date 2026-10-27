package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// Help assistant limits. Per-IP limits live in the route middleware.
const (
	helpAssistantMaxQuestionRunes = 1000
	helpAssistantSessionTurns     = 20
	helpAssistantDailyQuestions   = 500
	// helpAssistantConcurrentTurns caps a workspace's turns being answered at
	// once. Lightweight runs have no concurrency limit in Agent Runtime.
	helpAssistantConcurrentTurns = 5
	// helpAssistantTurnTimeout fails a turn that has not been answered; a
	// lightweight run lost to a Runtime restart is never recovered.
	helpAssistantTurnTimeout     = 75 * time.Second
	helpAssistantTranscriptTurns = 6
	helpAssistantTriggerType     = "help_assistant"
	helpAssistantStartLabel      = "Searching the help articles…"
)

// Help assistant errors.
var (
	ErrHelpAssistantUnavailable = errors.New("the help assistant is not available")
	ErrHelpAssistantInvalid     = errors.New("invalid help assistant request")
	ErrHelpAssistantNotFound    = errors.New("help assistant conversation not found")
	ErrHelpAssistantBusy        = errors.New("the help assistant is busy, try again in a moment")
	ErrHelpAssistantLimit       = errors.New("the help assistant cannot take more questions right now")
	ErrHelpAssistantPending     = errors.New("the previous question is still being answered")
	ErrHelpAssistantSessionFull = errors.New("this conversation is full, start a new one")
)

var helpAssistantVisitorID = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

// HelpAssistantService runs the help assistant: one agent run per question,
// answering from published help articles in the session's audience.
type HelpAssistantService struct {
	repo         *repository.HelpAssistantRepository
	agents       *AgentService
	inbox        *SupportInboxService
	hcRepo       *repository.DocsHelpcenterRepository
	publications *repository.DocsHelpcenterPublicationRepository
	chunks       *repository.DocsChunkRepository
	knowledge    *HelpcenterAISearchService
	evidence     *repository.SupportRunEvidenceRepository
	now          func() time.Time
}

// NewHelpAssistantService creates a HelpAssistantService.
func NewHelpAssistantService(
	repo *repository.HelpAssistantRepository,
	agents *AgentService,
	inbox *SupportInboxService,
	hcRepo *repository.DocsHelpcenterRepository,
	publications *repository.DocsHelpcenterPublicationRepository,
	chunks *repository.DocsChunkRepository,
	knowledge *HelpcenterAISearchService,
	evidence *repository.SupportRunEvidenceRepository,
) *HelpAssistantService {
	return &HelpAssistantService{
		repo: repo, agents: agents, inbox: inbox, hcRepo: hcRepo, publications: publications,
		chunks: chunks, knowledge: knowledge, evidence: evidence, now: time.Now,
	}
}

// HelpAssistantAsker identifies who asks: the surface, the visitor's browser,
// and, for a customer signed in to the portal, their portal identity, which
// adds the help spaces for signed-in customers.
type HelpAssistantAsker struct {
	WorkspaceID      string
	Surface          string
	VisitorID        string
	PortalIdentityID string
}

func (a HelpAssistantAsker) audience() string {
	if a.PortalIdentityID != "" {
		return model.HelpcenterAudienceCustomers
	}
	return model.HelpcenterAudiencePublic
}

// HelpAssistantQuestion is a question in a new or existing session.
type HelpAssistantQuestion struct {
	SessionID     string `json:"session_id"`
	Message       string `json:"message"`
	PageArticleID string `json:"page_article_id"`
}

// HelpAssistantTurn identifies a question being answered.
type HelpAssistantTurn struct {
	SessionID         string `json:"session_id"`
	QuestionID        string `json:"question_id"`
	AnswerID          string `json:"answer_id"`
	TurnsLeft         int    `json:"turns_left"`
	ConversationEnded bool   `json:"conversation_ended,omitempty"`
}

// HelpAssistantMessageView is a message as the asker sees it.
type HelpAssistantMessageView struct {
	ID            string                        `json:"id"`
	Role          string                        `json:"role"`
	Content       string                        `json:"content"`
	Status        string                        `json:"status"`
	ProgressLabel string                        `json:"progress_label,omitempty"`
	Citations     []model.HelpAssistantCitation `json:"citations"`
	Helpful       *bool                         `json:"helpful,omitempty"`
	CreatedAt     time.Time                     `json:"created_at"`
}

// HelpAssistantSessionView is a session as the asker sees it.
type HelpAssistantSessionView struct {
	ID        string                     `json:"id"`
	Messages  []HelpAssistantMessageView `json:"messages"`
	TurnsLeft int                        `json:"turns_left"`
}

// Available reports whether the workspace offers the help assistant: it is
// turned on, the help center is published, and agents can run.
func (s *HelpAssistantService) Available(ctx context.Context, workspaceID string) bool {
	if s == nil || s.agents == nil || !s.agents.agentRuntimeLaunchEnabled || s.inbox == nil || s.hcRepo == nil {
		return false
	}
	_, settings, err := s.inbox.GetInstallation(ctx, workspaceID)
	if err != nil || settings == nil || !settings.HelpAssistantEnabled {
		return false
	}
	cfg, err := s.hcRepo.GetConfig(ctx, workspaceID)
	return err == nil && cfg != nil && cfg.IsPublished
}

// Ask stores a question and starts the run that answers it. The caller polls
// Session for the answer.
func (s *HelpAssistantService) Ask(ctx context.Context, asker HelpAssistantAsker, question HelpAssistantQuestion) (*HelpAssistantTurn, error) {
	text := strings.TrimSpace(question.Message)
	if n := utf8.RuneCountInString(text); n < 2 || n > helpAssistantMaxQuestionRunes || !helpAssistantVisitorID.MatchString(asker.VisitorID) {
		return nil, fmt.Errorf("%w: ask a question of up to %d characters", ErrHelpAssistantInvalid, helpAssistantMaxQuestionRunes)
	}
	if !s.Available(ctx, asker.WorkspaceID) {
		return nil, ErrHelpAssistantUnavailable
	}
	agent, err := s.agents.ensureBuiltInAgent(ctx, asker.WorkspaceID, "", model.AgentPresetHelpAssistant)
	if err != nil {
		return nil, err
	}
	agent, err = s.agents.requireRunnableAgent(ctx, asker.WorkspaceID, agent.ID, model.HelpAssistantTurnTarget)
	if err != nil {
		return nil, err
	}
	if agent.MonthlyTokenBudget != nil && agent.TokensUsedThisMonth >= *agent.MonthlyTokenBudget {
		return nil, ErrHelpAssistantLimit
	}

	session, err := s.sessionFor(ctx, asker, question.SessionID)
	if err != nil {
		return nil, err
	}
	if session.TurnCount >= helpAssistantSessionTurns {
		return nil, ErrHelpAssistantSessionFull
	}
	now := s.now()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if asked, err := s.repo.CountQuestionsSince(ctx, asker.WorkspaceID, day); err != nil {
		return nil, err
	} else if asked >= helpAssistantDailyQuestions {
		return nil, ErrHelpAssistantLimit
	}
	if pending, err := s.repo.CountPending(ctx, asker.WorkspaceID, now.Add(-helpAssistantTurnTimeout)); err != nil {
		return nil, err
	} else if pending >= helpAssistantConcurrentTurns {
		return nil, ErrHelpAssistantBusy
	}

	history, err := s.repo.ListMessages(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	label := helpAssistantStartLabel
	pageArticleID := strings.TrimSpace(question.PageArticleID)
	userMessage := &model.HelpAssistantMessage{SessionID: session.ID, WorkspaceID: session.WorkspaceID, Role: model.HelpAssistantRoleUser, Content: text, Status: model.HelpAssistantStatusAnswered}
	if pageArticleID != "" {
		userMessage.PageArticleID = &pageArticleID
	}
	answer := &model.HelpAssistantMessage{SessionID: session.ID, WorkspaceID: session.WorkspaceID, Role: model.HelpAssistantRoleAssistant, Status: model.HelpAssistantStatusPending, ProgressLabel: &label}
	if err := s.repo.StartTurn(ctx, session, userMessage, answer); err != nil {
		if errors.Is(err, repository.ErrHelpAssistantTurnPending) {
			return nil, ErrHelpAssistantPending
		}
		return nil, err
	}

	runID, err := s.startRun(ctx, agent, session, answer, history, text, pageArticleID)
	if err != nil {
		slog.ErrorContext(ctx, "help assistant run did not start", "workspace_id", session.WorkspaceID, "session_id", session.ID, "error", err)
		if _, completeErr := s.repo.Complete(ctx, answer.ID, model.HelpAssistantStatusFailed, "", nil); completeErr != nil {
			slog.ErrorContext(ctx, "help assistant turn could not be closed", "answer_id", answer.ID, "error", completeErr)
		}
		return nil, fmt.Errorf("%w: %v", ErrHelpAssistantUnavailable, err)
	}
	if err := s.repo.SetRunID(ctx, answer.ID, runID); err != nil {
		slog.ErrorContext(ctx, "help assistant run id not saved", "answer_id", answer.ID, "run_id", runID, "error", err)
	}
	slog.InfoContext(ctx, "help assistant question", "workspace_id", session.WorkspaceID, "session_id", session.ID, "surface", session.Surface, "audience", session.Audience, "run_id", runID)
	return &HelpAssistantTurn{SessionID: session.ID, QuestionID: userMessage.ID, AnswerID: answer.ID, TurnsLeft: helpAssistantSessionTurns - session.TurnCount}, nil
}

// sessionFor returns the asker's session, creating one when sessionID is
// empty. A session belongs to the visitor's browser and, in the portal, to the
// signed-in customer; anyone else gets ErrHelpAssistantNotFound.
func (s *HelpAssistantService) sessionFor(ctx context.Context, asker HelpAssistantAsker, sessionID string) (*model.HelpAssistantSession, error) {
	if sessionID = strings.TrimSpace(sessionID); sessionID != "" {
		return s.ownedSession(ctx, asker, sessionID)
	}
	locale := ""
	if cfg, err := s.hcRepo.GetConfig(ctx, asker.WorkspaceID); err == nil && cfg != nil {
		locale = defaultHelpcenterLocale(cfg)
	}
	session := &model.HelpAssistantSession{WorkspaceID: asker.WorkspaceID, Surface: asker.Surface, Audience: asker.audience(), VisitorID: asker.VisitorID, Locale: locale}
	if asker.PortalIdentityID != "" {
		identity := asker.PortalIdentityID
		session.PortalIdentityID = &identity
	}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *HelpAssistantService) ownedSession(ctx context.Context, asker HelpAssistantAsker, sessionID string) (*model.HelpAssistantSession, error) {
	if _, err := uuid.Parse(sessionID); err != nil {
		return nil, ErrHelpAssistantNotFound
	}
	session, err := s.repo.GetSession(ctx, asker.WorkspaceID, sessionID)
	if err != nil {
		return nil, err
	}
	// A session keeps the audience it started with: signing in or out starts
	// a new conversation rather than widening an old one.
	if session == nil || session.VisitorID != asker.VisitorID || session.Surface != asker.Surface ||
		derefString(session.PortalIdentityID) != asker.PortalIdentityID || session.Audience != asker.audience() {
		return nil, ErrHelpAssistantNotFound
	}
	return session, nil
}

// startRun launches the lightweight run that answers one question.
func (s *HelpAssistantService) startRun(ctx context.Context, agent *model.Agent, session *model.HelpAssistantSession, answer *model.HelpAssistantMessage, history []model.HelpAssistantMessage, question, pageArticleID string) (string, error) {
	trigger := &model.AgentRunTriggerContext{Source: model.AgentRunTriggerSourceSystem, TriggerType: helpAssistantTriggerType}
	input := model.AgentRunInputPayload{
		Trigger:           trigger,
		Target:            &model.AgentRunTargetContext{TargetType: model.HelpAssistantTurnTarget, TargetID: answer.ID},
		AllowedTools:      append([]string(nil), agentcontract.HelpAssistantTools...),
		AdditionalContext: s.turnContext(ctx, session, history, question, pageArticleID),
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	run, err := s.agents.createRun(ctx, createRunParams{
		runID: uuid.NewString(), workspaceID: session.WorkspaceID, agent: agent,
		targetType: model.HelpAssistantTurnTarget, targetID: answer.ID, trigger: trigger,
		input: encoded, invocationMode: model.InvocationModeInteractive,
	})
	if err != nil {
		return "", err
	}
	return run.ID, nil
}

// turnContext gives the run the recent conversation, the article the person
// is reading, and the new question.
func (s *HelpAssistantService) turnContext(ctx context.Context, session *model.HelpAssistantSession, history []model.HelpAssistantMessage, question, pageArticleID string) string {
	var b strings.Builder
	if session.Audience == model.HelpcenterAudienceCustomers {
		b.WriteString("The person is a customer signed in to the customer portal.\n")
	} else {
		b.WriteString("The person is a visitor on the public help center.\n")
	}
	var turns []string
	for _, message := range history {
		if message.Role == model.HelpAssistantRoleUser {
			turns = append(turns, "Person: "+truncateRunes(message.Content, 600))
		} else if message.Status == model.HelpAssistantStatusAnswered || message.Status == model.HelpAssistantStatusHandedOff {
			turns = append(turns, "You: "+truncateRunes(message.Content, 900))
		}
	}
	if len(turns) > helpAssistantTranscriptTurns*2 {
		turns = turns[len(turns)-helpAssistantTranscriptTurns*2:]
	}
	if len(turns) > 0 {
		b.WriteString("\nConversation so far:\n" + strings.Join(turns, "\n") + "\n")
	}
	if pageArticleID != "" {
		if title := s.articleTitle(ctx, session, pageArticleID); title != "" {
			fmt.Fprintf(&b, "\nThe person is reading the article %q (article_id %s). Questions like \"this page\" refer to it.\n", title, pageArticleID)
		}
	}
	b.WriteString("\nNew question:\n" + question)
	return b.String()
}

// Session returns a session's messages. An answer that has waited past the
// turn timeout is closed as failed and its run cancelled.
func (s *HelpAssistantService) Session(ctx context.Context, asker HelpAssistantAsker, sessionID string) (*HelpAssistantSessionView, error) {
	if s == nil {
		return nil, ErrHelpAssistantUnavailable
	}
	session, err := s.ownedSession(ctx, asker, sessionID)
	if err != nil {
		return nil, err
	}
	messages, err := s.repo.ListMessages(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	view := &HelpAssistantSessionView{ID: session.ID, Messages: make([]HelpAssistantMessageView, 0, len(messages)), TurnsLeft: max(0, helpAssistantSessionTurns-session.TurnCount)}
	for _, message := range messages {
		if message.Status == model.HelpAssistantStatusPending && s.now().Sub(message.CreatedAt) > helpAssistantTurnTimeout {
			s.expireTurn(ctx, &message)
		}
		view.Messages = append(view.Messages, helpAssistantMessageView(message))
	}
	return view, nil
}

func (s *HelpAssistantService) expireTurn(ctx context.Context, message *model.HelpAssistantMessage) {
	closed, err := s.repo.Complete(ctx, message.ID, model.HelpAssistantStatusFailed, "", nil)
	if err != nil {
		slog.ErrorContext(ctx, "help assistant turn timeout not saved", "answer_id", message.ID, "error", err)
		return
	}
	message.Status = model.HelpAssistantStatusFailed
	message.ProgressLabel = nil
	if !closed || message.RunID == nil {
		return
	}
	slog.WarnContext(ctx, "help assistant turn timed out", "workspace_id", message.WorkspaceID, "run_id", *message.RunID)
	if _, err := s.agents.CancelRun(ctx, message.WorkspaceID, *message.RunID, ""); err != nil {
		slog.WarnContext(ctx, "help assistant run not cancelled", "run_id", *message.RunID, "error", err)
	}
}

// Feedback records whether an answer helped.
func (s *HelpAssistantService) Feedback(ctx context.Context, asker HelpAssistantAsker, sessionID, messageID string, helpful bool) error {
	if s == nil {
		return ErrHelpAssistantUnavailable
	}
	session, err := s.ownedSession(ctx, asker, sessionID)
	if err != nil {
		return err
	}
	if _, err := uuid.Parse(messageID); err != nil {
		return ErrHelpAssistantNotFound
	}
	saved, err := s.repo.SetFeedback(ctx, session.ID, messageID, helpful)
	if err != nil {
		return err
	}
	if !saved {
		return ErrHelpAssistantNotFound
	}
	return nil
}

// FinalizeRun closes a turn whose run ended without an answer.
func (s *HelpAssistantService) FinalizeRun(ctx context.Context, run *model.AgentRun) error {
	if s == nil || run == nil || run.TargetType != model.HelpAssistantTurnTarget {
		return nil
	}
	closed, err := s.repo.Complete(ctx, run.TargetID, model.HelpAssistantStatusFailed, "", nil)
	if err != nil {
		return err
	}
	if closed {
		slog.WarnContext(ctx, "help assistant run ended without an answer", "workspace_id", run.WorkspaceID, "run_id", run.ID, "status", run.Status)
	}
	return nil
}

func helpAssistantMessageView(message model.HelpAssistantMessage) HelpAssistantMessageView {
	view := HelpAssistantMessageView{
		ID: message.ID, Role: message.Role, Content: message.Content, Status: message.Status,
		ProgressLabel: derefString(message.ProgressLabel), Citations: []model.HelpAssistantCitation{},
		Helpful: message.Helpful, CreatedAt: message.CreatedAt,
	}
	if len(message.Citations) > 0 {
		if err := json.Unmarshal(message.Citations, &view.Citations); err != nil {
			view.Citations = []model.HelpAssistantCitation{}
		}
	}
	return view
}

func truncateRunes(text string, limit int) string {
	if runes := []rune(strings.TrimSpace(text)); len(runes) > limit {
		return string(runes[:limit]) + "…"
	}
	return strings.TrimSpace(text)
}
