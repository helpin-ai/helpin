package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// ConversationMergeCandidates lists visible destination conversations.
func (h *SupportInboxHandler) ConversationMergeCandidates(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	data, err := h.supportService.ConversationMergeCandidates(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), r.URL.Query().Get("search"), r.URL.Query().Get("all_customers") == "true", page)
	if err != nil {
		writeSupportMergeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// PreviewConversationMerge reviews an explicit destination and source set.
func (h *SupportInboxHandler) PreviewConversationMerge(w http.ResponseWriter, r *http.Request) {
	var req model.SupportMergeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, 400, "Invalid merge request")
		return
	}
	data, err := h.supportService.PreviewConversationMerge(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), req)
	if err != nil {
		writeSupportMergeError(w, r, err)
		return
	}
	writeJSON(w, 200, data)
}

// MergeConversations commits an idempotent reviewed merge.
func (h *SupportInboxHandler) MergeConversations(w http.ResponseWriter, r *http.Request) {
	var req model.SupportMergeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, 400, "Invalid merge request")
		return
	}
	data, err := h.supportService.MergeConversations(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), req)
	if err != nil {
		writeSupportMergeError(w, r, err)
		return
	}
	writeJSON(w, 200, data)
}

// ListConversationMerges returns authorized provenance for the thread.
func (h *SupportInboxHandler) ListConversationMerges(w http.ResponseWriter, r *http.Request) {
	data, err := h.supportService.ListConversationMerges(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"))
	if err != nil {
		writeSupportMergeError(w, r, err)
		return
	}
	writeJSON(w, 200, data)
}

// ConversationSharedHistory reads bounded history from one merged interval.
func (h *SupportInboxHandler) ConversationSharedHistory(w http.ResponseWriter, r *http.Request) {
	data, err := h.supportService.ConversationSharedHistory(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), r.URL.Query().Get("merge_id"), r.URL.Query().Get("cursor"))
	if err != nil {
		writeSupportMergeError(w, r, err)
		return
	}
	writeJSON(w, 200, data)
}

// PreviewConversationUnmerge reviews separation without changing records.
func (h *SupportInboxHandler) PreviewConversationUnmerge(w http.ResponseWriter, r *http.Request) {
	var req model.SupportMergeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, 400, "Invalid unmerge request")
		return
	}
	data, err := h.supportService.PreviewConversationUnmerge(r.Context(), getWorkspaceID(r), chi.URLParam(r, "mergeID"), req.RestoreMailboxID)
	if err != nil {
		writeSupportMergeError(w, r, err)
		return
	}
	writeJSON(w, 200, data)
}

// UnmergeConversation restores a source with AI paused.
func (h *SupportInboxHandler) UnmergeConversation(w http.ResponseWriter, r *http.Request) {
	var req model.SupportMergeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, 400, "Invalid unmerge request")
		return
	}
	data, err := h.supportService.UnmergeConversation(r.Context(), getWorkspaceID(r), chi.URLParam(r, "mergeID"), req)
	if err != nil {
		writeSupportMergeError(w, r, err)
		return
	}
	writeJSON(w, 200, data)
}

func writeSupportMergeError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, service.ErrSupportMergeDenied):
		status = http.StatusForbidden
	case errors.Is(err, repository.ErrSupportMergeNotFound):
		status = http.StatusNotFound
	case errors.Is(err, repository.ErrSupportMerged), errors.Is(err, repository.ErrSupportMergeChanged), errors.Is(err, repository.ErrSupportMergeSending):
		status = http.StatusConflict
	default:
		// Only known validation messages cross the API boundary.
		switch err.Error() {
		case "move spam conversations back to the inbox before merging", "select between 2 and 50 conversations", "select each conversation once", "select up to 50 conversations including merged history", "deleted customers cannot be merged", "different customer; ask an admin to enable this", "a valid operation_id is required", "this conversation was deleted and cannot be restored", "invalid page":
		default:
			slog.ErrorContext(r.Context(), "support merge failed", "error", err, "workspace_id", getWorkspaceID(r))
			writeError(w, 500, "Could not update the merge. Please try again.")
			return
		}
	}
	writeError(w, status, err.Error())
}

// RequireWritableConversation blocks legacy and modern mutation routes on sources.
func (h *SupportInboxHandler) RequireWritableConversation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id != "" && (strings.Contains(r.URL.Path, "/inbox/conversations/") || strings.Contains(r.URL.Path, "/tickets/")) {
			if err := h.supportService.RequireWritableSupportConversation(r.Context(), getWorkspaceID(r), id); err != nil {
				writeSupportMergeError(w, r, err)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
