package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// SupportInboxHandler handles internal support HTTP endpoints.
type SupportInboxHandler struct {
	supportService        *service.SupportInboxService
	agentService          *service.AgentService
	messageActionsService *service.SupportMessageActionsService
}

// NewSupportInboxHandler creates a new SupportInboxHandler.
func NewSupportInboxHandler(supportService *service.SupportInboxService, agentService *service.AgentService, messageActionsService *service.SupportMessageActionsService) *SupportInboxHandler {
	return &SupportInboxHandler{supportService: supportService, agentService: agentService, messageActionsService: messageActionsService}
}

// ListConversations handles GET /api/support/tickets.
func (h *SupportInboxHandler) ListConversations(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	userID := middleware.GetUserID(r.Context())
	search := strings.TrimSpace(r.URL.Query().Get("search"))

	// Mentions filter: return conversations where the user was @mentioned.
	if r.URL.Query().Get("filter") == "mentions" {
		resp, err := h.supportService.ListConversationsWithMentions(r.Context(), workspaceID, userID, search)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	status := r.URL.Query().Get("status")
	priority := r.URL.Query().Get("priority")
	aiState := r.URL.Query().Get("ai_state")
	flowState := r.URL.Query().Get("flow_state")
	var mailboxID *string
	if values, ok := r.URL.Query()["mailbox_id"]; ok {
		mailboxParam := strings.TrimSpace(values[0])
		if mailboxParam == "shared" || mailboxParam == "" {
			empty := ""
			mailboxID = &empty
		} else {
			mailboxID = &mailboxParam
		}
	}
	pagination := queryPagination(r)

	resp, err := h.supportService.ListConversationsWithMeta(r.Context(), workspaceID, userID, status, priority, pagination, mailboxID, flowState, search, aiState)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// GetConversation handles GET /api/support/tickets/{id}.
func (h *SupportInboxHandler) GetConversation(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	id := chi.URLParam(r, "id")

	conversation, err := h.supportService.GetConversation(r.Context(), workspaceID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, conversation)
}

// GetMessageEmailDetail handles GET /api/support/inbox/messages/{id}/email.
func (h *SupportInboxHandler) GetMessageEmailDetail(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "message id is required")
		return
	}

	detail, err := h.supportService.GetMessageEmailDetail(r.Context(), workspaceID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if detail == nil {
		writeError(w, http.StatusNotFound, "email details not found")
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// CreateConversation handles POST /api/support/tickets.
func (h *SupportInboxHandler) CreateConversation(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())

	var req model.CreateConversationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.WorkspaceID = workspaceID

	conversation, err := h.supportService.CreateConversation(r.Context(), req, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, conversation)
}

// UpdateConversationStatus handles PUT /api/support/tickets/{id}/status.
func (h *SupportInboxHandler) UpdateConversationStatus(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	ticketID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.UpdateConversationStatusRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	conversation, err := h.supportService.UpdateConversationStatus(r.Context(), workspaceID, ticketID, req.Status, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, conversation)
}

// ListConversationMessages handles GET /api/support/tickets/{id}/messages.
func (h *SupportInboxHandler) ListConversationMessages(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	ticketID := chi.URLParam(r, "id")

	messages, err := h.supportService.ListConversationMessages(r.Context(), workspaceID, ticketID, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if messages == nil {
		messages = []model.SupportMessage{}
	}
	writeJSON(w, http.StatusOK, messages)
}

// CreateConversationMessage handles POST /api/support/tickets/{id}/messages.
func (h *SupportInboxHandler) CreateConversationMessage(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	ticketID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.CreateMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	msg, err := h.supportService.CreateConversationMessage(r.Context(), workspaceID, ticketID, req, "user", &actorID, nil, nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, msg)
}

// DeleteMessage handles DELETE /api/support/inbox/conversations/{id}/messages/{msg_id}.
func (h *SupportInboxHandler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	if h.messageActionsService == nil {
		writeError(w, http.StatusInternalServerError, "message actions are not configured")
		return
	}
	workspaceID := getWorkspaceID(r)
	conversationID := chi.URLParam(r, "id")
	messageID := chi.URLParam(r, "msg_id")
	actorID := middleware.GetUserID(r.Context())
	undo := r.URL.Query().Get("undo") == "1" || strings.EqualFold(r.URL.Query().Get("undo"), "true")

	result, err := h.messageActionsService.Delete(r.Context(), workspaceID, conversationID, actorID, messageID, undo)
	if err != nil {
		h.writeMessageActionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GetMessageInfo handles GET /api/support/inbox/conversations/{id}/messages/{msg_id}.
func (h *SupportInboxHandler) GetMessageInfo(w http.ResponseWriter, r *http.Request) {
	if h.messageActionsService == nil {
		writeError(w, http.StatusInternalServerError, "message actions are not configured")
		return
	}
	workspaceID := getWorkspaceID(r)
	conversationID := chi.URLParam(r, "id")
	messageID := chi.URLParam(r, "msg_id")
	actorID := middleware.GetUserID(r.Context())

	info, err := h.messageActionsService.Info(r.Context(), workspaceID, conversationID, actorID, messageID)
	if err != nil {
		h.writeMessageActionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (h *SupportInboxHandler) writeMessageActionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrCancellableExpired):
		writeError(w, http.StatusGone, err.Error())
	case errors.Is(err, service.ErrSupportMessageActionForbidden):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, service.ErrSupportMessageActionNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

// LinkConversationStory handles POST /api/support/tickets/{id}/link-task.
func (h *SupportInboxHandler) LinkConversationStory(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	ticketID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.LinkStoryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	taskID := strings.TrimSpace(req.TaskID)
	if err := h.supportService.LinkConversationStory(r.Context(), workspaceID, ticketID, taskID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"linked": true})
}

// CreateTaskFromConversation handles POST /api/support/inbox/conversations/{id}/create-task.
func (h *SupportInboxHandler) CreateTaskFromConversation(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	conversationID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.CreateTaskFromConversationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	response, err := h.supportService.CreateTaskFromConversation(r.Context(), workspaceID, conversationID, actorID, req)
	if err != nil {
		if errors.Is(err, service.ErrSupportTaskInsufficientContext) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

// AssignConversationAgent handles POST /api/support/tickets/{id}/assign-agent.
func (h *SupportInboxHandler) AssignConversationAgent(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	ticketID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.AssignConversationAgentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.supportService.AssignConversationAgent(r.Context(), workspaceID, ticketID, req.AgentID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"assigned": true})
}

// AssignConversationUser handles POST /api/support/tickets/{id}/assign-user.
func (h *SupportInboxHandler) AssignConversationUser(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	ticketID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.AssignConversationUserRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.supportService.AssignConversationUser(r.Context(), workspaceID, ticketID, req.UserID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"assigned": true})
}

// UpdateConversationCRMContact handles PUT /api/support/inbox/conversations/{id}/crm-contact.
func (h *SupportInboxHandler) UpdateConversationCRMContact(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	conversationID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.UpdateConversationCRMContactRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	conversation, err := h.supportService.UpdateConversationCRMContact(r.Context(), workspaceID, conversationID, req.CRMContactID, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, conversation)
}

// ListConversationAssignableUsers handles GET /api/support/inbox/conversations/{id}/assignees.
func (h *SupportInboxHandler) ListConversationAssignableUsers(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	conversationID := chi.URLParam(r, "id")

	members, err := h.supportService.ListConversationAssignableUsers(r.Context(), workspaceID, conversationID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if members == nil {
		members = []model.AssignableMember{}
	}
	writeJSON(w, http.StatusOK, members)
}

// MarkConversationRead handles POST /api/support/inbox/conversations/{id}/read.
func (h *SupportInboxHandler) MarkConversationRead(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	conversationID := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	if err := h.supportService.MarkConversationRead(r.Context(), workspaceID, conversationID, userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// MarkConversationUnread handles POST /api/support/inbox/conversations/{id}/unread.
func (h *SupportInboxHandler) MarkConversationUnread(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	conversationID := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	if err := h.supportService.MarkConversationUnread(r.Context(), workspaceID, conversationID, userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UpdateConversationSubject handles PUT /api/support/inbox/conversations/{id}/subject.
func (h *SupportInboxHandler) UpdateConversationSubject(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	conversationID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req struct {
		Subject string `json:"subject"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	conv, err := h.supportService.UpdateConversationSubject(r.Context(), workspaceID, conversationID, req.Subject, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, conv)
}

// DeleteConversation handles DELETE /api/support/inbox/conversations/{id}.
func (h *SupportInboxHandler) DeleteConversation(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	conversationID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	if err := h.supportService.DeleteConversation(r.Context(), workspaceID, conversationID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetUnreadStats handles GET /api/support/inbox/unread-stats.
func (h *SupportInboxHandler) GetUnreadStats(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	userID := middleware.GetUserID(r.Context())
	var mailboxID *string
	if values, ok := r.URL.Query()["mailbox_id"]; ok {
		mailboxParam := strings.TrimSpace(values[0])
		if mailboxParam == "shared" || mailboxParam == "" {
			empty := ""
			mailboxID = &empty
		} else {
			mailboxID = &mailboxParam
		}
	}

	stats, err := h.supportService.GetUnreadStats(r.Context(), workspaceID, userID, mailboxID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *SupportInboxHandler) ListInboxScopes(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	resp, err := h.supportService.ListInboxScopes(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// ListWorkspaceUnread handles GET /api/support/workspace-unread.
// Returns per-workspace unread counts across all workspaces the caller is a member of.
// Used to render badges in the workspace switcher.
func (h *SupportInboxHandler) ListWorkspaceUnread(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	counts, err := h.supportService.ListUnreadByWorkspace(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, counts)
}

func (h *SupportInboxHandler) ListMailboxes(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	mailboxes, err := h.supportService.ListMailboxesAdmin(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, mailboxes)
}

func (h *SupportInboxHandler) CreateMailbox(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())

	var req model.CreateSupportMailboxRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	mailbox, err := h.supportService.CreateMailbox(r.Context(), workspaceID, req, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, mailbox)
}

func (h *SupportInboxHandler) UpdateMailbox(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	mailboxID := chi.URLParam(r, "mailboxId")

	var req model.UpdateSupportMailboxRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	mailbox, err := h.supportService.UpdateMailbox(r.Context(), workspaceID, mailboxID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, mailbox)
}

func (h *SupportInboxHandler) ArchiveMailbox(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	mailboxID := chi.URLParam(r, "mailboxId")

	mailbox, err := h.supportService.ArchiveMailbox(r.Context(), workspaceID, mailboxID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, mailbox)
}

func (h *SupportInboxHandler) ReorderMailboxes(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	var req model.ReorderSupportMailboxesRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.supportService.ReorderMailboxes(r.Context(), workspaceID, req.MailboxIDs); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SupportInboxHandler) ListMailboxMembers(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	mailboxID := chi.URLParam(r, "mailboxId")
	members, err := h.supportService.ListMailboxMembers(r.Context(), workspaceID, mailboxID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, members)
}

func (h *SupportInboxHandler) ListEmailRoutes(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	routes, err := h.supportService.ListEmailRoutes(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, routes)
}

func (h *SupportInboxHandler) CreateEmailRoute(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())

	var req model.CreateSupportEmailRouteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	route, err := h.supportService.CreateEmailRoute(r.Context(), workspaceID, req, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, route)
}

func (h *SupportInboxHandler) DisableEmailRoute(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	routeID := chi.URLParam(r, "routeId")

	if err := h.supportService.DisableEmailRoute(r.Context(), workspaceID, routeID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SupportInboxHandler) ListEmailSenderDomains(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	domains, err := h.supportService.ListEmailSenderDomains(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, domains)
}

func (h *SupportInboxHandler) CreateEmailSenderDomain(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())

	var req model.CreateSupportEmailSenderDomainRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	domain, err := h.supportService.CreateEmailSenderDomain(r.Context(), workspaceID, req, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, domain)
}

func (h *SupportInboxHandler) VerifyEmailSenderDomain(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	domainID := chi.URLParam(r, "domainId")

	domain, err := h.supportService.VerifyEmailSenderDomain(r.Context(), workspaceID, domainID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, domain)
}

func (h *SupportInboxHandler) ActivateEmailSenderDomain(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	domainID := chi.URLParam(r, "domainId")

	domain, err := h.supportService.ActivateEmailSenderDomain(r.Context(), workspaceID, domainID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, domain)
}

func (h *SupportInboxHandler) DeactivateEmailSenderDomain(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	domainID := chi.URLParam(r, "domainId")

	if err := h.supportService.DeactivateEmailSenderDomain(r.Context(), workspaceID, domainID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SupportInboxHandler) ListTriageRules(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	rules, err := h.supportService.ListTriageRules(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func (h *SupportInboxHandler) CreateTriageRule(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())

	var req model.CreateSupportTriageRuleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	rule, err := h.supportService.CreateTriageRule(r.Context(), workspaceID, actorID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (h *SupportInboxHandler) UpdateTriageRule(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	ruleID := chi.URLParam(r, "ruleId")

	var req model.UpdateSupportTriageRuleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	rule, err := h.supportService.UpdateTriageRule(r.Context(), workspaceID, ruleID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (h *SupportInboxHandler) DeleteTriageRule(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	ruleID := chi.URLParam(r, "ruleId")

	if err := h.supportService.DeleteTriageRule(r.Context(), workspaceID, ruleID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SupportInboxHandler) MoveConversation(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	conversationID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.MoveSupportConversationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	conversation, err := h.supportService.MoveConversation(r.Context(), workspaceID, conversationID, req.MailboxID, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, conversation)
}

func (h *SupportInboxHandler) DismissConversationTriage(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	conversationID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	triage, err := h.supportService.DismissConversationTriage(r.Context(), workspaceID, conversationID, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, triage)
}

// ListTeammatePresence handles GET /api/support/inbox/teammates/presence.
func (h *SupportInboxHandler) ListTeammatePresence(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	statuses, err := h.supportService.ListTeammatePresence(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, statuses)
}

// UpdateMyTeammatePresence handles PUT /api/support/inbox/me/presence.
func (h *SupportInboxHandler) UpdateMyTeammatePresence(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	userID := middleware.GetUserID(r.Context())

	var req model.UpdateSupportTeammatePresenceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	status, err := h.supportService.UpdateMyTeammatePresence(r.Context(), workspaceID, userID, req.ManualStatus)
	if err != nil {
		if err == service.ErrInvalidTeammateStatus {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// ListContactConversations handles GET /api/crm/contacts/{id}/support-tickets.
func (h *SupportInboxHandler) ListContactConversations(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	contactID := chi.URLParam(r, "id")
	pagination := queryPagination(r)

	conversations, total, err := h.supportService.ListContactConversations(r.Context(), workspaceID, contactID, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if conversations == nil {
		conversations = []model.SupportConversation{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  conversations,
		"total": total,
		"page":  pagination.Page,
	})
}

// RunAgent handles POST /api/support/tickets/{id}/run-agent.
func (h *SupportInboxHandler) RunAgent(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	ticketID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	run, err := h.agentService.RunConversationAgent(r.Context(), workspaceID, ticketID, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, run)
}

// ListCannedResponses handles GET /api/support/inbox/canned-responses.
func (h *SupportInboxHandler) ListCannedResponses(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	responses, err := h.supportService.ListCannedResponses(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if responses == nil {
		responses = []model.SupportCannedResponse{}
	}
	writeJSON(w, http.StatusOK, responses)
}

// SearchCannedResponses handles GET /api/support/inbox/canned-responses/search.
func (h *SupportInboxHandler) SearchCannedResponses(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}

	responses, err := h.supportService.SearchCannedResponses(r.Context(), workspaceID, query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if responses == nil {
		responses = []model.SupportCannedResponse{}
	}
	writeJSON(w, http.StatusOK, responses)
}

// CreateCannedResponse handles POST /api/support/inbox/canned-responses.
func (h *SupportInboxHandler) CreateCannedResponse(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	actorID := middleware.GetUserID(r.Context())

	var req model.CannedResponseRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	response, err := h.supportService.CreateCannedResponse(r.Context(), workspaceID, req, actorID)
	if err != nil {
		if errors.Is(err, service.ErrCannedResponseDuplicate) {
			writeError(w, http.StatusConflict, "shortcut already exists")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

// UpdateCannedResponse handles PUT /api/support/inbox/canned-responses/{id}.
func (h *SupportInboxHandler) UpdateCannedResponse(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	id := chi.URLParam(r, "id")

	var req model.CannedResponseRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	response, err := h.supportService.UpdateCannedResponse(r.Context(), workspaceID, id, req)
	if err != nil {
		if errors.Is(err, service.ErrCannedResponseDuplicate) {
			writeError(w, http.StatusConflict, "shortcut already exists")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// DeleteCannedResponse handles DELETE /api/support/inbox/canned-responses/{id}.
func (h *SupportInboxHandler) DeleteCannedResponse(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	id := chi.URLParam(r, "id")

	if err := h.supportService.DeleteCannedResponse(r.Context(), workspaceID, id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// GetInstallation handles GET /api/support/inbox/installations.
func (h *SupportInboxHandler) GetInstallation(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	inst, settings, err := h.supportService.GetInstallation(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.InstallationSettingsResponse{
		ID:          inst.ID,
		WorkspaceID: inst.WorkspaceID,
		WidgetKey:   inst.WidgetKey,
		Settings:    *settings,
		Active:      inst.Active,
		CreatedAt:   inst.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   inst.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// UpdateInstallationSettings handles PATCH /api/support/inbox/installations.
func (h *SupportInboxHandler) UpdateInstallationSettings(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	var req model.UpdateInstallationSettingsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	inst, settings, err := h.supportService.UpdateInstallationSettings(r.Context(), workspaceID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.InstallationSettingsResponse{
		ID:          inst.ID,
		WorkspaceID: inst.WorkspaceID,
		WidgetKey:   inst.WidgetKey,
		Settings:    *settings,
		Active:      inst.Active,
		CreatedAt:   inst.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   inst.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// RegenerateWidgetKey handles POST /api/support/inbox/installations/regenerate-key.
func (h *SupportInboxHandler) RegenerateWidgetKey(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	inst, settings, err := h.supportService.RegenerateWidgetKey(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.InstallationSettingsResponse{
		ID:          inst.ID,
		WorkspaceID: inst.WorkspaceID,
		WidgetKey:   inst.WidgetKey,
		Settings:    *settings,
		Active:      inst.Active,
		CreatedAt:   inst.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   inst.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// GetVisitorContext handles GET /api/support/inbox/conversations/{id}/visitor-context.
func (h *SupportInboxHandler) GetVisitorContext(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	conversationID := chi.URLParam(r, "id")

	resp, err := h.supportService.GetVisitorContext(r.Context(), workspaceID, conversationID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// TypingIndicator handles POST /api/support/inbox/conversations/{id}/typing.
func (h *SupportInboxHandler) TypingIndicator(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	conversationID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.TypingIndicatorRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	h.supportService.PublishTypingIndicator(r.Context(), workspaceID, conversationID, actorID, req.IsTyping, req.Content)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// ViewingPresence handles POST /api/support/inbox/conversations/{id}/viewing.
func (h *SupportInboxHandler) ViewingPresence(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	conversationID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.ViewingPresenceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	h.supportService.PublishViewingPresence(r.Context(), workspaceID, conversationID, actorID, req.Viewing)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}
