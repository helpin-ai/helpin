package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type PMRecurringTemplateHandler struct {
	recurringService *service.PMRecurringTemplateService
}

func NewPMRecurringTemplateHandler(recurringService *service.PMRecurringTemplateService) *PMRecurringTemplateHandler {
	return &PMRecurringTemplateHandler{recurringService: recurringService}
}

func (h *PMRecurringTemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	status := queryStringPtr(r, "status")
	teamID := queryStringPtr(r, "team_id")
	search := queryStringPtr(r, "search")
	items, err := h.recurringService.List(r.Context(), workspaceID, status, teamID, search)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if items == nil {
		items = []model.RecurringTemplateDetail{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *PMRecurringTemplateHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	item, err := h.recurringService.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *PMRecurringTemplateHandler) GetByStory(w http.ResponseWriter, r *http.Request) {
	storyID := chi.URLParam(r, "storyId")
	item, err := h.recurringService.GetByStoryID(r.Context(), storyID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if item == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *PMRecurringTemplateHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateRecurringTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	actorID := middleware.GetUserID(r.Context())
	item, err := h.recurringService.Create(r.Context(), req, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *PMRecurringTemplateHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req model.UpdateRecurringTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	actorID := middleware.GetUserID(r.Context())
	item, err := h.recurringService.Update(r.Context(), id, req, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *PMRecurringTemplateHandler) Pause(w http.ResponseWriter, r *http.Request) {
	h.handleAction(w, r, func(ctxUserID string, id string) (*model.RecurringTemplateDetail, error) {
		return h.recurringService.Pause(r.Context(), id, ctxUserID)
	})
}

func (h *PMRecurringTemplateHandler) Resume(w http.ResponseWriter, r *http.Request) {
	h.handleAction(w, r, func(ctxUserID string, id string) (*model.RecurringTemplateDetail, error) {
		return h.recurringService.Resume(r.Context(), id, ctxUserID)
	})
}

func (h *PMRecurringTemplateHandler) Stop(w http.ResponseWriter, r *http.Request) {
	h.handleAction(w, r, func(ctxUserID string, id string) (*model.RecurringTemplateDetail, error) {
		return h.recurringService.Stop(r.Context(), id, ctxUserID)
	})
}

func (h *PMRecurringTemplateHandler) SkipNext(w http.ResponseWriter, r *http.Request) {
	h.handleAction(w, r, func(ctxUserID string, id string) (*model.RecurringTemplateDetail, error) {
		return h.recurringService.SkipNext(r.Context(), id, ctxUserID)
	})
}

func (h *PMRecurringTemplateHandler) GenerateNow(w http.ResponseWriter, r *http.Request) {
	h.handleAction(w, r, func(ctxUserID string, id string) (*model.RecurringTemplateDetail, error) {
		return h.recurringService.GenerateNow(r.Context(), id, ctxUserID)
	})
}

func (h *PMRecurringTemplateHandler) Duplicate(w http.ResponseWriter, r *http.Request) {
	h.handleAction(w, r, func(ctxUserID string, id string) (*model.RecurringTemplateDetail, error) {
		return h.recurringService.Duplicate(r.Context(), id, ctxUserID)
	})
}

func (h *PMRecurringTemplateHandler) handleAction(
	w http.ResponseWriter,
	r *http.Request,
	fn func(actorID string, id string) (*model.RecurringTemplateDetail, error),
) {
	id := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())
	item, err := fn(actorID, id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}
